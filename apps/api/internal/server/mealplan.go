package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

const maxPlanSpan = 400 * 24 * time.Hour

func validSlot(slot api.MealSlot) bool {
	switch slot {
	case api.Breakfast, api.Lunch, api.Dinner, api.Extra:
		return true
	default:
		return false
	}
}

func pgDate(d openapi_types.Date) pgtype.Date {
	t := d.Time.UTC()
	return pgtype.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

func apiDate(d pgtype.Date) openapi_types.Date {
	t := d.Time.UTC()
	return openapi_types.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

func toMealPlan(id pgtype.UUID, planDate pgtype.Date, slot, title string, recipeID pgtype.UUID, completedAt pgtype.Timestamptz, completedBy pgtype.UUID, notes string, createdAt, updatedAt pgtype.Timestamptz, recipeTitle pgtype.Text) api.MealPlanEntry {
	out := api.MealPlanEntry{
		Id: uuidx.String(id), PlanDate: apiDate(planDate), Slot: api.MealSlot(slot), Title: title,
		Completed: completedAt.Valid, CompletedAt: timePtr(completedAt), CompletedBy: uuidx.Ptr(completedBy),
		Notes: notes, CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
	}
	// A soft-deleted recipe leaves the column set until detach runs; hide the link either way.
	if recipeID.Valid && recipeTitle.Valid {
		rid := uuidx.String(recipeID)
		out.RecipeId = &rid
		out.Recipe = &api.MealPlanRecipe{Id: rid, Title: recipeTitle.String}
	}
	return out
}

func toMealPlanRow(row dbgen.GetMealPlanEntryRow) api.MealPlanEntry {
	return toMealPlan(row.ID, row.PlanDate, row.Slot, row.Title, row.RecipeID, row.CompletedAt, row.CompletedBy, row.Notes, row.CreatedAt, row.UpdatedAt, row.RecipeTitle)
}

func (s *server) loadMeal(ctx context.Context, q *dbgen.Queries, id pgtype.UUID) (api.MealPlanEntry, error) {
	row, err := q.GetMealPlanEntry(ctx, id)
	if err != nil {
		return api.MealPlanEntry{}, orNotFound(err, "meal")
	}
	return toMealPlanRow(row), nil
}

// parseRecipeLink resolves an optional recipe. An empty string means no link.
func parseRecipeLink(ctx context.Context, q *dbgen.Queries, raw string) (pgtype.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pgtype.UUID{}, nil
	}
	id, ok := uuidx.Parse(raw)
	if !ok {
		return id, fail(http.StatusBadRequest, "invalid recipeId")
	}
	if _, err := q.GetRecipe(ctx, id); err != nil {
		if err == pgx.ErrNoRows {
			return id, fail(http.StatusBadRequest, "recipe not found")
		}
		return id, err
	}
	return id, nil
}

func (s *server) ListMealPlan(ctx context.Context, req api.ListMealPlanRequestObject) (api.ListMealPlanResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	from, to := req.Params.From.Time, req.Params.To.Time
	if to.Before(from) {
		return nil, fail(http.StatusBadRequest, "to is before from")
	}
	if to.Sub(from) > maxPlanSpan {
		return nil, fail(http.StatusBadRequest, "date range is too long")
	}
	rows, err := s.q.ListMealPlanEntries(ctx, dbgen.ListMealPlanEntriesParams{
		FromDate: pgDate(req.Params.From), ToDate: pgDate(req.Params.To),
	})
	if err != nil {
		return nil, err
	}
	out := make(api.ListMealPlan200JSONResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMealPlan(row.ID, row.PlanDate, row.Slot, row.Title, row.RecipeID, row.CompletedAt, row.CompletedBy, row.Notes, row.CreatedAt, row.UpdatedAt, row.RecipeTitle))
	}
	return out, nil
}

func (s *server) UpsertMealPlanEntry(ctx context.Context, req api.UpsertMealPlanEntryRequestObject) (api.UpsertMealPlanEntryResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil || strings.TrimSpace(b.Title) == "" {
		return nil, fail(http.StatusBadRequest, "title is required")
	}
	if !validSlot(b.Slot) {
		return nil, fail(http.StatusBadRequest, "invalid slot")
	}
	var out api.MealPlanEntry
	err := s.withTx(ctx, func(q *dbgen.Queries) error {
		var recipe pgtype.UUID
		if b.RecipeId != nil {
			var err error
			if recipe, err = parseRecipeLink(ctx, q, *b.RecipeId); err != nil {
				return err
			}
		}
		row, err := q.UpsertMealPlanEntry(ctx, dbgen.UpsertMealPlanEntryParams{
			PlanDate: pgDate(b.PlanDate), Slot: string(b.Slot), Title: strings.TrimSpace(b.Title),
			RecipeID: recipe, Notes: deref(b.Notes),
		})
		if err != nil {
			return err
		}
		out, err = s.loadMeal(ctx, q, row.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return api.UpsertMealPlanEntry200JSONResponse(out), nil
}

func (s *server) UpdateMealPlanEntry(ctx context.Context, req api.UpdateMealPlanEntryRequestObject) (api.UpdateMealPlanEntryResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.EntryId, "meal")
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	if b.Title != nil && strings.TrimSpace(*b.Title) == "" {
		return nil, fail(http.StatusBadRequest, "title cannot be empty")
	}
	var out api.MealPlanEntry
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		if _, err := q.GetMealPlanEntry(ctx, id); err != nil {
			return orNotFound(err, "meal")
		}
		if b.Title != nil || b.Notes != nil || b.RecipeId != nil {
			p := dbgen.PatchMealPlanEntryParams{ID: id, Title: trimmed(b.Title), Notes: textOf2(b.Notes)}
			if b.RecipeId != nil {
				p.SetRecipe = true
				if strings.TrimSpace(*b.RecipeId) != "" {
					if p.RecipeID, err = parseRecipeLink(ctx, q, *b.RecipeId); err != nil {
						return err
					}
				}
			}
			if _, err := q.PatchMealPlanEntry(ctx, p); err != nil {
				return orNotFound(err, "meal")
			}
		}
		if b.Completed != nil {
			user, _ := uuidx.Parse(m.ID)
			if _, err := q.SetMealPlanCompleted(ctx, dbgen.SetMealPlanCompletedParams{
				ID: id, Completed: *b.Completed, UserID: user,
			}); err != nil {
				return orNotFound(err, "meal")
			}
		}
		out, err = s.loadMeal(ctx, q, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return api.UpdateMealPlanEntry200JSONResponse(out), nil
}

func (s *server) DeleteMealPlanEntry(ctx context.Context, req api.DeleteMealPlanEntryRequestObject) (api.DeleteMealPlanEntryResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	id, err := idOf(req.EntryId, "meal")
	if err != nil {
		return nil, err
	}
	n, err := s.q.DeleteMealPlanEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, notFound("meal")
	}
	return api.DeleteMealPlanEntry204Response{}, nil
}
