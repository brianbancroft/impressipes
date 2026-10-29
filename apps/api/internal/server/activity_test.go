package server

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUndoItemChanges(t *testing.T) {
	e := newDataEnv(t)
	var list, item obj
	e.call("POST", "/lists", `{"name":"L","colour":"#000"}`, 201, &list)
	lid := list["id"].(string)
	e.call("POST", "/lists/"+lid+"/items", `{"name":"Milk","quantityText":"2"}`, 201, &item)
	iid := item["id"].(string)

	var detail struct{ Items []obj }
	itemsNow := func() []obj {
		detail.Items = nil
		e.call("GET", "/lists/"+lid, "", 200, &detail)
		return detail.Items
	}
	lastActivity := func(action string) obj {
		var acts []obj
		e.call("GET", "/activity?action="+action, "", 200, &acts)
		if len(acts) == 0 {
			t.Fatalf("no %s activity", action)
		}
		return acts[0]
	}
	id := func(a obj) string { return fmt.Sprintf("%.0f", a["id"].(float64)) }

	// An edit records before and after, and undo puts the old values back.
	e.call("PATCH", "/items/"+iid, `{"name":"Oat milk","quantityText":"3","note":"ripe","checked":true}`, 200, nil)
	edit := lastActivity("item_checked")
	d := edit["detail"].(obj)
	if d["before"].(obj)["name"] != "Milk" || d["after"].(obj)["name"] != "Oat milk" || edit["undoable"] != true {
		t.Fatalf("edit detail: %v", edit)
	}
	var undone obj
	e.call("POST", "/activity/"+id(edit)+"/undo", "{}", 200, &undone)
	if undone["action"] != "undo" || undone["undoOf"] != edit["id"] {
		t.Fatalf("undo row: %v", undone)
	}
	got := itemsNow()[0]
	if got["name"] != "Milk" || got["quantityText"] != "2" || got["note"] != "" || got["checked"] != false {
		t.Fatalf("after undo: %v", got)
	}
	// The original is kept, marked undone, and cannot be undone twice.
	if a := lastActivity("item_checked"); a["undoneAt"] == nil || a["undoable"] != false {
		t.Fatalf("original not stamped: %v", a)
	}
	e.call("POST", "/activity/"+id(edit)+"/undo", "{}", 409, nil)
	e.call("POST", "/activity/"+id(undone)+"/undo", "{}", 422, nil)

	// Deleting and undoing restores the item.
	e.call("DELETE", "/items/"+iid, "", 204, nil)
	if len(itemsNow()) != 0 {
		t.Fatal("item should be deleted")
	}
	e.call("POST", "/activity/"+id(lastActivity("item_deleted"))+"/undo", "{}", 200, nil)
	if len(itemsNow()) != 1 {
		t.Fatal("item should be restored")
	}

	// A later change to the same item is a conflict unless forced.
	add := lastActivity("item_added")
	e.call("PATCH", "/items/"+iid, `{"note":"later"}`, 200, nil)
	e.call("POST", "/activity/"+id(add)+"/undo", "{}", 409, nil)
	e.call("POST", "/activity/"+id(add)+"/undo", `{"force":true}`, 200, nil)
	if len(itemsNow()) != 0 {
		t.Fatal("forced undo should remove the item")
	}

	// List edits are audited but cannot be undone.
	e.call("PATCH", "/lists/"+lid, `{"name":"Renamed"}`, 200, nil)
	e.call("POST", "/activity/"+id(lastActivity("list_updated"))+"/undo", "{}", 422, nil)
	e.call("POST", "/activity/999999/undo", "{}", 404, nil)
}

func TestUndoRecipeBatchAndFeed(t *testing.T) {
	e := newDataEnv(t)
	var list, r, om obj
	e.call("POST", "/lists", `{"name":"Groceries","colour":"#000"}`, 201, &list)
	lid := list["id"].(string)
	e.call("POST", "/recipes", `{"title":"Eggs on toast","servings":2,"ingredients":[{"quantity":2,"item":"Eggs"},{"item":"Bread","rawText":"2 slices"}],"steps":[{"text":"Cook."}]}`, 201, &r)
	e.call("POST", "/recipes", `{"title":"Omelette","servings":2,"ingredients":[{"quantity":3,"item":"Eggs"}],"steps":[{"text":"Cook."}]}`, 201, &om)

	// First add creates two items; the second merges eggs. Each call is one batch.
	e.call("POST", "/lists/"+lid+"/add-recipe", `{"recipeId":"`+r["id"].(string)+`"}`, 200, nil)
	e.call("POST", "/lists/"+lid+"/add-recipe", `{"recipeId":"`+om["id"].(string)+`"}`, 200, nil)

	var feed []obj
	e.call("GET", "/activity?action=recipe_added_to_list", "", 200, &feed)
	if len(feed) != 2 || feed[0]["batchId"] == nil || feed[0]["batchId"] == feed[1]["batchId"] {
		t.Fatalf("batches: %v", feed)
	}
	merge, first := feed[0]["batchId"].(string), feed[1]["batchId"].(string)

	var detail struct{ Items []obj }
	qty := func() map[string]string {
		detail.Items = nil
		e.call("GET", "/lists/"+lid, "", 200, &detail)
		m := map[string]string{}
		for _, it := range detail.Items {
			m[it["name"].(string)] = it["quantityText"].(string)
		}
		return m
	}
	if q := qty(); q["Eggs"] != "5" || len(q) != 2 {
		t.Fatalf("before undo: %v", q)
	}

	// The first batch's eggs were merged into by the second, so undoing it conflicts and changes nothing.
	e.call("POST", "/activity/batches/"+first+"/undo", "{}", 409, nil)
	if q := qty(); q["Eggs"] != "5" || len(q) != 2 {
		t.Fatalf("conflicting batch undo must be all-or-nothing: %v", q)
	}

	// Undo the merge, then the original adds.
	var undone []obj
	e.call("POST", "/activity/batches/"+merge+"/undo", "{}", 200, &undone)
	if len(undone) != 1 || qty()["Eggs"] != "2" {
		t.Fatalf("merge undo: %v %v", undone, qty())
	}
	e.call("POST", "/activity/batches/"+first+"/undo", "{}", 200, &undone)
	if len(undone) != 2 || len(qty()) != 0 {
		t.Fatalf("batch undo: %v %v", undone, qty())
	}
	e.call("POST", "/activity/batches/"+first+"/undo", "{}", 200, &undone) // nothing left
	if len(undone) != 0 {
		t.Fatalf("repeat batch undo: %v", undone)
	}
	e.call("POST", "/activity/batches/00000000-0000-4000-8000-000000000000/undo", "{}", 404, nil)

	// A client-supplied batch groups separate calls.
	batched := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/lists/"+lid+"/items", strings.NewReader(body))
		req.AddCookie(e.cookie)
		req.Header.Set("X-Impressipies-Batch", "5f7c5d1e-2d0e-4d54-9c1d-0a2f6a5c1b11")
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		return rec
	}
	rec, rec2 := batched(`{"name":"Apples"}`), batched(`{"name":"Pears"}`)
	if rec.Code != 201 || rec2.Code != 201 {
		t.Fatalf("batched adds: %d %d", rec.Code, rec2.Code)
	}
	e.call("POST", "/activity/batches/5f7c5d1e-2d0e-4d54-9c1d-0a2f6a5c1b11/undo", "{}", 200, &undone)
	if len(undone) != 2 {
		t.Fatalf("client batch: %v", undone)
	}

	// Recipe deletion is restorable.
	e.call("DELETE", "/recipes/"+r["id"].(string), "", 204, nil)
	e.call("GET", "/recipes/"+r["id"].(string), "", 404, nil)
	var del []obj
	e.call("GET", "/activity?action=recipe_deleted", "", 200, &del)
	e.call("POST", "/activity/"+fmt.Sprintf("%.0f", del[0]["id"].(float64))+"/undo", "{}", 200, nil)
	e.call("GET", "/recipes/"+r["id"].(string), "", 200, nil)

	// Feed filters: client, person, date.
	var rows []obj
	e.call("GET", "/activity?via=web&limit=3", "", 200, &rows)
	if len(rows) != 3 {
		t.Fatalf("limit: %d", len(rows))
	}
	e.call("GET", "/activity?via=ios", "", 200, &rows)
	if len(rows) != 0 {
		t.Fatalf("via=ios: %v", rows)
	}
	e.call("GET", "/activity?userId=not-a-uuid", "", 400, nil)
	e.call("GET", "/activity?since=2999-01-01T00:00:00Z", "", 200, &rows)
	if len(rows) != 0 {
		t.Fatalf("since future: %v", rows)
	}
}

func itoa(v any) string { return fmt.Sprintf("%.0f", v.(float64)) }
