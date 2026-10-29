package pairing

import "testing"

var corpus = []Recipe{
	{ID: "1", Title: "Tacos", Ingredients: []string{"ground beef", "tortillas", "fresh cilantro", "limes", "onion", "salt"}, Tags: []string{"mexican", "weeknight"}},
	{ID: "2", Title: "Burrito bowls", Ingredients: []string{"rice", "ground beef", "lime", "chopped cilantro", "black beans", "olive oil"}, Tags: []string{"mexican"}},
	{ID: "3", Title: "Pad thai", Ingredients: []string{"rice noodles", "shrimp", "lime", "peanuts", "eggs"}, Tags: []string{"thai"}},
	{ID: "4", Title: "Pancakes", Ingredients: []string{"flour", "milk", "eggs", "sugar", "butter"}, Tags: []string{"breakfast"}},
}

func TestKey(t *testing.T) {
	for in, want := range map[string]string{
		"Fresh Cilantro": "cilantro", "limes": "lime", "tomatoes": "tomato", "berries": "berry",
		"chopped yellow onions": "yellow onion", "molasses": "molasses", "": "",
	} {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSimilarRanksByOverlapAndSkipsTargets(t *testing.T) {
	got := Similar([]Recipe{corpus[0]}, corpus, 0)
	if len(got) == 0 || got[0].Recipe.ID != "2" {
		t.Fatalf("want burrito bowls first, got %+v", got)
	}
	for _, m := range got {
		if m.Recipe.ID == "1" {
			t.Fatal("target must not match itself")
		}
		if m.Recipe.ID == "4" {
			t.Fatal("pancakes share only staples and no tags, so they must not match")
		}
	}
	shared := map[string]bool{}
	for _, k := range got[0].Shared {
		shared[k] = true
	}
	for _, k := range []string{"ground beef", "cilantro", "lime"} {
		if !shared[k] {
			t.Errorf("shared is missing %q: %v", k, got[0].Shared)
		}
	}
	if len(got[0].SharedT) != 1 || got[0].SharedT[0] != "mexican" {
		t.Errorf("shared tags = %v", got[0].SharedT)
	}
}

func TestSimilarPoolsSeveralTargetsAndLimits(t *testing.T) {
	got := Similar([]Recipe{corpus[0], corpus[3]}, corpus, 1)
	if len(got) != 1 {
		t.Fatalf("limit not applied: %d", len(got))
	}
}

func TestPairings(t *testing.T) {
	res := Pairings("Limes", corpus, 3)
	if len(res.Recipes) != 3 {
		t.Fatalf("recipes using lime = %d, want 3", len(res.Recipes))
	}
	if len(res.Partners) != 3 {
		t.Fatalf("limit not applied: %d", len(res.Partners))
	}
	top := res.Partners[0]
	if top.Ingredient != "cilantro" && top.Ingredient != "ground beef" {
		t.Errorf("top partner = %q, want cilantro or ground beef", top.Ingredient)
	}
	if top.Recipes != 2 {
		t.Errorf("top partner count = %d, want 2", top.Recipes)
	}
	for _, p := range res.Partners {
		if p.Ingredient == "lime" {
			t.Error("the ingredient is not its own partner")
		}
	}
}

func TestPairingsSubstringAndUnknown(t *testing.T) {
	if res := Pairings("noodle", corpus, 0); len(res.Recipes) != 1 || res.Recipes[0].ID != "3" {
		t.Errorf("substring match failed: %+v", res.Recipes)
	}
	if res := Pairings("saffron", corpus, 0); len(res.Recipes) != 0 || len(res.Partners) != 0 {
		t.Errorf("unknown ingredient should find nothing: %+v", res)
	}
	if res := Pairings("  ", corpus, 0); len(res.Recipes) != 0 {
		t.Errorf("blank should find nothing")
	}
}

func TestPairingsForAStapleStillWorks(t *testing.T) {
	res := Pairings("flour", corpus, 0)
	if len(res.Recipes) != 1 || res.Recipes[0].ID != "4" {
		t.Errorf("flour should find pancakes: %+v", res.Recipes)
	}
}
