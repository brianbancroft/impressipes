package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

// PullSync returns every item change after a version. Versions come from a sequence
// assigned on write, so a client that pulls and stores the highest version it saw
// never skips a change made by a short, single-statement write.
func (s *server) PullSync(ctx context.Context, req api.PullSyncRequestObject) (api.PullSyncResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	if req.Params.Since < 0 {
		return nil, fail(http.StatusBadRequest, "since must not be negative")
	}
	rows, err := s.q.ItemsSince(ctx, req.Params.Since)
	if err != nil {
		return nil, err
	}
	out := api.SyncChanges{Version: req.Params.Since, Items: make([]api.Item, 0, len(rows))}
	for _, r := range rows {
		out.Items = append(out.Items, toItem(r))
		out.Version = max(out.Version, r.Version)
	}
	return api.PullSync200JSONResponse(out), nil
}

// PushSync applies offline operations in order. Each is idempotent on its item id,
// so a client can resend a whole batch after a dropped response.
func (s *server) PushSync(ctx context.Context, req api.PushSyncRequestObject) (api.PushSyncResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	user, _ := uuidx.Parse(m.ID)
	res := api.SyncResult{Results: make([]struct {
		ItemId string                      `json:"itemId"`
		Status api.SyncResultResultsStatus `json:"status"`
	}, 0, len(req.Body.Ops))}

	for _, op := range req.Body.Ops {
		status := api.Applied
		if err := s.applySyncOp(ctx, m, user, op); err != nil {
			switch {
			case err == errOpNotFound:
				status = api.NotFound
			case err == errOpInvalid:
				status = api.Invalid
			default:
				return nil, err
			}
		}
		res.Results = append(res.Results, struct {
			ItemId string                      `json:"itemId"`
			Status api.SyncResultResultsStatus `json:"status"`
		}{op.ItemId, status})
	}
	if res.Version, err = s.q.CurrentItemVersion(ctx); err != nil {
		return nil, err
	}
	return api.PushSync200JSONResponse(res), nil
}

var (
	errOpNotFound = &httpError{http.StatusNotFound, "not found"}
	errOpInvalid  = &httpError{http.StatusBadRequest, "invalid"}
)

func (s *server) applySyncOp(ctx context.Context, m *family.Member, user pgtype.UUID, op api.SyncOp) error {
	id, ok := uuidx.Parse(op.ItemId)
	if !ok {
		return errOpInvalid
	}
	return s.withTx(ctx, func(q *dbgen.Queries) error {
		switch op.Type {
		case api.Upsert:
			if op.Item == nil || op.ListId == nil || strings.TrimSpace(op.Item.Name) == "" {
				return errOpInvalid
			}
			listID, ok := uuidx.Parse(*op.ListId)
			if !ok {
				return errOpInvalid
			}
			if _, err := q.GetList(ctx, listID); err != nil {
				if err == pgx.ErrNoRows {
					return errOpNotFound
				}
				return err
			}
			name := strings.TrimSpace(op.Item.Name)
			cur, err := q.GetItem(ctx, id)
			switch {
			case err == pgx.ErrNoRows:
				item, err := addItem(ctx, q, listID, user, id, name, deref(op.Item.QuantityText), deref(op.Item.Note), op.Item.CategoryId, pgtype.UUID{})
				if err != nil {
					return syncInvalid(err)
				}
				return s.record(ctx, q, m, "item_added", listID, pgtype.UUID{}, map[string]any{"item": item.Name, "itemId": op.ItemId})
			case err != nil:
				return err
			case cur.ListID != listID:
				return errOpInvalid
			}
			p := dbgen.UpdateItemParams{ID: id, Name: pgtype.Text{String: name, Valid: true},
				QuantityText: textOf2(op.Item.QuantityText), Note: textOf2(op.Item.Note)}
			if op.Item.CategoryId != nil {
				p.SetCategory = true
				if *op.Item.CategoryId != "" {
					if p.CategoryID, err = categoryIn(ctx, q, *op.Item.CategoryId, listID); err != nil {
						return syncInvalid(err)
					}
				}
			}
			item, err := q.UpdateItem(ctx, p)
			if err != nil {
				return err
			}
			return s.record(ctx, q, m, "item_updated", listID, pgtype.UUID{}, itemDetail(item, cur))
		case api.Check, api.Uncheck:
			checked := op.Type == api.Check
			cur, err := q.GetItem(ctx, id)
			if err == pgx.ErrNoRows {
				return errOpNotFound
			} else if err != nil {
				return err
			}
			item, err := q.SetItemChecked(ctx, dbgen.SetItemCheckedParams{ID: id, Checked: checked, UserID: user})
			if err == pgx.ErrNoRows {
				return errOpNotFound
			} else if err != nil {
				return err
			}
			action := "item_unchecked"
			if checked {
				action = "item_checked"
			}
			return s.record(ctx, q, m, action, item.ListID, pgtype.UUID{}, itemDetail(item, cur))
		case api.Delete:
			item, err := q.SoftDeleteItem(ctx, id)
			if err == pgx.ErrNoRows {
				return nil // already gone: a retried delete is a success
			} else if err != nil {
				return err
			}
			return s.record(ctx, q, m, "item_deleted", item.ListID, pgtype.UUID{}, map[string]any{"item": item.Name, "itemId": op.ItemId})
		}
		return errOpInvalid
	})
}

// syncInvalid turns a validation failure (a 400) into an invalid outcome for one
// operation, so it does not fail the whole batch.
func syncInvalid(err error) error {
	if he, ok := err.(*httpError); ok && he.code == http.StatusBadRequest {
		return errOpInvalid
	}
	return err
}
