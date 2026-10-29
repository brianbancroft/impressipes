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

func toSummary(r dbgen.Recipe, tags []string, marks []api.RecipeMark, cover *string) api.RecipeSummary {
	if tags == nil {
		tags = []string{}
	}
	if marks == nil {
		marks = []api.RecipeMark{}
	}
	return api.RecipeSummary{
		Id: uuidx.String(r.ID), Title: r.Title, Summary: r.Summary, Servings: floatOf(r.Servings),
		PrepMinutes: intOf(r.PrepMinutes), CookMinutes: intOf(r.CookMinutes), SourceKind: r.SourceKind,
		Tags: tags, Marks: marks, CreatedBy: uuidx.String(r.CreatedBy), UpdatedAt: r.UpdatedAt.Time,
		CoverUrl: cover,
	}
}

// recipeLabels loads the tags and the member's marks for a set of recipes.
func recipeLabels(ctx context.Context, q *dbgen.Queries, user pgtype.UUID, ids []pgtype.UUID) (map[string][]string, map[string][]api.RecipeMark, error) {
	tags, marks := map[string][]string{}, map[string][]api.RecipeMark{}
	if len(ids) == 0 {
		return tags, marks, nil
	}
	trows, err := q.TagsForRecipes(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range trows {
		k := uuidx.String(t.RecipeID)
		tags[k] = append(tags[k], t.Name)
	}
	mrows, err := q.MarksForRecipes(ctx, dbgen.MarksForRecipesParams{UserID: user, Ids: ids})
	if err != nil {
		return nil, nil, err
	}
	for _, m := range mrows {
		k := uuidx.String(m.RecipeID)
		marks[k] = append(marks[k], api.RecipeMark(m.Kind))
	}
	return tags, marks, nil
}

func loadRecipe(ctx context.Context, q *dbgen.Queries, user pgtype.UUID, r dbgen.Recipe) (api.Recipe, error) {
	tags, marks, err := recipeLabels(ctx, q, user, []pgtype.UUID{r.ID})
	if err != nil {
		return api.Recipe{}, err
	}
	ings, err := q.RecipeIngredients(ctx, r.ID)
	if err != nil {
		return api.Recipe{}, err
	}
	steps, err := q.RecipeSteps(ctx, r.ID)
	if err != nil {
		return api.Recipe{}, err
	}
	imgs, err := q.ListRecipeImages(ctx, r.ID)
	if err != nil {
		return api.Recipe{}, err
	}
	k := uuidx.String(r.ID)
	var cover *string
	if len(imgs) > 0 {
		u := imageRef(imgs[0].ID).ThumbUrl
		cover = &u
	}
	sum := toSummary(r, tags[k], marks[k], cover)
	out := api.Recipe{
		Id: sum.Id, Title: sum.Title, Summary: sum.Summary, Servings: sum.Servings,
		PrepMinutes: sum.PrepMinutes, CookMinutes: sum.CookMinutes, SourceKind: sum.SourceKind,
		Tags: sum.Tags, Marks: sum.Marks, CreatedBy: sum.CreatedBy, UpdatedAt: sum.UpdatedAt,
		CoverUrl: sum.CoverUrl, Images: make([]api.Image, 0, len(imgs)),
		SourceUrl: textOf(r.SourceUrl), Notes: r.Notes,
		Ingredients: make([]api.Ingredient, 0, len(ings)), Steps: make([]api.Step, 0, len(steps)),
	}
	for _, im := range imgs {
		out.Images = append(out.Images, imageRef(im.ID))
	}
	for _, i := range ings {
		in := api.Ingredient{Item: i.Item, Quantity: floatOf(i.Quantity)}
		in.Section, in.Unit, in.PrepNote, in.RawText = nonEmpty(i.Section), nonEmpty(i.Unit), nonEmpty(i.PrepNote), nonEmpty(i.RawText)
		out.Ingredients = append(out.Ingredients, in)
	}
	for _, st := range steps {
		out.Steps = append(out.Steps, api.Step{Text: st.Text, TimerSeconds: intOf(st.TimerSeconds)})
	}
	return out, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *server) ListRecipes(ctx context.Context, req api.ListRecipesRequestObject) (api.ListRecipesResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	user, _ := uuidx.Parse(m.ID)
	p := dbgen.SearchRecipesParams{UserID: user}
	if req.Params.Q != nil {
		p.Query = strings.TrimSpace(*req.Params.Q)
	}
	if req.Params.Tag != nil {
		p.Tag = strings.ToLower(strings.TrimSpace(*req.Params.Tag))
	}
	if req.Params.Mark != nil {
		p.Mark = string(*req.Params.Mark)
	}
	rows, err := s.q.SearchRecipes(ctx, p)
	if err != nil {
		return nil, err
	}
	ids := make([]pgtype.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	tags, marks, err := recipeLabels(ctx, s.q, user, ids)
	if err != nil {
		return nil, err
	}
	covers, err := s.q.RecipeCovers(ctx, ids)
	if err != nil {
		return nil, err
	}
	coverOf := map[string]*string{}
	for _, c := range covers {
		u := imageRef(c.ID).ThumbUrl
		coverOf[uuidx.String(c.RecipeID)] = &u
	}
	out := make(api.ListRecipes200JSONResponse, 0, len(rows))
	for _, r := range rows {
		k := uuidx.String(r.ID)
		out = append(out, toSummary(r, tags[k], marks[k], coverOf[k]))
	}
	return out, nil
}

// writeRecipe validates the input and stores it, replacing the ingredients, steps and tags.
func (s *server) writeRecipe(ctx context.Context, q *dbgen.Queries, id pgtype.UUID, user pgtype.UUID, b *api.RecipeInput) (dbgen.Recipe, error) {
	var r dbgen.Recipe
	kind := "manual"
	if b.SourceKind != nil {
		kind = string(*b.SourceKind)
	}
	var err error
	if id.Valid {
		r, err = q.UpdateRecipe(ctx, dbgen.UpdateRecipeParams{
			ID: id, Title: strings.TrimSpace(b.Title), Summary: deref(b.Summary), Servings: numeric(b.Servings),
			PrepMinutes: int4(b.PrepMinutes), CookMinutes: int4(b.CookMinutes), SourceUrl: text(b.SourceUrl),
			SourceKind: kind, Notes: deref(b.Notes),
		})
		if err != nil {
			return r, orNotFound(err, "recipe")
		}
		if err := q.DeleteRecipeIngredients(ctx, id); err != nil {
			return r, err
		}
		if err := q.DeleteRecipeSteps(ctx, id); err != nil {
			return r, err
		}
		if err := q.DeleteRecipeTags(ctx, id); err != nil {
			return r, err
		}
	} else {
		r, err = q.CreateRecipe(ctx, dbgen.CreateRecipeParams{
			Title: strings.TrimSpace(b.Title), Summary: deref(b.Summary), Servings: numeric(b.Servings),
			PrepMinutes: int4(b.PrepMinutes), CookMinutes: int4(b.CookMinutes), SourceUrl: text(b.SourceUrl),
			SourceKind: kind, Notes: deref(b.Notes), CreatedBy: user,
		})
		if err != nil {
			return r, err
		}
	}
	if b.Ingredients != nil {
		for i, in := range *b.Ingredients {
			if err := q.AddRecipeIngredient(ctx, dbgen.AddRecipeIngredientParams{
				RecipeID: r.ID, Position: int32(i), Section: deref(in.Section), Quantity: numeric(in.Quantity),
				Unit: deref(in.Unit), Item: strings.TrimSpace(in.Item), PrepNote: deref(in.PrepNote), RawText: deref(in.RawText),
			}); err != nil {
				return r, err
			}
		}
	}
	if b.Steps != nil {
		for i, st := range *b.Steps {
			if err := q.AddRecipeStep(ctx, dbgen.AddRecipeStepParams{
				RecipeID: r.ID, Position: int32(i), Text: strings.TrimSpace(st.Text), TimerSeconds: int4(st.TimerSeconds),
			}); err != nil {
				return r, err
			}
		}
	}
	if b.Tags != nil {
		seen := map[string]bool{}
		for _, name := range *b.Tags {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			tag, err := q.UpsertTag(ctx, name)
			if err != nil {
				return r, err
			}
			if err := q.AddRecipeTag(ctx, dbgen.AddRecipeTagParams{RecipeID: r.ID, TagID: tag.ID}); err != nil {
				return r, err
			}
		}
	}
	return r, nil
}

func validateRecipe(b *api.RecipeInput) error {
	switch {
	case b == nil || strings.TrimSpace(b.Title) == "":
		return fail(http.StatusBadRequest, "title is required")
	case b.Servings != nil && *b.Servings <= 0:
		return fail(http.StatusBadRequest, "servings must be positive")
	case b.PrepMinutes != nil && *b.PrepMinutes < 0, b.CookMinutes != nil && *b.CookMinutes < 0:
		return fail(http.StatusBadRequest, "minutes cannot be negative")
	}
	if b.Ingredients != nil {
		for _, in := range *b.Ingredients {
			if strings.TrimSpace(in.Item) == "" {
				return fail(http.StatusBadRequest, "every ingredient needs an item")
			}
		}
	}
	if b.Steps != nil {
		for _, st := range *b.Steps {
			if strings.TrimSpace(st.Text) == "" {
				return fail(http.StatusBadRequest, "every step needs text")
			}
			if st.TimerSeconds != nil && *st.TimerSeconds <= 0 {
				return fail(http.StatusBadRequest, "timerSeconds must be positive")
			}
		}
	}
	return nil
}

func (s *server) CreateRecipe(ctx context.Context, req api.CreateRecipeRequestObject) (api.CreateRecipeResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateRecipe(req.Body); err != nil {
		return nil, err
	}
	user, _ := uuidx.Parse(m.ID)
	var out api.Recipe
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		r, err := s.writeRecipe(ctx, q, pgtype.UUID{}, user, req.Body)
		if err != nil {
			return err
		}
		if out, err = loadRecipe(ctx, q, user, r); err != nil {
			return err
		}
		return s.record(ctx, q, m, "recipe_created", pgtype.UUID{}, r.ID, map[string]any{"title": r.Title})
	})
	if err != nil {
		return nil, err
	}
	return api.CreateRecipe201JSONResponse(out), nil
}

func (s *server) GetRecipe(ctx context.Context, req api.GetRecipeRequestObject) (api.GetRecipeResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	r, err := s.q.GetRecipe(ctx, id)
	if err != nil {
		return nil, orNotFound(err, "recipe")
	}
	user, _ := uuidx.Parse(m.ID)
	out, err := loadRecipe(ctx, s.q, user, r)
	if err != nil {
		return nil, err
	}
	return api.GetRecipe200JSONResponse(out), nil
}

func (s *server) ReplaceRecipe(ctx context.Context, req api.ReplaceRecipeRequestObject) (api.ReplaceRecipeResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	if err := validateRecipe(req.Body); err != nil {
		return nil, err
	}
	user, _ := uuidx.Parse(m.ID)
	var out api.Recipe
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		r, err := s.writeRecipe(ctx, q, id, user, req.Body)
		if err != nil {
			return err
		}
		if out, err = loadRecipe(ctx, q, user, r); err != nil {
			return err
		}
		return s.record(ctx, q, m, "recipe_updated", pgtype.UUID{}, r.ID, map[string]any{"title": r.Title})
	})
	if err != nil {
		return nil, err
	}
	return api.ReplaceRecipe200JSONResponse(out), nil
}

func (s *server) DeleteRecipe(ctx context.Context, req api.DeleteRecipeRequestObject) (api.DeleteRecipeResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		r, err := q.GetRecipe(ctx, id)
		if err != nil {
			return orNotFound(err, "recipe")
		}
		if _, err := q.SoftDeleteRecipe(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, q, m, "recipe_deleted", pgtype.UUID{}, id, map[string]any{"title": r.Title})
	})
	if err != nil {
		return nil, err
	}
	return api.DeleteRecipe204Response{}, nil
}

func (s *server) MarkRecipe(ctx context.Context, req api.MarkRecipeRequestObject) (api.MarkRecipeResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	if _, err := s.q.GetRecipe(ctx, id); err != nil {
		return nil, orNotFound(err, "recipe")
	}
	user, _ := uuidx.Parse(m.ID)
	if err := s.q.MarkRecipe(ctx, dbgen.MarkRecipeParams{RecipeID: id, UserID: user, Kind: string(req.Mark)}); err != nil {
		return nil, err
	}
	return api.MarkRecipe204Response{}, nil
}

func (s *server) UnmarkRecipe(ctx context.Context, req api.UnmarkRecipeRequestObject) (api.UnmarkRecipeResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	if _, err := s.q.GetRecipe(ctx, id); err != nil {
		return nil, orNotFound(err, "recipe")
	}
	user, _ := uuidx.Parse(m.ID)
	if err := s.q.UnmarkRecipe(ctx, dbgen.UnmarkRecipeParams{RecipeID: id, UserID: user, Kind: string(req.Mark)}); err != nil {
		return nil, err
	}
	return api.UnmarkRecipe204Response{}, nil
}

func (s *server) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	names, err := s.q.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	if names == nil {
		names = []string{}
	}
	return api.ListTags200JSONResponse(names), nil
}

func (s *server) AddRecipeToList(ctx context.Context, req api.AddRecipeToListRequestObject) (api.AddRecipeToListResponseObject, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	listID, err := idOf(req.ListId, "list")
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, fail(http.StatusBadRequest, "invalid body")
	}
	recipeID, err := idOf(req.Body.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	if req.Body.Servings != nil && *req.Body.Servings <= 0 {
		return nil, fail(http.StatusBadRequest, "servings must be positive")
	}
	user, _ := uuidx.Parse(m.ID)
	res := api.AddRecipeResult{Added: []api.Item{}, Merged: []api.Item{}}
	// Everything this call changes shares one batch, so it can be undone as a group.
	if !batchOf(ctx).Valid {
		ctx = context.WithValue(ctx, batchKey{}, uuidx.New())
	}
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		if _, err := q.GetList(ctx, listID); err != nil {
			return orNotFound(err, "list")
		}
		recipe, err := q.GetRecipe(ctx, recipeID)
		if err != nil {
			return orNotFound(err, "recipe")
		}
		ings, err := q.RecipeIngredients(ctx, recipeID)
		if err != nil {
			return err
		}
		recipeServings, want := 0.0, 0.0
		if v := floatOf(recipe.Servings); v != nil {
			recipeServings, want = *v, *v
		}
		if req.Body.Servings != nil {
			want = *req.Body.Servings
		}

		addedAt, mergedAt := map[string]int{}, map[string]int{}
		for _, in := range ings {
			qtyText := in.RawText
			if v := floatOf(in.Quantity); v != nil {
				qtyText = listsvc.Format(listsvc.Scale(*v, recipeServings, want), in.Unit)
			} else if in.Unit != "" {
				qtyText = in.Unit
			}
			name := in.Item
			norm := listsvc.Normalize(name)

			open, err := q.FindOpenItemByName(ctx, dbgen.FindOpenItemByNameParams{ListID: listID, Name: norm})
			if err == nil {
				item, err := q.UpdateItem(ctx, dbgen.UpdateItemParams{
					ID: open.ID, QuantityText: pgtype.Text{String: listsvc.Merge(open.QuantityText, qtyText), Valid: true},
				})
				if err != nil {
					return err
				}
				if err := s.record(ctx, q, m, "item_merged", listID, recipeID, itemDetail(item, open)); err != nil {
					return err
				}
				k := uuidx.String(item.ID)
				if i, ok := addedAt[k]; ok {
					res.Added[i] = toItem(item)
				} else if i, ok := mergedAt[k]; ok {
					res.Merged[i] = toItem(item)
				} else {
					mergedAt[k] = len(res.Merged)
					res.Merged = append(res.Merged, toItem(item))
				}
				continue
			} else if err != pgx.ErrNoRows {
				return err
			}
			item, err := addItem(ctx, q, listID, user, uuidx.New(), name, qtyText, in.PrepNote, nil, recipeID)
			if err != nil {
				return err
			}
			if err := s.record(ctx, q, m, "item_added", listID, recipeID, map[string]any{"item": item.Name, "itemId": uuidx.String(item.ID)}); err != nil {
				return err
			}
			addedAt[uuidx.String(item.ID)] = len(res.Added)
			res.Added = append(res.Added, toItem(item))
		}
		return s.record(ctx, q, m, "recipe_added_to_list", listID, recipeID, map[string]any{
			"title": recipe.Title, "added": len(res.Added), "merged": len(res.Merged),
		})
	})
	if err != nil {
		return nil, err
	}
	return api.AddRecipeToList200JSONResponse(res), nil
}
