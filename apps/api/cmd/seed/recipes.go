package main

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ingredient struct {
	section  string
	quantity float64
	unit     string
	item     string
	prepNote string
}

type step struct {
	text  string
	timer int // seconds, 0 for none
}

type recipe struct {
	id, title, summary       string
	servings                 float64
	prepMinutes, cookMinutes int
	tags                     []string
	ingredients              []ingredient
	steps                    []step
}

// seedRecipes' ids are fixed so a re-run can tell each one is already loaded.
var seedRecipes = []recipe{
	{
		id: "5eed0000-0000-4000-8000-0000000000a1", title: "Weeknight tomato pasta",
		summary:  "A quick garlicky tomato sauce that is on the table in half an hour.",
		servings: 4, prepMinutes: 10, cookMinutes: 20,
		tags: []string{"weeknight", "vegetarian", "pasta"},
		ingredients: []ingredient{
			{quantity: 400, unit: "g", item: "spaghetti"},
			{quantity: 3, unit: "tbsp", item: "olive oil"},
			{quantity: 4, unit: "cloves", item: "garlic", prepNote: "thinly sliced"},
			{quantity: 800, unit: "g", item: "canned whole tomatoes", prepNote: "crushed by hand"},
			{quantity: 1, unit: "small bunch", item: "basil", prepNote: "torn"},
			{quantity: 40, unit: "g", item: "parmesan", prepNote: "grated"},
		},
		steps: []step{
			{text: "Bring a large pot of well-salted water to a boil and cook the spaghetti until just shy of tender.", timer: 600},
			{text: "Meanwhile, warm the olive oil in a wide pan over medium heat and cook the garlic until fragrant, about a minute."},
			{text: "Add the tomatoes and a pinch of salt. Simmer, stirring now and then, until the sauce thickens.", timer: 900},
			{text: "Toss the drained pasta through the sauce with a splash of the pasta water. Finish with basil and parmesan."},
		},
	},
	{
		id: "5eed0000-0000-4000-8000-0000000000a2", title: "Sheet-pan lemon chicken and potatoes",
		summary:  "Crisp-skinned chicken thighs roasted over lemony potatoes, all on one tray.",
		servings: 4, prepMinutes: 15, cookMinutes: 45,
		tags: []string{"weeknight", "chicken", "one-pan"},
		ingredients: []ingredient{
			{quantity: 8, unit: "", item: "bone-in chicken thighs"},
			{quantity: 700, unit: "g", item: "baby potatoes", prepNote: "halved"},
			{quantity: 2, unit: "", item: "lemons", prepNote: "1 zested and juiced, 1 sliced"},
			{quantity: 4, unit: "tbsp", item: "olive oil"},
			{quantity: 1, unit: "tbsp", item: "dried oregano"},
			{quantity: 5, unit: "cloves", item: "garlic", prepNote: "smashed"},
		},
		steps: []step{
			{text: "Heat the oven to 220°C (425°F)."},
			{text: "Toss the potatoes with half the oil, the lemon zest and juice, garlic, oregano, salt and pepper on a large sheet pan."},
			{text: "Rub the chicken with the remaining oil and season well. Nestle it skin-side up among the potatoes and tuck in the lemon slices."},
			{text: "Roast until the skin is deep golden and the chicken is cooked through.", timer: 2700},
			{text: "Rest for five minutes, then spoon the pan juices over everything."},
		},
	},
	{
		id: "5eed0000-0000-4000-8000-0000000000a3", title: "Banana oat pancakes",
		summary:  "Fluffy, lightly sweet pancakes made with ripe bananas and rolled oats.",
		servings: 4, prepMinutes: 10, cookMinutes: 15,
		tags: []string{"breakfast", "vegetarian"},
		ingredients: []ingredient{
			{quantity: 2, unit: "", item: "ripe bananas", prepNote: "mashed"},
			{quantity: 1, unit: "cup", item: "rolled oats"},
			{quantity: 1, unit: "cup", item: "milk"},
			{quantity: 2, unit: "", item: "eggs"},
			{quantity: 2, unit: "tsp", item: "baking powder"},
			{quantity: 1, unit: "tbsp", item: "butter", prepNote: "for the pan"},
		},
		steps: []step{
			{text: "Blend or whisk the bananas, oats, milk, eggs and baking powder into a thick batter."},
			{text: "Let the batter rest while the pan heats.", timer: 300},
			{text: "Melt a little butter in a non-stick pan over medium heat. Cook scoops of batter until bubbles form, then flip and cook until golden.", timer: 240},
			{text: "Serve warm with fruit or maple syrup."},
		},
	},
}

// seedRecipesOnce adds any missing sample recipe, owned by the earliest enabled member.
func seedRecipesOnce(ctx context.Context, pool *pgxpool.Pool) {
	var owner string
	err := pool.QueryRow(ctx, `SELECT id FROM users WHERE disabled_at IS NULL ORDER BY created_at LIMIT 1`).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Print("seed: no member yet, so no sample recipes; sign up, then run the seed again")
		return
	}
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	loaded := 0
	for _, r := range seedRecipes {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM recipes WHERE id = $1)`, r.id).Scan(&exists); err != nil {
			log.Fatalf("seed: %v", err)
		}
		if exists {
			continue
		}
		if err := insertRecipe(ctx, pool, owner, r); err != nil {
			log.Fatalf("seed: recipe %q: %v", r.title, err)
		}
		loaded++
	}
	log.Printf("seed: loaded %d sample recipes", loaded)
}

func insertRecipe(ctx context.Context, pool *pgxpool.Pool, owner string, r recipe) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO recipes (id, title, summary, servings, prep_minutes, cook_minutes, source_kind, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, 'manual', $7)`,
		r.id, r.title, r.summary, r.servings, r.prepMinutes, r.cookMinutes, owner); err != nil {
		return err
	}
	for i, in := range r.ingredients {
		if _, err := tx.Exec(ctx,
			`INSERT INTO recipe_ingredients (recipe_id, position, section, quantity, unit, item, prep_note)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			r.id, i, in.section, in.quantity, in.unit, in.item, in.prepNote); err != nil {
			return err
		}
	}
	for i, st := range r.steps {
		var timer any
		if st.timer > 0 {
			timer = st.timer
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO recipe_steps (recipe_id, position, text, timer_seconds) VALUES ($1, $2, $3, $4)`,
			r.id, i, st.text, timer); err != nil {
			return err
		}
	}
	for _, tag := range r.tags {
		var tagID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, tag).Scan(&tagID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO recipe_tags (recipe_id, tag_id) VALUES ($1, $2)`, r.id, tagID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
