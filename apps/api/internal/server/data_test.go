package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbtest"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

// dataEnv is an API on a throwaway Postgres with one signed-in admin.
type dataEnv struct {
	t      *testing.T
	h      http.Handler
	cookie *http.Cookie
}

func newDataEnv(t *testing.T) *dataEnv {
	t.Helper()
	pool := dbtest.Pool(t)
	dir := family.NewPG(pool)
	h := NewWithStore(t.Context(), dir, pool)
	inv, err := dir.CreateInvite(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	rec := do(h, "POST", "/invites/"+inv.Token+"/accept", `{"name":"Mum","color":"#e11d48","credentialId":"c1","email":"mum@example.com","password":"long enough pw"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	return &dataEnv{t: t, h: h, cookie: sessionOf(t, rec)}
}

// call sends a request as the admin, checks the status and decodes the JSON body into out.
func (e *dataEnv) call(method, path, body string, want int, out any) *httptest.ResponseRecorder {
	e.t.Helper()
	rec := do(e.h, method, path, body, e.cookie)
	if rec.Code != want {
		e.t.Fatalf("%s %s: got %d, want %d: %s", method, path, rec.Code, want, rec.Body)
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			e.t.Fatalf("%s %s: decode %q: %v", method, path, rec.Body, err)
		}
	}
	return rec
}

type obj = map[string]any

func TestDataRequiresSession(t *testing.T) {
	h := NewWithStore(t.Context(), family.NewStore(), nil)
	if rec := do(h, "GET", "/lists", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: got %d, want 401", rec.Code)
	}
	// Signed in but no database: the data endpoints say so.
	store := family.NewStore()
	h = NewWithStore(t.Context(), store, nil)
	inv, _ := store.CreateInvite(t.Context(), "")
	rec := do(h, "POST", "/invites/"+inv.Token+"/accept", `{"name":"A","color":"#000","credentialId":"c","email":"a@example.com","password":"long enough pw"}`)
	if rec := do(h, "GET", "/lists", "", sessionOf(t, rec)); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no database: got %d, want 503", rec.Code)
	}
}

func TestListsItemsAndMemory(t *testing.T) {
	e := newDataEnv(t)

	var list obj
	e.call("POST", "/lists", `{"name":"Groceries","emoji":"🛒","colour":"#16a34a","categories":[{"name":"Produce","colour":"#22c55e"},{"name":"Dairy","colour":"#3b82f6"}]}`, 201, &list)
	listID := list["id"].(string)
	e.call("POST", "/lists", `{"name":"","colour":"#000"}`, 400, nil)

	var detail struct {
		Categories []obj
		Items      []obj
	}
	e.call("GET", "/lists/"+listID, "", 200, &detail)
	if len(detail.Categories) != 2 || len(detail.Items) != 0 {
		t.Fatalf("detail: %+v", detail)
	}
	produce := detail.Categories[0]["id"].(string)

	// Client-supplied id makes create idempotent.
	itemID := "5b1f1f6e-0a54-4d3c-9d0e-3f0f8a7c1a01"
	body := `{"id":"` + itemID + `","name":"Bananas","quantityText":"6","categoryId":"` + produce + `"}`
	var item obj
	e.call("POST", "/lists/"+listID+"/items", body, 201, &item)
	var again obj
	e.call("POST", "/lists/"+listID+"/items", body, 201, &again)
	if again["version"] != item["version"] {
		t.Fatalf("retry changed the item: %v vs %v", again, item)
	}
	e.call("GET", "/lists/"+listID, "", 200, &detail)
	if len(detail.Items) != 1 {
		t.Fatalf("retried create made %d items", len(detail.Items))
	}

	// Item memory fills a missing category and quantity next time.
	var second obj
	e.call("POST", "/lists/"+listID+"/items", `{"name":"  bananas "}`, 201, &second)
	if second["categoryId"] != produce || second["quantityText"] != "6" {
		t.Fatalf("memory not applied: %v", second)
	}
	var sugg []obj
	e.call("GET", "/lists/"+listID+"/suggestions?q=Ban", "", 200, &sugg)
	if len(sugg) != 1 || sugg[0]["name"] != "bananas" {
		t.Fatalf("suggestions: %v", sugg)
	}

	// A category from another list is rejected.
	var other obj
	e.call("POST", "/lists", `{"name":"Hardware","colour":"#000","categories":[{"name":"Tools","colour":"#111"}]}`, 201, &other)
	var otherDetail struct{ Categories []obj }
	e.call("GET", "/lists/"+other["id"].(string), "", 200, &otherDetail)
	e.call("POST", "/lists/"+listID+"/items", `{"name":"Hammer","categoryId":"`+otherDetail.Categories[0]["id"].(string)+`"}`, 400, nil)

	// Check, edit, clear category.
	var upd obj
	e.call("PATCH", "/items/"+itemID, `{"checked":true,"note":"ripe","categoryId":""}`, 200, &upd)
	if upd["checked"] != true || upd["note"] != "ripe" || upd["categoryId"] != nil || upd["checkedBy"] == nil {
		t.Fatalf("patch: %v", upd)
	}
	upd = nil
	e.call("PATCH", "/items/"+itemID, `{"checked":false}`, 200, &upd)
	if upd["checked"] != false || upd["checkedBy"] != nil {
		t.Fatalf("uncheck: %v", upd)
	}

	// Deleting a category uncategorises its items.
	e.call("PATCH", "/items/"+itemID, `{"categoryId":"`+produce+`"}`, 200, nil)
	e.call("DELETE", "/lists/"+listID+"/categories/"+produce, "", 204, nil)
	e.call("GET", "/items/"+itemID, "", 405, nil)
	detail.Items = nil
	e.call("GET", "/lists/"+listID, "", 200, &detail)
	for _, it := range detail.Items {
		if it["categoryId"] != nil {
			t.Fatalf("item kept deleted category: %v", it)
		}
	}

	e.call("DELETE", "/items/"+itemID, "", 204, nil)
	e.call("DELETE", "/items/"+itemID, "", 404, nil)

	// Archived lists disappear.
	e.call("DELETE", "/lists/"+other["id"].(string), "", 204, nil)
	var lists []obj
	e.call("GET", "/lists", "", 200, &lists)
	if len(lists) != 1 {
		t.Fatalf("lists after archive: %v", lists)
	}
	e.call("GET", "/lists/not-a-uuid", "", 404, nil)

	var acts []obj
	e.call("GET", "/lists/"+listID+"/activity", "", 200, &acts)
	if len(acts) < 5 || acts[0]["action"] != "item_deleted" {
		t.Fatalf("activity: %v", acts)
	}
}

func TestSync(t *testing.T) {
	e := newDataEnv(t)
	var list obj
	e.call("POST", "/lists", `{"name":"L","colour":"#000"}`, 201, &list)
	lid := list["id"].(string)
	a, b := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"

	ops := `{"ops":[
		{"type":"upsert","itemId":"` + a + `","listId":"` + lid + `","item":{"name":"Milk","quantityText":"2"}},
		{"type":"upsert","itemId":"` + b + `","listId":"` + lid + `","item":{"name":"Eggs"}},
		{"type":"check","itemId":"` + a + `"},
		{"type":"upsert","itemId":"` + a + `","listId":"` + lid + `","item":{"name":"Oat milk"}},
		{"type":"check","itemId":"33333333-3333-4333-8333-333333333333"},
		{"type":"upsert","itemId":"` + b + `","item":{"name":"no list"}},
		{"type":"delete","itemId":"` + b + `"}
	]}`
	var res struct {
		Version int64
		Results []obj
	}
	e.call("POST", "/sync", ops, 200, &res)
	want := []string{"applied", "applied", "applied", "applied", "not_found", "invalid", "applied"}
	for i, r := range res.Results {
		if r["status"] != want[i] {
			t.Errorf("op %d: %v, want %s", i, r["status"], want[i])
		}
	}
	// Replaying the batch is safe.
	var replay struct {
		Version int64
		Results []obj
	}
	e.call("POST", "/sync", ops, 200, &replay)
	for i, r := range replay.Results {
		if r["status"] != want[i] {
			t.Errorf("replay op %d: %v, want %s", i, r["status"], want[i])
		}
	}

	var pull struct {
		Version int64
		Items   []obj
	}
	e.call("GET", "/sync?since=0", "", 200, &pull)
	if len(pull.Items) != 2 || pull.Version != replay.Version {
		t.Fatalf("pull: version %d (want %d), %d items", pull.Version, replay.Version, len(pull.Items))
	}
	byID := map[string]obj{}
	for _, it := range pull.Items {
		byID[it["id"].(string)] = it
	}
	if it := byID[a]; it["name"] != "Oat milk" || it["checked"] != true || it["quantityText"] != "2" {
		t.Fatalf("item a: %v", it)
	}
	if it := byID[b]; it["deleted"] != true {
		t.Fatalf("item b should be a tombstone: %v", it)
	}

	// Nothing new since the last version.
	e.call("GET", "/sync?since="+jsonNum(pull.Version), "", 200, &pull)
	if len(pull.Items) != 0 {
		t.Fatalf("expected no changes, got %v", pull.Items)
	}
	e.call("GET", "/sync?since=-1", "", 400, nil)
}

func jsonNum(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestRecipesAndAddToList(t *testing.T) {
	e := newDataEnv(t)
	recipe := `{"title":"Pancakes","summary":"Fluffy","servings":4,"prepMinutes":10,"cookMinutes":15,"tags":["Breakfast","  breakfast ","sweet"],
		"ingredients":[{"quantity":2,"unit":"cups","item":"Flour"},{"quantity":1,"unit":"cup","item":"milk"},{"quantity":2,"item":"Eggs"},{"item":"Salt","rawText":"a pinch"}],
		"steps":[{"text":"Mix."},{"text":"Fry.","timerSeconds":120}]}`
	var r obj
	e.call("POST", "/recipes", recipe, 201, &r)
	rid := r["id"].(string)
	if tags := r["tags"].([]any); len(tags) != 2 {
		t.Fatalf("tags not deduped/lowercased: %v", tags)
	}
	e.call("POST", "/recipes", `{"title":" "}`, 400, nil)
	e.call("POST", "/recipes", `{"title":"x","servings":0}`, 400, nil)

	e.call("POST", "/recipes", `{"title":"Omelette","ingredients":[{"quantity":3,"item":"eggs"}],"tags":["breakfast"]}`, 201, nil)
	e.call("POST", "/recipes", `{"title":"Soup","tags":["dinner"]}`, 201, nil)

	var got []obj
	e.call("GET", "/recipes", "", 200, &got)
	if len(got) != 3 || got[0]["title"] != "Omelette" || got[2]["title"] != "Soup" {
		t.Fatalf("A-Z list: %v", got)
	}
	e.call("GET", "/recipes?q=flour", "", 200, &got) // ingredient match
	if len(got) != 1 || got[0]["title"] != "Pancakes" {
		t.Fatalf("ingredient search: %v", got)
	}
	e.call("GET", "/recipes?tag=breakfast", "", 200, &got)
	if len(got) != 2 {
		t.Fatalf("tag filter: %v", got)
	}
	e.call("GET", "/recipes?q=100%25", "", 200, &got)
	if len(got) != 0 {
		t.Fatalf("wildcard characters must match literally: %v", got)
	}

	e.call("PUT", "/recipes/"+rid+"/marks/favourite", "", 204, nil)
	e.call("GET", "/recipes?mark=favourite", "", 200, &got)
	if len(got) != 1 || got[0]["id"] != rid {
		t.Fatalf("mark filter: %v", got)
	}
	e.call("DELETE", "/recipes/"+rid+"/marks/favourite", "", 204, nil)
	e.call("GET", "/recipes?mark=favourite", "", 200, &got)
	if len(got) != 0 {
		t.Fatalf("unmark: %v", got)
	}

	var tags []string
	e.call("GET", "/tags", "", 200, &tags)
	if len(tags) != 3 {
		t.Fatalf("tags: %v", tags)
	}

	// Replace swaps the children.
	e.call("PUT", "/recipes/"+rid, `{"title":"Pancakes","servings":4,"ingredients":[{"quantity":2,"unit":"cups","item":"Flour"},{"quantity":1,"unit":"cup","item":"milk"},{"quantity":2,"item":"Eggs"},{"item":"Salt","rawText":"a pinch"}],"steps":[{"text":"Only."}]}`, 200, &r)
	if len(r["steps"].([]any)) != 1 || len(r["tags"].([]any)) != 0 {
		t.Fatalf("replace: %v", r)
	}

	// Add to a list, doubled, then add the omelette: eggs merge.
	var list obj
	e.call("POST", "/lists", `{"name":"Groceries","colour":"#000"}`, 201, &list)
	lid := list["id"].(string)
	var res struct{ Added, Merged []obj }
	e.call("POST", "/lists/"+lid+"/add-recipe", `{"recipeId":"`+rid+`","servings":8}`, 200, &res)
	names := map[string]string{}
	for _, it := range res.Added {
		names[it["name"].(string)] = it["quantityText"].(string)
		if it["sourceRecipeId"] != rid {
			t.Errorf("source recipe not recorded: %v", it)
		}
	}
	if names["Flour"] != "4 cups" || names["milk"] != "2 cup" || names["Eggs"] != "4" || names["Salt"] != "a pinch" || len(res.Merged) != 0 {
		t.Fatalf("scaled items: %v merged=%v", names, res.Merged)
	}
	var om []obj
	e.call("GET", "/recipes?q=omelette", "", 200, &om)
	e.call("POST", "/lists/"+lid+"/add-recipe", `{"recipeId":"`+om[0]["id"].(string)+`"}`, 200, &res)
	if len(res.Added) != 0 || len(res.Merged) != 1 || res.Merged[0]["quantityText"] != "7" {
		t.Fatalf("merge: added=%v merged=%v", res.Added, res.Merged)
	}
	var detail struct{ Items []obj }
	e.call("GET", "/lists/"+lid, "", 200, &detail)
	if len(detail.Items) != 4 {
		t.Fatalf("list should hold 4 items, has %d", len(detail.Items))
	}
	e.call("POST", "/lists/"+lid+"/add-recipe", `{"recipeId":"`+rid+`","servings":-1}`, 400, nil)
	e.call("POST", "/lists/"+lid+"/add-recipe", `{"recipeId":"00000000-0000-4000-8000-000000000000"}`, 404, nil)

	e.call("DELETE", "/recipes/"+rid, "", 204, nil)
	e.call("GET", "/recipes/"+rid, "", 404, nil)
	e.call("DELETE", "/recipes/"+rid, "", 404, nil)
}

func TestEventsStream(t *testing.T) {
	e := newDataEnv(t)
	srv := httptest.NewServer(e.h)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/events", nil)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated stream: %v %v", resp, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	req.AddCookie(e.cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}

	var list obj
	e.call("POST", "/lists", `{"name":"L","colour":"#000"}`, 201, &list)
	var item obj
	e.call("POST", "/lists/"+list["id"].(string)+"/items", `{"name":"Milk"}`, 201, &item)

	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatal("stream closed before an item event")
			}
			if strings.HasPrefix(l, "data: ") {
				var ev struct {
					ListID, ItemID string
					Version        int64
				}
				json.Unmarshal([]byte(strings.TrimPrefix(l, "data: ")), &ev)
				if ev.ItemID != item["id"] || ev.ListID != list["id"] || ev.Version != int64(item["version"].(float64)) {
					t.Fatalf("event %q does not match item %v", l, item)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("no event received")
		}
	}
}

func TestSetupStatus(t *testing.T) {
	e := newDataEnv(t)
	setup := func() (s struct {
		PendingEmail string
		AppConnected bool
		AiConnected  bool
	}) {
		var me struct {
			Setup *struct {
				PendingEmail string
				AppConnected bool
				AiConnected  bool
			}
		}
		e.call("GET", "/me", "", http.StatusOK, &me)
		if me.Setup == nil {
			t.Fatal("no setup on /me")
		}
		return *me.Setup
	}
	if s := setup(); s.PendingEmail != "mum@example.com" || s.AppConnected || s.AiConnected {
		t.Fatalf("fresh member: %+v", s)
	}

	// Any request from the iOS app ticks off the mobile step.
	req := httptest.NewRequest("GET", "/lists", nil)
	req.Header.Set("X-Impressipies-Client", "ios")
	req.AddCookie(e.cookie)
	e.h.ServeHTTP(httptest.NewRecorder(), req)
	if s := setup(); !s.AppConnected {
		t.Fatalf("after an iOS request: %+v", s)
	}
}
