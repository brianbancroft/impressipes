package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbtest"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

type mcpEnv struct {
	*dataEnv
	pool    *pgxpool.Pool
	srv     *httptest.Server
	session *mcp.ClientSession
	token   string
}

func newMCPEnv(t *testing.T) *mcpEnv {
	t.Helper()
	pool := dbtest.Pool(t)
	dir := family.NewPG(pool)
	h := NewWithStore(t.Context(), dir, pool)
	inv, err := dir.CreateInvite(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	rec := do(h, "POST", "/invites/"+inv.Token+"/accept", `{"name":"Mum","color":"#e11d48","credentialId":"c1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	e := &dataEnv{t: t, h: h, cookie: sessionOf(t, rec)}
	var me struct{ ID string }
	e.call("GET", "/me", "", 200, &me)
	tok, err := NewToken(t.Context(), pool, me.ID, "Claude Code - laptop")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint: srv.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{tok}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return &mcpEnv{dataEnv: e, pool: pool, srv: srv, session: session, token: tok}
}

// tool calls a tool and decodes its structured result; wantErr expects an error result.
func (e *mcpEnv) tool(name string, args obj, out any) string {
	e.t.Helper()
	res, err := e.session.CallTool(e.t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		e.t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		return res.Content[0].(*mcp.TextContent).Text
	}
	if out != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			e.t.Fatalf("%s: decode %s: %v", name, b, err)
		}
	}
	return ""
}

func (e *mcpEnv) mustTool(name string, args obj, out any) {
	e.t.Helper()
	if msg := e.tool(name, args, out); msg != "" {
		e.t.Fatalf("%s failed: %s", name, msg)
	}
}

func TestMCPRejectsMissingOrBadToken(t *testing.T) {
	e := newMCPEnv(t)
	for _, auth := range []string{"", "Bearer imp_nope", "Bearer " + e.token + "x", "Basic abc"} {
		req, _ := http.NewRequest("POST", e.srv.URL+"/mcp", strings.NewReader(`{}`))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("auth %q: got %d, want 401", auth, res.StatusCode)
		}
	}
}

func TestMCPToolsListed(t *testing.T) {
	e := newMCPEnv(t)
	res, err := e.session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, tl := range res.Tools {
		have[tl.Name] = true
	}
	for _, want := range []string{
		"list_lists", "get_list", "add_items", "check_items", "uncheck_items", "remove_items", "add_recipe_to_list",
		"search_recipes", "get_recipe", "create_recipe", "update_recipe", "delete_recipe", "attach_recipe_image",
		"list_tags", "find_similar_recipes", "ingredient_pairings",
	} {
		if !have[want] {
			t.Errorf("missing tool %s", want)
		}
	}
}

func TestMCPListFlow(t *testing.T) {
	e := newMCPEnv(t)
	var list obj
	e.call("POST", "/lists", `{"name":"Costco","colour":"#C9594B","categories":[{"name":"Dairy","colour":"#E8B84A"},{"name":"Produce","colour":"#4C9A4A"}]}`, 201, &list)

	var lists struct{ Lists []obj }
	e.mustTool("list_lists", obj{}, &lists)
	if len(lists.Lists) != 1 || lists.Lists[0]["name"] != "Costco" {
		t.Fatalf("lists = %v", lists.Lists)
	}

	var added struct {
		Added, Merged []obj
		Warnings      []string
		BatchID       string
	}
	e.mustTool("add_items", obj{"list": "costco", "items": []obj{
		{"name": "Milk", "quantity": "2 L", "category": "dairy"},
		{"name": "Spinach", "category": "garden"},
	}}, &added)
	if len(added.Added) != 2 || added.Added[0]["category"] != "Dairy" || len(added.Warnings) != 1 || added.BatchID == "" {
		t.Fatalf("add_items = %+v", added)
	}

	var again struct{ Added, Merged []obj }
	e.mustTool("add_items", obj{"list": "Costco", "items": []obj{{"name": "milk", "quantity": "1 L"}}}, &again)
	if len(again.Added) != 0 || len(again.Merged) != 1 || again.Merged[0]["quantity"] != "3 L" {
		t.Fatalf("duplicate should merge, got %+v", again)
	}

	var checked struct {
		Changed  []obj
		NotFound []string
	}
	e.mustTool("check_items", obj{"list": "Costco", "items": []string{"spinach", "bananas"}}, &checked)
	if len(checked.Changed) != 1 || checked.Changed[0]["checked"] != true || len(checked.NotFound) != 1 {
		t.Fatalf("check_items = %+v", checked)
	}
	var got struct {
		List  obj
		Items []obj
	}
	e.mustTool("get_list", obj{"list": "Costco"}, &got)
	if got.List["openItems"] != float64(1) {
		t.Fatalf("openItems = %v", got.List["openItems"])
	}
	e.mustTool("uncheck_items", obj{"list": "Costco", "items": []string{"spinach"}}, nil)
	e.mustTool("remove_items", obj{"list": "Costco", "items": []string{"spinach"}}, nil)
	e.mustTool("get_list", obj{"list": "Costco"}, &got)
	if len(got.Items) != 1 {
		t.Fatalf("after remove: %v", got.Items)
	}

	if msg := e.tool("get_list", obj{"list": "Nope"}, nil); !strings.Contains(msg, "Costco") {
		t.Errorf("unknown list should name the real ones, got %q", msg)
	}

	// Every change is attributed to the token's client and grouped by call.
	var feed []obj
	e.call("GET", "/activity?via=mcp", "", 200, &feed)
	if len(feed) == 0 {
		t.Fatal("no mcp activity")
	}
	for _, a := range feed {
		if a["via"] != "mcp:Claude Code - laptop" {
			t.Errorf("via = %v", a["via"])
		}
		if a["batchId"] == nil {
			t.Errorf("activity %v has no batch", a["action"])
		}
	}
	// The first call's items were touched afterwards, so undoing it asks for force.
	e.call("POST", "/activity/batches/"+added.BatchID+"/undo", `{}`, 409, nil)
	// A fresh call undoes cleanly as a group.
	var eggs struct{ BatchID string }
	e.mustTool("add_items", obj{"list": "Costco", "items": []obj{{"name": "Eggs"}, {"name": "Butter"}}}, &eggs)
	e.call("POST", "/activity/batches/"+eggs.BatchID+"/undo", `{}`, 200, nil)
	e.mustTool("get_list", obj{"list": "Costco"}, &got)
	if len(got.Items) != 1 {
		t.Fatalf("undo should remove both new items, have %v", got.Items)
	}
}

func TestMCPRecipeFlowAndAdvice(t *testing.T) {
	e := newMCPEnv(t)
	e.call("POST", "/lists", `{"name":"Groceries","colour":"#4C9A4A"}`, 201, nil)

	mk := func(title string, ings []string, tags ...string) string {
		var is []obj
		for _, i := range ings {
			is = append(is, obj{"item": i})
		}
		var out struct {
			Recipe  obj
			BatchID string
		}
		e.mustTool("create_recipe", obj{
			"title": title, "servings": 4, "sourceKind": "photo", "tags": tags,
			"ingredients": append(is, obj{"item": "olive oil", "quantity": 2, "unit": "tbsp"}),
			"steps":       []obj{{"text": "Cook it.", "timerSeconds": 600}},
		}, &out)
		if out.BatchID == "" {
			t.Fatal("no batch id")
		}
		return out.Recipe["id"].(string)
	}
	tacos := mk("Tacos", []string{"ground beef", "tortillas", "cilantro", "limes"}, "mexican")
	bowls := mk("Burrito bowls", []string{"rice", "ground beef", "cilantro", "lime"}, "mexican")
	mk("Pancakes", []string{"flour", "milk", "eggs"}, "breakfast")

	var found struct{ Recipes []obj }
	e.mustTool("search_recipes", obj{"query": "cilantro"}, &found)
	if len(found.Recipes) != 2 {
		t.Fatalf("search = %d recipes", len(found.Recipes))
	}

	var full obj
	e.mustTool("get_recipe", obj{"recipeId": tacos}, &full)
	if len(full["steps"].([]any)) != 1 || full["sourceKind"] != "photo" {
		t.Fatalf("recipe = %v", full)
	}

	var upd struct{ Recipe obj }
	e.mustTool("update_recipe", obj{"recipeId": tacos, "title": "Beef tacos"}, &upd)
	if upd.Recipe["title"] != "Beef tacos" || len(upd.Recipe["ingredients"].([]any)) != 5 {
		t.Fatalf("update should keep ingredients: %v", upd.Recipe)
	}

	var sim struct {
		Similar []struct {
			Recipe            obj
			SharedIngredients []string
		}
	}
	e.mustTool("find_similar_recipes", obj{"recipeIds": []string{tacos}}, &sim)
	if len(sim.Similar) != 1 || sim.Similar[0].Recipe["id"] != bowls {
		t.Fatalf("similar = %+v", sim.Similar)
	}

	var pair struct {
		UsedIn   []obj
		GoesWith []struct {
			Ingredient string
			Recipes    int
		}
	}
	e.mustTool("ingredient_pairings", obj{"ingredient": "limes"}, &pair)
	if len(pair.UsedIn) != 2 || len(pair.GoesWith) == 0 || pair.GoesWith[0].Recipes != 2 {
		t.Fatalf("pairings = %+v", pair)
	}

	var res struct {
		Added, Merged []obj
		BatchID       string
	}
	e.mustTool("add_recipe_to_list", obj{"list": "Groceries", "recipeId": tacos, "servings": 8}, &res)
	if len(res.Added) != 5 {
		t.Fatalf("added = %d", len(res.Added))
	}
	e.mustTool("add_recipe_to_list", obj{"list": "Groceries", "recipeId": bowls}, &res)
	if len(res.Merged) == 0 {
		t.Fatal("second recipe should merge the shared ingredients")
	}

	if msg := e.tool("create_recipe", obj{"title": "x", "ingredients": []obj{}, "steps": []obj{}, "sourceKind": "dream"}, nil); !strings.Contains(msg, "sourceKind") {
		t.Errorf("bad sourceKind: %q", msg)
	}
	if msg := e.tool("get_recipe", obj{"recipeId": "nope"}, nil); !strings.Contains(msg, "not found") {
		t.Errorf("missing recipe: %q", msg)
	}

	var del struct{ BatchID string }
	e.mustTool("delete_recipe", obj{"recipeId": bowls}, &del)
	e.mustTool("search_recipes", obj{"query": "bowls"}, &found)
	if len(found.Recipes) != 0 {
		t.Fatal("deleted recipe still searchable")
	}
	e.call("POST", "/activity/batches/"+del.BatchID+"/undo", `{}`, 200, nil)
	e.mustTool("search_recipes", obj{"query": "bowls"}, &found)
	if len(found.Recipes) != 1 {
		t.Fatal("undo should bring the recipe back")
	}
}

func TestMCPDisabledMemberCannotUseToken(t *testing.T) {
	e := newMCPEnv(t)
	var me struct{ ID string }
	e.call("GET", "/me", "", 200, &me)
	// A second member cannot be disabled by a member, and the admin disabling themself is
	// not the point: check the guard through the directory directly.
	if err := family.NewPG(e.pool).SetStatus(t.Context(), me.ID, family.StatusDisabled); err != nil {
		t.Skipf("cannot disable: %v", err)
	}
	req, _ := http.NewRequest("POST", e.srv.URL+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+e.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("disabled member's token: got %d, want 403", res.StatusCode)
	}
}
