package server

import (
	"testing"
)

func TestMealPlanCookedAndOnSale(t *testing.T) {
	e := newDataEnv(t)

	var recipe obj
	e.call("POST", "/recipes", `{"title":"Mexican Chicken Fajitas","prepMinutes":20,"cookMinutes":15}`, 201, &recipe)
	recipeID := recipe["id"].(string)
	if recipe["timesCooked"].(float64) != 0 || recipe["lastCookedAt"] != nil {
		t.Fatalf("new recipe cook count: %v", recipe)
	}

	// Free-text meals have no recipe. Linked meals keep their own title.
	var tacos obj
	e.call("PUT", "/meal-plan/entries", `{"planDate":"2026-11-08","slot":"dinner","title":"Tacos"}`, 200, &tacos)
	if tacos["recipeId"] != nil || tacos["title"] != "Tacos" || tacos["completed"] != false || tacos["slot"] != "dinner" {
		t.Fatalf("free-text meal: %v", tacos)
	}
	var fajitas obj
	e.call("PUT", "/meal-plan/entries", `{"planDate":"2026-11-09","slot":"dinner","title":"Fajita night","recipeId":"`+recipeID+`"}`, 200, &fajitas)
	if fajitas["recipeId"] != recipeID || fajitas["recipe"].(obj)["title"] != "Mexican Chicken Fajitas" {
		t.Fatalf("linked meal: %v", fajitas)
	}
	// Replacing the same slot keeps completion untouched until it is patched.
	e.call("PATCH", "/meal-plan/entries/"+fajitas["id"].(string), `{"completed":true}`, 200, &fajitas)
	if fajitas["completed"] != true || fajitas["completedAt"] == nil || fajitas["completedBy"] == nil {
		t.Fatalf("complete: %v", fajitas)
	}
	var replaced obj
	e.call("PUT", "/meal-plan/entries", `{"planDate":"2026-11-09","slot":"dinner","title":"Fajita night","recipeId":"`+recipeID+`"}`, 200, &replaced)
	if replaced["id"] != fajitas["id"] || replaced["completed"] != true {
		t.Fatalf("upsert changed completion or id: %v", replaced)
	}

	var week []obj
	e.call("GET", "/meal-plan?from=2026-11-08&to=2026-11-14", "", 200, &week)
	if len(week) != 2 || week[0]["title"] != "Tacos" || week[1]["title"] != "Fajita night" {
		t.Fatalf("week: %v", week)
	}
	e.call("GET", "/meal-plan?from=2026-11-14&to=2026-11-08", "", 400, nil)
	e.call("PUT", "/meal-plan/entries", `{"planDate":"2026-11-08","slot":"brunch","title":"Nope"}`, 400, nil)
	e.call("PUT", "/meal-plan/entries", `{"planDate":"2026-11-08","slot":"lunch","title":"  "}`, 400, nil)

	// Completing a planned meal does not count as cooking the recipe.
	var still obj
	e.call("GET", "/recipes/"+recipeID, "", 200, &still)
	if still["timesCooked"].(float64) != 0 {
		t.Fatalf("completion incremented timesCooked: %v", still["timesCooked"])
	}
	var cooked obj
	e.call("POST", "/recipes/"+recipeID+"/cooked", "", 200, &cooked)
	e.call("POST", "/recipes/"+recipeID+"/cooked", "", 200, &cooked)
	if cooked["timesCooked"].(float64) != 2 || cooked["lastCookedAt"] == nil {
		t.Fatalf("cooked: %v", cooked)
	}
	e.call("GET", "/recipes/"+recipeID, "", 200, &still)
	if still["timesCooked"].(float64) != 2 {
		t.Fatalf("recipe timesCooked: %v", still["timesCooked"])
	}

	// Unchecking is allowed, and clearing the recipe leaves the title.
	var cleared obj
	e.call("PATCH", "/meal-plan/entries/"+fajitas["id"].(string), `{"completed":false,"recipeId":""}`, 200, &cleared)
	if cleared["completed"] != false || cleared["recipeId"] != nil || cleared["title"] != "Fajita night" {
		t.Fatalf("clear: %v", cleared)
	}
	e.call("PATCH", "/meal-plan/entries/"+fajitas["id"].(string), `{"recipeId":"`+recipeID+`"}`, 200, &cleared)

	// Deleting the recipe drops the link and keeps the planned title.
	e.call("DELETE", "/recipes/"+recipeID, "", 204, nil)
	e.call("GET", "/meal-plan?from=2026-11-09&to=2026-11-09", "", 200, &week)
	if len(week) != 1 || week[0]["title"] != "Fajita night" || week[0]["recipeId"] != nil {
		t.Fatalf("after recipe delete: %v", week)
	}
	e.call("POST", "/recipes/"+recipeID+"/cooked", "", 404, nil)
	e.call("DELETE", "/meal-plan/entries/"+tacos["id"].(string), "", 204, nil)
	e.call("DELETE", "/meal-plan/entries/"+tacos["id"].(string), "", 404, nil)

	// On sale only is optional, visible on the item, and not cleared by an unrelated edit.
	var list obj
	e.call("POST", "/lists", `{"name":"Costco","colour":"#111"}`, 201, &list)
	listID := list["id"].(string)
	var sale obj
	e.call("POST", "/lists/"+listID+"/items", `{"name":"Avocados","quantityText":"4","onSaleOnly":true}`, 201, &sale)
	if sale["onSaleOnly"] != true {
		t.Fatalf("create on sale: %v", sale)
	}
	var plain obj
	e.call("POST", "/lists/"+listID+"/items", `{"name":"Chicken breast"}`, 201, &plain)
	if plain["onSaleOnly"] != false {
		t.Fatalf("default on sale: %v", plain)
	}
	e.call("PATCH", "/items/"+sale["id"].(string), `{"note":"ripe"}`, 200, &sale)
	if sale["onSaleOnly"] != true || sale["note"] != "ripe" {
		t.Fatalf("unrelated edit cleared the flag: %v", sale)
	}
	e.call("PATCH", "/items/"+sale["id"].(string), `{"onSaleOnly":false}`, 200, &sale)
	if sale["onSaleOnly"] != false {
		t.Fatalf("toggle off: %v", sale)
	}
	e.call("PATCH", "/items/"+sale["id"].(string), `{"onSaleOnly":true}`, 200, &sale)

	var changes struct {
		Items []obj `json:"items"`
	}
	e.call("GET", "/sync?since=0", "", 200, &changes)
	var seen bool
	for _, it := range changes.Items {
		if it["id"] == sale["id"] {
			seen = it["onSaleOnly"] == true
		}
	}
	if !seen {
		t.Fatalf("sync omitted onSaleOnly: %v", changes.Items)
	}
}
