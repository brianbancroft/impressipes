package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/listsvc"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

func toList(l dbgen.List) api.List {
	return api.List{Id: uuidx.String(l.ID), Name: l.Name, Emoji: l.Emoji, Colour: l.Colour, SortOrder: int(l.SortOrder)}
}

func toCategory(c dbgen.ListCategory) api.Category {
	return api.Category{
		Id: uuidx.String(c.ID), ListId: uuidx.String(c.ListID), Name: c.Name,
		Colour: c.Colour, Icon: c.Icon, SortOrder: int(c.SortOrder),
	}
}

func toItem(i dbgen.ListItem) api.Item {
	return api.Item{
		Id: uuidx.String(i.ID), ListId: uuidx.String(i.ListID), CategoryId: uuidx.Ptr(i.CategoryID),
		Name: i.Name, QuantityText: i.QuantityText, Note: i.Note,
		Checked: i.CheckedAt.Valid, CheckedBy: uuidx.Ptr(i.CheckedBy), AddedBy: uuidx.String(i.AddedBy),
		SourceRecipeId: uuidx.Ptr(i.SourceRecipeID), Position: int(i.Position),
		Version: i.Version, UpdatedAt: i.UpdatedAt.Time, Deleted: i.DeletedAt.Valid,
	}
}

func (s *server) ListLists(ctx context.Context, _ api.ListListsRequestObject) (api.ListListsResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	rows, err := s.q.ListLists(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListLists200JSONResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, toList(l))
	}
	return out, nil
}

func (s *server) CreateList(ctx context.Context, req api.CreateListRequestObject) (api.CreateListResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil || strings.TrimSpace(b.Name) == "" || b.Colour == "" {
		return nil, fail(http.StatusBadRequest, "name and colour are required")
	}
	emoji := ""
	if b.Emoji != nil {
		emoji = *b.Emoji
	}
	var list dbgen.List
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		order, err := q.NextListSortOrder(ctx)
		if err != nil {
			return err
		}
		if list, err = q.CreateList(ctx, dbgen.CreateListParams{
			Name: strings.TrimSpace(b.Name), Emoji: emoji, Colour: b.Colour, SortOrder: order,
		}); err != nil {
			return err
		}
		if b.Categories != nil {
			for i, c := range *b.Categories {
				icon := ""
				if c.Icon != nil {
					icon = *c.Icon
				}
				if strings.TrimSpace(c.Name) == "" || c.Colour == "" {
					return fail(http.StatusBadRequest, "category name and colour are required")
				}
				if _, err := q.CreateListCategory(ctx, dbgen.CreateListCategoryParams{
					ListID: list.ID, Name: strings.TrimSpace(c.Name), Colour: c.Colour, Icon: icon, SortOrder: int32(i),
				}); err != nil {
					return err
				}
			}
		}
		return s.record(ctx, q, m, "list_created", list.ID, pgtype.UUID{}, map[string]any{"name": list.Name})
	})
	if err != nil {
		return nil, err
	}
	return api.CreateList201JSONResponse(toList(list)), nil
}

func (s *server) GetList(ctx context.Context, req api.GetListRequestObject) (api.GetListResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	id, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	list, err := s.q.GetList(ctx, id)
	if err != nil {
		return nil, orNotFound(err, "list")
	}
	cats, err := s.q.ListCategories(ctx, id)
	if err != nil {
		return nil, err
	}
	items, err := s.q.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	d := api.ListDetail{
		Id: uuidx.String(list.ID), Name: list.Name, Emoji: list.Emoji, Colour: list.Colour, SortOrder: int(list.SortOrder),
		Categories: make([]api.Category, 0, len(cats)), Items: make([]api.Item, 0, len(items)),
	}
	for _, c := range cats {
		d.Categories = append(d.Categories, toCategory(c))
	}
	for _, i := range items {
		d.Items = append(d.Items, toItem(i))
	}
	return api.GetList200JSONResponse(d), nil
}

func (s *server) UpdateList(ctx context.Context, req api.UpdateListRequestObject) (api.UpdateListResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	if b.Name != nil && strings.TrimSpace(*b.Name) == "" {
		return nil, fail(http.StatusBadRequest, "name cannot be empty")
	}
	p := dbgen.UpdateListParams{ID: id, Name: trimmed(b.Name), Emoji: textOf2(b.Emoji), Colour: textOf2(b.Colour)}
	if b.SortOrder != nil {
		p.SortOrder = pgtype.Int4{Int32: int32(*b.SortOrder), Valid: true}
	}
	var list dbgen.List
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		var err error
		if list, err = q.UpdateList(ctx, p); err != nil {
			return orNotFound(err, "list")
		}
		return s.record(ctx, q, m, "list_updated", list.ID, pgtype.UUID{}, nil)
	})
	if err != nil {
		return nil, err
	}
	return api.UpdateList200JSONResponse(toList(list)), nil
}

func (s *server) ArchiveList(ctx context.Context, req api.ArchiveListRequestObject) (api.ArchiveListResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		n, err := q.ArchiveList(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return notFound("list")
		}
		return s.record(ctx, q, m, "list_archived", id, pgtype.UUID{}, nil)
	})
	if err != nil {
		return nil, err
	}
	return api.ArchiveList204Response{}, nil
}

func trimmed(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(*p), Valid: true}
}

func textOf2(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func (s *server) CreateCategory(ctx context.Context, req api.CreateCategoryRequestObject) (api.CreateCategoryResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	listID, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil || strings.TrimSpace(b.Name) == "" || b.Colour == "" {
		return nil, fail(http.StatusBadRequest, "name and colour are required")
	}
	icon := ""
	if b.Icon != nil {
		icon = *b.Icon
	}
	var cat dbgen.ListCategory
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		if _, err := q.GetList(ctx, listID); err != nil {
			return orNotFound(err, "list")
		}
		order, err := q.NextCategorySortOrder(ctx, listID)
		if err != nil {
			return err
		}
		if cat, err = q.CreateListCategory(ctx, dbgen.CreateListCategoryParams{
			ListID: listID, Name: strings.TrimSpace(b.Name), Colour: b.Colour, Icon: icon, SortOrder: order,
		}); err != nil {
			return err
		}
		return s.record(ctx, q, m, "category_created", listID, pgtype.UUID{}, map[string]any{"name": cat.Name})
	})
	if err != nil {
		return nil, err
	}
	return api.CreateCategory201JSONResponse(toCategory(cat)), nil
}

func (s *server) UpdateCategory(ctx context.Context, req api.UpdateCategoryRequestObject) (api.UpdateCategoryResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	listID, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.CategoryId, "category")
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	if b.Name != nil && strings.TrimSpace(*b.Name) == "" {
		return nil, fail(http.StatusBadRequest, "name cannot be empty")
	}
	p := dbgen.UpdateCategoryParams{ID: id, ListID: listID, Name: trimmed(b.Name), Colour: textOf2(b.Colour), Icon: textOf2(b.Icon)}
	if b.SortOrder != nil {
		p.SortOrder = pgtype.Int4{Int32: int32(*b.SortOrder), Valid: true}
	}
	cat, err := s.q.UpdateCategory(ctx, p)
	if err != nil {
		return nil, orNotFound(err, "category")
	}
	return api.UpdateCategory200JSONResponse(toCategory(cat)), nil
}

func (s *server) DeleteCategory(ctx context.Context, req api.DeleteCategoryRequestObject) (api.DeleteCategoryResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	listID, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.CategoryId, "category")
	if err != nil {
		return nil, err
	}
	n, err := s.q.DeleteCategory(ctx, dbgen.DeleteCategoryParams{ID: id, ListID: listID})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, notFound("category")
	}
	return api.DeleteCategory204Response{}, nil
}

// categoryIn checks a category id belongs to the list.
func categoryIn(ctx context.Context, q *dbgen.Queries, raw string, list pgtype.UUID) (pgtype.UUID, error) {
	id, ok := uuidx.Parse(raw)
	if !ok {
		return id, fail(http.StatusBadRequest, "invalid categoryId")
	}
	if _, err := q.GetCategory(ctx, dbgen.GetCategoryParams{ID: id, ListID: list}); err != nil {
		if err == pgx.ErrNoRows {
			return id, fail(http.StatusBadRequest, "category is not in this list")
		}
		return id, err
	}
	return id, nil
}

func (s *server) CreateItem(ctx context.Context, req api.CreateItemRequestObject) (api.CreateItemResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	listID, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil || strings.TrimSpace(b.Name) == "" {
		return nil, fail(http.StatusBadRequest, "name is required")
	}
	itemID := uuidx.New()
	if b.Id != nil {
		var ok bool
		if itemID, ok = uuidx.Parse(*b.Id); !ok {
			return nil, fail(http.StatusBadRequest, "invalid id")
		}
	}
	// A retried create returns the stored item untouched.
	if existing, err := s.q.GetItem(ctx, itemID); err == nil {
		if existing.ListID != listID {
			return nil, fail(http.StatusBadRequest, "id is already used by another list")
		}
		return api.CreateItem201JSONResponse(toItem(existing)), nil
	} else if err != pgx.ErrNoRows {
		return nil, err
	}

	var item dbgen.ListItem
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		if _, err := q.GetList(ctx, listID); err != nil {
			return orNotFound(err, "list")
		}
		user, _ := uuidx.Parse(m.ID)
		item, err = addItem(ctx, q, listID, user, itemID, strings.TrimSpace(b.Name), deref(b.QuantityText), deref(b.Note), b.CategoryId, pgtype.UUID{})
		if err != nil {
			return err
		}
		return s.record(ctx, q, m, "item_added", listID, pgtype.UUID{}, map[string]any{"item": item.Name, "itemId": uuidx.String(item.ID)})
	})
	if err != nil {
		return nil, err
	}
	return api.CreateItem201JSONResponse(toItem(item)), nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// addItem inserts an item, filling a missing category or quantity from the list's
// item memory, and teaches the memory about it.
func addItem(ctx context.Context, q *dbgen.Queries, listID, user, id pgtype.UUID, name, qty, note string, category *string, recipe pgtype.UUID) (dbgen.ListItem, error) {
	norm := listsvc.Normalize(name)
	mem, err := q.GetItemMemory(ctx, dbgen.GetItemMemoryParams{ListID: listID, NormalizedName: norm})
	if err != nil && err != pgx.ErrNoRows {
		return dbgen.ListItem{}, err
	}
	known := err == nil

	var cat pgtype.UUID
	switch {
	case category != nil && *category != "":
		if cat, err = categoryIn(ctx, q, *category, listID); err != nil {
			return dbgen.ListItem{}, err
		}
	case known:
		cat = mem.LastCategoryID
	}
	if qty == "" && known {
		qty = mem.LastQuantity
	}
	pos, err := q.NextItemPosition(ctx, listID)
	if err != nil {
		return dbgen.ListItem{}, err
	}
	item, err := q.InsertItemIfAbsent(ctx, dbgen.InsertItemIfAbsentParams{
		ID: id, ListID: listID, CategoryID: cat, Name: name, QuantityText: qty, Note: note,
		AddedBy: user, SourceRecipeID: recipe, Position: pos,
	})
	if err != nil {
		return item, err
	}
	return item, q.RememberItem(ctx, dbgen.RememberItemParams{
		ListID: listID, NormalizedName: norm, LastCategoryID: item.CategoryID, LastQuantity: item.QuantityText,
	})
}

func (s *server) SuggestItems(ctx context.Context, req api.SuggestItemsRequestObject) (api.SuggestItemsResponseObject, error) {
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
	rows, err := s.q.SuggestItems(ctx, dbgen.SuggestItemsParams{ListID: listID, Prefix: listsvc.Normalize(req.Params.Q)})
	if err != nil {
		return nil, err
	}
	out := make(api.SuggestItems200JSONResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.Suggestion{Name: r.NormalizedName, QuantityText: r.LastQuantity, CategoryId: uuidx.Ptr(r.LastCategoryID)})
	}
	return out, nil
}

func (s *server) UpdateItem(ctx context.Context, req api.UpdateItemRequestObject) (api.UpdateItemResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.ItemId, "item")
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	if b.Name != nil && strings.TrimSpace(*b.Name) == "" {
		return nil, fail(http.StatusBadRequest, "name cannot be empty")
	}
	var item dbgen.ListItem
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		cur, err := q.GetItem(ctx, id)
		if err != nil {
			return orNotFound(err, "item")
		}
		p := dbgen.UpdateItemParams{ID: id, Name: trimmed(b.Name), QuantityText: textOf2(b.QuantityText), Note: textOf2(b.Note)}
		if b.Position != nil {
			p.Position = pgtype.Int4{Int32: int32(*b.Position), Valid: true}
		}
		if b.CategoryId != nil {
			p.SetCategory = true
			if *b.CategoryId != "" {
				if p.CategoryID, err = categoryIn(ctx, q, *b.CategoryId, cur.ListID); err != nil {
					return err
				}
			}
		}
		if item, err = q.UpdateItem(ctx, p); err != nil {
			return orNotFound(err, "item")
		}
		if b.Checked != nil && *b.Checked != cur.CheckedAt.Valid {
			user, _ := uuidx.Parse(m.ID)
			if item, err = q.SetItemChecked(ctx, dbgen.SetItemCheckedParams{ID: id, Checked: *b.Checked, UserID: user}); err != nil {
				return orNotFound(err, "item")
			}
		}
		if b.CategoryId != nil {
			if err := q.RememberItem(ctx, dbgen.RememberItemParams{
				ListID: item.ListID, NormalizedName: listsvc.Normalize(item.Name),
				LastCategoryID: item.CategoryID, LastQuantity: item.QuantityText,
			}); err != nil {
				return err
			}
		}
		action := "item_updated"
		if b.Checked != nil && *b.Checked != cur.CheckedAt.Valid {
			action = "item_unchecked"
			if *b.Checked {
				action = "item_checked"
			}
		}
		return s.record(ctx, q, m, action, item.ListID, pgtype.UUID{}, map[string]any{"item": item.Name, "itemId": uuidx.String(item.ID)})
	})
	if err != nil {
		return nil, err
	}
	return api.UpdateItem200JSONResponse(toItem(item)), nil
}

func (s *server) DeleteItem(ctx context.Context, req api.DeleteItemRequestObject) (api.DeleteItemResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.ItemId, "item")
	if err != nil {
		return nil, err
	}
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		item, err := q.SoftDeleteItem(ctx, id)
		if err != nil {
			return orNotFound(err, "item")
		}
		return s.record(ctx, q, m, "item_deleted", item.ListID, pgtype.UUID{}, map[string]any{"item": item.Name, "itemId": uuidx.String(item.ID)})
	})
	if err != nil {
		return nil, err
	}
	return api.DeleteItem204Response{}, nil
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
	out := make(api.ListActivity200JSONResponse, 0, len(rows))
	for _, a := range rows {
		detail := map[string]any{}
		_ = jsonUnmarshal(a.Detail, &detail)
		out = append(out, api.Activity{
			Id: a.ID, UserId: uuidx.Ptr(a.UserID), Via: a.Via, Action: a.Action,
			RecipeId: uuidx.Ptr(a.RecipeID), Detail: detail, CreatedAt: a.CreatedAt.Time,
		})
	}
	return out, nil
}
