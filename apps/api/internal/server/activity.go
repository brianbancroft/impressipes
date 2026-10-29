package server

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

// restoreWindow is how long a change stays undoable.
const restoreWindow = 90 * 24 * time.Hour

type batchKey struct{}

// batchOf is the AI request a change belongs to. A client (the MCP server) that
// makes several calls for one request sends the same X-Impressipies-Batch id on
// each, so the feed can group and undo them together.
func batchOf(ctx context.Context) pgtype.UUID {
	b, _ := ctx.Value(batchKey{}).(pgtype.UUID)
	return b
}

// undoable lists the actions that can be undone, and how.
var undoable = map[string]bool{
	"item_added": true, "item_deleted": true, "item_checked": true, "item_unchecked": true,
	"item_updated": true, "item_merged": true, "recipe_deleted": true,
}

// itemSnap is an item as the feed shows it before and after an edit, and as undo restores it.
func itemSnap(i dbgen.ListItem) map[string]any {
	return map[string]any{
		"name": i.Name, "quantityText": i.QuantityText, "note": i.Note,
		"categoryId": uuidx.Ptr(i.CategoryID), "checked": i.CheckedAt.Valid,
		"checkedBy": uuidx.Ptr(i.CheckedBy),
	}
}

func itemDetail(item dbgen.ListItem, before dbgen.ListItem) map[string]any {
	return map[string]any{
		"item": item.Name, "itemId": uuidx.String(item.ID),
		"before": itemSnap(before), "after": itemSnap(item),
	}
}

func toActivity(a dbgen.Activity, now time.Time) api.Activity {
	detail := map[string]any{}
	_ = jsonUnmarshal(a.Detail, &detail)
	until := a.CreatedAt.Time.Add(restoreWindow)
	out := api.Activity{
		Id: a.ID, UserId: uuidx.Ptr(a.UserID), Via: a.Via, Action: a.Action,
		ListId: uuidx.Ptr(a.ListID), RecipeId: uuidx.Ptr(a.RecipeID), BatchId: uuidx.Ptr(a.BatchID),
		Detail: detail, CreatedAt: a.CreatedAt.Time,
		Undoable: undoable[a.Action] && !a.UndoneAt.Valid && now.Before(until),
	}
	if undoable[a.Action] && !a.UndoneAt.Valid && !a.UndoOf.Valid {
		out.RestorableUntil = &until
	}
	if a.UndoneAt.Valid {
		out.UndoneAt = &a.UndoneAt.Time
		out.UndoneBy = uuidx.Ptr(a.UndoneBy)
	}
	if a.UndoOf.Valid {
		out.UndoOf = &a.UndoOf.Int64
	}
	return out
}

func toActivities(rows []dbgen.Activity) []api.Activity {
	now := timeNow()
	out := make([]api.Activity, 0, len(rows))
	for _, a := range rows {
		out = append(out, toActivity(a, now))
	}
	return out
}

func (s *server) ListActivity(ctx context.Context, req api.ListActivityRequestObject) (api.ListActivityResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	listID, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	if _, err := s.q.GetList(ctx, listID); err != nil {
		return nil, orNotFound(err, "list")
	}
	limit := int32(50)
	if req.Params.Limit != nil {
		limit = int32(*req.Params.Limit)
	}
	rows, err := s.q.RecentActivity(ctx, dbgen.RecentActivityParams{ListID: listID, Limit: limit})
	if err != nil {
		return nil, err
	}
	return api.ListActivity200JSONResponse(toActivities(rows)), nil
}

func (s *server) ListAllActivity(ctx context.Context, req api.ListAllActivityRequestObject) (api.ListAllActivityResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	p := req.Params
	q := dbgen.FilterActivityParams{RowLimit: 50, Via: text(p.Via), Action: text(p.Action)}
	if p.Limit != nil {
		q.RowLimit = int32(*p.Limit)
	}
	if p.UserId != nil {
		u, ok := uuidx.Parse(*p.UserId)
		if !ok {
			return nil, fail(http.StatusBadRequest, "invalid userId")
		}
		q.UserID = u
	}
	if p.Since != nil {
		q.Since = pgtype.Timestamptz{Time: *p.Since, Valid: true}
	}
	if p.Until != nil {
		q.Until = pgtype.Timestamptz{Time: *p.Until, Valid: true}
	}
	rows, err := s.q.FilterActivity(ctx, q)
	if err != nil {
		return nil, err
	}
	return api.ListAllActivity200JSONResponse(toActivities(rows)), nil
}

func (s *server) UndoActivity(ctx context.Context, req api.UndoActivityRequestObject) (api.UndoActivityResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	var out dbgen.Activity
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		a, err := q.GetActivity(ctx, req.ActivityId)
		if err != nil {
			return orNotFound(err, "activity")
		}
		out, err = s.undo(ctx, q, m, a, forced(req.Body))
		return err
	})
	if err != nil {
		return nil, err
	}
	return api.UndoActivity200JSONResponse(toActivity(out, timeNow())), nil
}

func (s *server) UndoActivityBatch(ctx context.Context, req api.UndoActivityBatchRequestObject) (api.UndoActivityBatchResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	batch, err := idOf(req.BatchId, "batch")
	if err != nil {
		return nil, err
	}
	done := []dbgen.Activity{}
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		rows, err := q.ActivityByBatch(ctx, batch)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return notFound("batch")
		}
		// Newest first, so a later change in the batch is undone before the one it built on.
		for _, a := range rows {
			if a.UndoneAt.Valid || a.UndoOf.Valid {
				continue
			}
			if a.Action == "recipe_added_to_list" {
				if _, err := q.MarkActivityUndone(ctx, dbgen.MarkActivityUndoneParams{ID: a.ID, UndoneBy: uuidOf(m)}); err != nil {
					return err
				}
				continue
			}
			u, err := s.undo(ctx, q, m, a, forced(req.Body))
			if err != nil {
				return err
			}
			done = append(done, u)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return api.UndoActivityBatch200JSONResponse(toActivities(done)), nil
}

func forced(b *api.UndoInput) bool { return b != nil && b.Force != nil && *b.Force }

func uuidOf(m *family.Member) pgtype.UUID {
	u, _ := uuidx.Parse(m.ID)
	return u
}

// undo reverses one change and records the undo as a new row. It answers 409 when a
// later change touched the same record, unless force is set.
func (s *server) undo(ctx context.Context, q *dbgen.Queries, m *family.Member, a dbgen.Activity, force bool) (dbgen.Activity, error) {
	switch {
	case a.UndoneAt.Valid:
		return a, fail(http.StatusConflict, "already undone")
	case a.UndoOf.Valid:
		return a, fail(http.StatusUnprocessableEntity, "an undo cannot be undone")
	case a.Action == "recipe_added_to_list":
		return a, fail(http.StatusUnprocessableEntity, "undo the items this added, or the whole request")
	case !undoable[a.Action]:
		return a, fail(http.StatusUnprocessableEntity, "this change cannot be undone")
	case !timeNow().Before(a.CreatedAt.Time.Add(restoreWindow)):
		return a, fail(http.StatusUnprocessableEntity, "the restore window has closed")
	}
	detail := map[string]any{}
	_ = jsonUnmarshal(a.Detail, &detail)
	itemID, _ := detail["itemId"].(string)

	if !force {
		// Item rows from add-to-list carry the recipe id too; only a recipe's own rows match on it.
		recipe := pgtype.UUID{}
		if a.Action == "recipe_deleted" {
			recipe = a.RecipeID
		}
		changed, err := q.ChangedSince(ctx, dbgen.ChangedSinceParams{AfterID: a.ID, ItemID: itemID, RecipeID: recipe})
		if err != nil {
			return a, err
		}
		if changed {
			return a, fail(http.StatusConflict, "changed since; undo again with force to overwrite")
		}
	}

	switch a.Action {
	case "recipe_deleted":
		if _, err := q.RestoreRecipe(ctx, a.RecipeID); err != nil {
			return a, err
		}
	default:
		id, ok := uuidx.Parse(itemID)
		if !ok {
			return a, fail(http.StatusUnprocessableEntity, "this change cannot be undone")
		}
		if err := undoItem(ctx, q, m, a.Action, id, detail); err != nil {
			return a, err
		}
	}

	if _, err := q.MarkActivityUndone(ctx, dbgen.MarkActivityUndoneParams{ID: a.ID, UndoneBy: uuidOf(m)}); err != nil {
		return a, err
	}
	d := map[string]any{"undid": a.Action}
	for _, k := range []string{"item", "itemId", "title"} {
		if v, ok := detail[k]; ok {
			d[k] = v
		}
	}
	return s.recordRow(ctx, q, m, "undo", a.ListID, a.RecipeID, d, a.BatchID, pgtype.Int8{Int64: a.ID, Valid: true})
}

func undoItem(ctx context.Context, q *dbgen.Queries, m *family.Member, action string, id pgtype.UUID, detail map[string]any) error {
	switch action {
	case "item_added":
		if _, err := q.SoftDeleteItem(ctx, id); err != nil && err != pgx.ErrNoRows {
			return err
		}
		return nil
	case "item_deleted":
		if _, err := q.RestoreItem(ctx, id); err != nil && err != pgx.ErrNoRows {
			return err
		}
		return nil
	}
	// Edits, checks and merges all put the item back to its "before" snapshot.
	before, _ := detail["before"].(map[string]any)
	cur, err := q.GetItem(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fail(http.StatusConflict, "the item was deleted since")
		}
		return err
	}
	str := func(k string) pgtype.Text {
		v, ok := before[k].(string)
		return pgtype.Text{String: v, Valid: ok}
	}
	p := dbgen.UpdateItemParams{ID: id, Name: str("name"), QuantityText: str("quantityText"), Note: str("note")}
	if _, has := before["categoryId"]; has {
		p.SetCategory = true
		if raw, _ := before["categoryId"].(string); raw != "" {
			// The category may have been deleted since; fall back to none.
			if c, ok := uuidx.Parse(raw); ok {
				if _, err := q.GetCategory(ctx, dbgen.GetCategoryParams{ID: c, ListID: cur.ListID}); err == nil {
					p.CategoryID = c
				}
			}
		}
	}
	if _, err := q.UpdateItem(ctx, p); err != nil {
		return err
	}
	if want, ok := before["checked"].(bool); ok && want != cur.CheckedAt.Valid {
		by := uuidOf(m)
		if raw, _ := before["checkedBy"].(string); raw != "" {
			if u, ok := uuidx.Parse(raw); ok {
				by = u
			}
		}
		if _, err := q.SetItemChecked(ctx, dbgen.SetItemCheckedParams{ID: id, Checked: want, UserID: by}); err != nil {
			return err
		}
	}
	return nil
}
