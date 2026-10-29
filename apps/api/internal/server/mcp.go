package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
	"github.com/brianbancroft/impressipies/apps/api/internal/listsvc"
	"github.com/brianbancroft/impressipies/apps/api/internal/pairing"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

// mcpHandler serves the Model Context Protocol over Streamable HTTP. Every request
// must carry a personal token (Authorization: Bearer imp_...); the tools then act as
// that token's member and are audited as "mcp:<token name>". The server never calls
// a model: the client does the reading and reasoning and hands structured data in.
func (s *server) mcpHandler() http.Handler {
	srv := s.mcpServer()
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{Stateless: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, client, ok := s.bearer(r.Context(), r.Header.Get("Authorization"))
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+baseURL(r)+`/.well-known/oauth-protected-resource"`)
			writeError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		if m.Status == family.StatusDisabled {
			writeError(w, http.StatusForbidden, "disabled")
			return
		}
		ctx := context.WithValue(r.Context(), memberKey{}, m)
		ctx = context.WithValue(ctx, viaKey{}, "mcp:"+client)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// addTool registers a tool. Each call gets its own batch id, so everything one call
// changes shows as one group in the activity feed and undoes together.
func addTool[In, Out any](srv *mcp.Server, name, desc string, readOnly bool, fn func(ctx context.Context, in In) (Out, error)) {
	no := false
	mcp.AddTool(srv, &mcp.Tool{
		Name: name, Description: desc,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: &no, OpenWorldHint: &no},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		ctx = context.WithValue(ctx, batchKey{}, uuidx.New())
		out, err := fn(ctx, in)
		return nil, out, toolError(err)
	})
}

// toolError shows the model our own messages and hides anything unexpected.
func toolError(err error) error {
	var he *httpError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &he):
		return errors.New(he.msg)
	}
	log.Printf("mcp: %v", err)
	return errors.New("internal error")
}

func batchString(ctx context.Context) string { return uuidx.String(batchOf(ctx)) }

// ---- shapes the model reads and writes ----

type itemView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Quantity string `json:"quantity,omitempty"`
	Note     string `json:"note,omitempty"`
	Category string `json:"category,omitempty"`
	Checked  bool   `json:"checked"`
}

type listView struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Emoji      string   `json:"emoji,omitempty"`
	Categories []string `json:"categories"`
	OpenItems  int      `json:"openItems"`
}

type listsOut struct {
	Lists []listView `json:"lists"`
}

type listIn struct {
	List string `json:"list" jsonschema:"the list's name (e.g. 'Costco') or its id"`
}

type listOut struct {
	List       listView   `json:"list"`
	Items      []itemView `json:"items"`
	RecentNote string     `json:"note,omitempty"`
}

type itemArg struct {
	Name     string `json:"name" jsonschema:"the item as it would be written on a shopping list, e.g. 'milk' or 'PLA white'. No quantity in the name."`
	Quantity string `json:"quantity,omitempty" jsonschema:"amount with its unit as one string, e.g. '2 L' or '3'. Leave out to reuse what this list usually buys."`
	Note     string `json:"note,omitempty" jsonschema:"short remark such as a brand or a size"`
	Category string `json:"category,omitempty" jsonschema:"the name of one of this list's categories (see list_lists). Leave out and the list picks from what it remembers."`
}

type addItemsIn struct {
	List  string    `json:"list" jsonschema:"the list's name (e.g. 'Costco') or its id"`
	Items []itemArg `json:"items" jsonschema:"everything to add. An item already on the list unchecked gets the quantity appended instead of a duplicate."`
}

type addItemsOut struct {
	Added    []itemView `json:"added"`
	Merged   []itemView `json:"merged" jsonschema:"items that were already on the list; their quantity now includes this addition"`
	Warnings []string   `json:"warnings,omitempty"`
	BatchID  string     `json:"batchId" jsonschema:"undo everything this call did with POST /activity/batches/{batchId}/undo, or from the activity feed"`
}

type itemsRefIn struct {
	List  string   `json:"list" jsonschema:"the list's name or id"`
	Items []string `json:"items" jsonschema:"item names (as shown on the list) or item ids"`
}

type itemsRefOut struct {
	Changed  []itemView `json:"changed"`
	NotFound []string   `json:"notFound,omitempty" jsonschema:"references that matched no item that could be changed"`
	BatchID  string     `json:"batchId"`
}

type addRecipeIn struct {
	List     string   `json:"list" jsonschema:"the list's name or id"`
	RecipeID string   `json:"recipeId"`
	Servings *float64 `json:"servings,omitempty" jsonschema:"scale the ingredients to this many servings; defaults to the recipe's own"`
}

type addRecipeOut struct {
	Added   []api.Item `json:"added"`
	Merged  []api.Item `json:"merged" jsonschema:"already on the list; quantity now includes the recipe's"`
	BatchID string     `json:"batchId"`
}

type searchIn struct {
	Query      string `json:"query,omitempty" jsonschema:"matches title, summary, ingredient names and tags; leave out to list everything"`
	Tag        string `json:"tag,omitempty"`
	Favourites bool   `json:"favourites,omitempty" jsonschema:"only recipes the signed-in member has marked as a favourite"`
}

type searchOut struct {
	Recipes []api.RecipeSummary `json:"recipes"`
}

type recipeIDIn struct {
	RecipeID string `json:"recipeId"`
}

type ingredientArg struct {
	Item     string   `json:"item" jsonschema:"the ingredient alone, e.g. 'yellow onion'. No quantity and no prep words."`
	Quantity *float64 `json:"quantity,omitempty" jsonschema:"a number only. Turn fractions into decimals (1/2 is 0.5). Leave out for 'to taste'."`
	Unit     string   `json:"unit,omitempty" jsonschema:"e.g. 'cup', 'tbsp', 'g'. Leave out for counted things like eggs."`
	PrepNote string   `json:"prepNote,omitempty" jsonschema:"e.g. 'finely chopped'"`
	Section  string   `json:"section,omitempty" jsonschema:"a group heading from the recipe, e.g. 'For the sauce'"`
	RawText  string   `json:"rawText,omitempty" jsonschema:"the line as written, when the quantity could not be parsed"`
}

type stepArg struct {
	Text         string `json:"text" jsonschema:"one instruction, without its step number"`
	TimerSeconds *int   `json:"timerSeconds,omitempty" jsonschema:"set when the step names a wait (e.g. 'simmer 20 minutes' is 1200)"`
}

type recipeArgs struct {
	Title       string          `json:"title"`
	Summary     string          `json:"summary,omitempty" jsonschema:"one or two sentences; skip any life story"`
	Servings    *float64        `json:"servings,omitempty"`
	PrepMinutes *int            `json:"prepMinutes,omitempty"`
	CookMinutes *int            `json:"cookMinutes,omitempty"`
	Ingredients []ingredientArg `json:"ingredients" jsonschema:"every ingredient in order, one entry each"`
	Steps       []stepArg       `json:"steps" jsonschema:"every instruction in order"`
	Tags        []string        `json:"tags,omitempty" jsonschema:"short lower-case labels such as 'weeknight', 'vegetarian'. Reuse existing ones (list_tags) when they fit."`
	SourceURL   string          `json:"sourceUrl,omitempty" jsonschema:"the page it came from, when it came from one"`
	SourceKind  string          `json:"sourceKind,omitempty" jsonschema:"one of 'photo' (read from a picture), 'url' (read from a web page), 'manual' (typed in), 'ai' (written by the assistant). Defaults to 'ai'."`
	Notes       string          `json:"notes,omitempty" jsonschema:"anything useful that is not an ingredient or step"`
}

type createRecipeOut struct {
	Recipe  api.Recipe `json:"recipe"`
	BatchID string     `json:"batchId"`
}

type updateRecipeIn struct {
	RecipeID    string           `json:"recipeId"`
	Title       *string          `json:"title,omitempty"`
	Summary     *string          `json:"summary,omitempty"`
	Servings    *float64         `json:"servings,omitempty"`
	PrepMinutes *int             `json:"prepMinutes,omitempty"`
	CookMinutes *int             `json:"cookMinutes,omitempty"`
	Ingredients *[]ingredientArg `json:"ingredients,omitempty" jsonschema:"when given, replaces the whole ingredient list"`
	Steps       *[]stepArg       `json:"steps,omitempty" jsonschema:"when given, replaces all the steps"`
	Tags        *[]string        `json:"tags,omitempty" jsonschema:"when given, replaces all the tags"`
	SourceURL   *string          `json:"sourceUrl,omitempty"`
	Notes       *string          `json:"notes,omitempty"`
}

type deleteRecipeOut struct {
	Deleted string `json:"deleted"`
	BatchID string `json:"batchId" jsonschema:"deleting is a soft delete; undo it with this batch id"`
}

type attachImageIn struct {
	RecipeID    string `json:"recipeId"`
	ImageBase64 string `json:"imageBase64" jsonschema:"the image file, base64 encoded (JPEG, PNG, WebP or GIF, up to 10 MB)"`
}

type tagsOut struct {
	Tags []string `json:"tags"`
}

type similarIn struct {
	RecipeIDs []string `json:"recipeIds" jsonschema:"one or more recipes to find look-alikes of; with several, the ingredients are pooled"`
	Limit     int      `json:"limit,omitempty" jsonschema:"how many to return; default 8"`
}

type similarRecipe struct {
	Recipe            api.RecipeSummary `json:"recipe"`
	Score             float64           `json:"score" jsonschema:"0 to 1; ingredient overlap with a small bonus for shared tags"`
	SharedIngredients []string          `json:"sharedIngredients"`
	SharedTags        []string          `json:"sharedTags,omitempty"`
}

type similarOut struct {
	Similar []similarRecipe `json:"similar"`
	Note    string          `json:"note"`
}

type pairingsIn struct {
	Ingredient string `json:"ingredient" jsonschema:"e.g. 'lime' or 'chickpeas'. Matches parts of names, so 'tomato' finds 'canned tomatoes'."`
	Limit      int    `json:"limit,omitempty" jsonschema:"how many partner ingredients to return; default 15"`
}

type partnerView struct {
	Ingredient string   `json:"ingredient"`
	Recipes    int      `json:"recipes" jsonschema:"in how many of the recipes using the ingredient this also appears"`
	Examples   []string `json:"examples"`
}

type pairingsOut struct {
	Ingredient string              `json:"ingredient"`
	UsedIn     []api.RecipeSummary `json:"usedIn"`
	GoesWith   []partnerView       `json:"goesWith"`
	Note       string              `json:"note"`
}

// ---- the server ----

func (s *server) mcpServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "impressipies", Version: "1.0.0"}, &mcp.ServerOptions{
		Instructions: "Impressipes is one family's recipe box and shared shopping lists. " +
			"Read recipes from photos or web pages yourself and save them with create_recipe (structured ingredients and steps). " +
			"Use list names like 'Costco' wherever a list is asked for. Changes are audited and undoable. " +
			"For meal ideas, use search_recipes, find_similar_recipes and ingredient_pairings for what the family already cooks, and your own knowledge for the rest.",
	})

	addTool(srv, "list_lists", "List every shopping list with its categories and how many items are still unchecked.", true,
		func(ctx context.Context, _ struct{}) (listsOut, error) {
			if _, err := s.member(ctx); err != nil {
				return listsOut{}, err
			}
			rows, err := s.q.ListLists(ctx)
			if err != nil {
				return listsOut{}, err
			}
			out := listsOut{Lists: []listView{}}
			for _, l := range rows {
				v, _, err := s.listView(ctx, l)
				if err != nil {
					return out, err
				}
				out.Lists = append(out.Lists, v)
			}
			return out, nil
		})

	addTool(srv, "get_list", "Show a list and its items. Each item carries its category, quantity and whether it is checked off.", true,
		func(ctx context.Context, in listIn) (listOut, error) {
			l, err := s.resolveList(ctx, in.List)
			if err != nil {
				return listOut{}, err
			}
			v, items, err := s.listView(ctx, l)
			return listOut{List: v, Items: items}, err
		})

	addTool(srv, "add_items", "Add items to a shopping list. Category and quantity default to what the list remembers for that item, and an item already on the list gets the quantity appended instead of a duplicate. Add everything in one call.", false,
		func(ctx context.Context, in addItemsIn) (addItemsOut, error) { return s.mcpAddItems(ctx, in) })

	addTool(srv, "check_items", "Tick items off a list (got it).", false,
		func(ctx context.Context, in itemsRefIn) (itemsRefOut, error) {
			return s.mcpItemsAction(ctx, in, "check")
		})
	addTool(srv, "uncheck_items", "Put ticked items back on the list.", false,
		func(ctx context.Context, in itemsRefIn) (itemsRefOut, error) {
			return s.mcpItemsAction(ctx, in, "uncheck")
		})
	addTool(srv, "remove_items", "Remove items from a list. They are soft-deleted and can be restored from the activity feed.", false,
		func(ctx context.Context, in itemsRefIn) (itemsRefOut, error) {
			return s.mcpItemsAction(ctx, in, "remove")
		})

	addTool(srv, "add_recipe_to_list", "Put a recipe's ingredients on a shopping list, scaled to the servings wanted. Ingredients already on the list are merged, not duplicated.", false,
		func(ctx context.Context, in addRecipeIn) (addRecipeOut, error) {
			l, err := s.resolveList(ctx, in.List)
			if err != nil {
				return addRecipeOut{}, err
			}
			res, err := s.AddRecipeToList(ctx, api.AddRecipeToListRequestObject{
				ListId: uuidx.String(l.ID),
				Body:   &api.AddRecipeToListJSONRequestBody{RecipeId: in.RecipeID, Servings: in.Servings},
			})
			if err != nil {
				return addRecipeOut{}, err
			}
			r := api.AddRecipeResult(res.(api.AddRecipeToList200JSONResponse))
			return addRecipeOut{Added: r.Added, Merged: r.Merged, BatchID: batchString(ctx)}, nil
		})

	addTool(srv, "search_recipes", "Find recipes in the family's recipe box by title, ingredient or tag.", true,
		func(ctx context.Context, in searchIn) (searchOut, error) {
			p := api.ListRecipesParams{}
			if in.Query != "" {
				p.Q = &in.Query
			}
			if in.Tag != "" {
				p.Tag = &in.Tag
			}
			if in.Favourites {
				fav := api.RecipeMark("favourite")
				p.Mark = &fav
			}
			res, err := s.ListRecipes(ctx, api.ListRecipesRequestObject{Params: p})
			if err != nil {
				return searchOut{}, err
			}
			return searchOut{Recipes: res.(api.ListRecipes200JSONResponse)}, nil
		})

	addTool(srv, "get_recipe", "Get one recipe in full: ingredients, steps, tags and photos.", true,
		func(ctx context.Context, in recipeIDIn) (api.Recipe, error) {
			res, err := s.GetRecipe(ctx, api.GetRecipeRequestObject{RecipeId: in.RecipeID})
			if err != nil {
				return api.Recipe{}, err
			}
			return api.Recipe(res.(api.GetRecipe200JSONResponse)), nil
		})

	addTool(srv, "list_tags", "List the tags used on recipes, so new recipes can reuse them.", true,
		func(ctx context.Context, _ struct{}) (tagsOut, error) {
			res, err := s.ListTags(ctx, api.ListTagsRequestObject{})
			if err != nil {
				return tagsOut{}, err
			}
			return tagsOut{Tags: res.(api.ListTags200JSONResponse)}, nil
		})

	addTool(srv, "create_recipe", "Save a new recipe. This is how recipes arrive from a photo of a cookbook page, a web page you read, or your own writing: read the source yourself and pass clean structured data. Check search_recipes first so you do not save a duplicate.", false,
		func(ctx context.Context, in recipeArgs) (createRecipeOut, error) {
			body, err := recipeInput(in)
			if err != nil {
				return createRecipeOut{}, err
			}
			res, err := s.CreateRecipe(ctx, api.CreateRecipeRequestObject{Body: &body})
			if err != nil {
				return createRecipeOut{}, err
			}
			return createRecipeOut{Recipe: api.Recipe(res.(api.CreateRecipe201JSONResponse)), BatchID: batchString(ctx)}, nil
		})

	addTool(srv, "update_recipe", "Change parts of a recipe. Only the fields you pass change; a list field you pass (ingredients, steps, tags) replaces the whole list.", false,
		func(ctx context.Context, in updateRecipeIn) (createRecipeOut, error) {
			cur, err := s.GetRecipe(ctx, api.GetRecipeRequestObject{RecipeId: in.RecipeID})
			if err != nil {
				return createRecipeOut{}, err
			}
			body := recipeToInput(api.Recipe(cur.(api.GetRecipe200JSONResponse)))
			if in.Title != nil {
				body.Title = *in.Title
			}
			if in.Summary != nil {
				body.Summary = in.Summary
			}
			if in.Servings != nil {
				body.Servings = in.Servings
			}
			if in.PrepMinutes != nil {
				body.PrepMinutes = in.PrepMinutes
			}
			if in.CookMinutes != nil {
				body.CookMinutes = in.CookMinutes
			}
			if in.Ingredients != nil {
				ings := ingredientsOf(*in.Ingredients)
				body.Ingredients = &ings
			}
			if in.Steps != nil {
				steps := stepsOf(*in.Steps)
				body.Steps = &steps
			}
			if in.Tags != nil {
				body.Tags = in.Tags
			}
			if in.SourceURL != nil {
				body.SourceUrl = in.SourceURL
			}
			if in.Notes != nil {
				body.Notes = in.Notes
			}
			res, err := s.ReplaceRecipe(ctx, api.ReplaceRecipeRequestObject{RecipeId: in.RecipeID, Body: &body})
			if err != nil {
				return createRecipeOut{}, err
			}
			return createRecipeOut{Recipe: api.Recipe(res.(api.ReplaceRecipe200JSONResponse)), BatchID: batchString(ctx)}, nil
		})

	addTool(srv, "delete_recipe", "Delete a recipe. It is a soft delete: the recipe can be restored from the activity feed.", false,
		func(ctx context.Context, in recipeIDIn) (deleteRecipeOut, error) {
			if _, err := s.DeleteRecipe(ctx, api.DeleteRecipeRequestObject{RecipeId: in.RecipeID}); err != nil {
				return deleteRecipeOut{}, err
			}
			return deleteRecipeOut{Deleted: in.RecipeID, BatchID: batchString(ctx)}, nil
		})

	addTool(srv, "attach_recipe_image", "Add a thumbnail to a recipe: an image you generated of the finished dish, or the source web page's own picture of it. Never attach the photo of the recipe card or cookbook page you read the recipe from. Only works when you hold the image file's bytes (for example from a file tool or an image generator); a picture the user showed you in chat is not available to tools, so skip this then.", false,
		func(ctx context.Context, in attachImageIn) (api.Image, error) {
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(in.ImageBase64))
			if err != nil {
				return api.Image{}, fail(http.StatusBadRequest, "imageBase64 is not valid base64")
			}
			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)
			part, err := w.CreateFormFile("file", "image")
			if err != nil {
				return api.Image{}, err
			}
			part.Write(data)
			w.Close()
			res, err := s.UploadRecipeImage(ctx, api.UploadRecipeImageRequestObject{
				RecipeId: in.RecipeID, Body: multipart.NewReader(&buf, w.Boundary()),
			})
			if err != nil {
				return api.Image{}, err
			}
			return api.Image(res.(api.UploadRecipeImage201JSONResponse)), nil
		})

	addTool(srv, "find_similar_recipes", "Find recipes in the family's box that resemble the given ones, by shared ingredients and tags (pantry staples like salt and oil are ignored). Use it for 'what else could we make like A, B and C'. It only knows the family's recipes; add your own suggestions on top.", true,
		func(ctx context.Context, in similarIn) (similarOut, error) {
			if len(in.RecipeIDs) == 0 {
				return similarOut{}, fail(http.StatusBadRequest, "recipeIds is required")
			}
			corpus, sums, err := s.corpus(ctx)
			if err != nil {
				return similarOut{}, err
			}
			var targets []pairing.Recipe
			for _, id := range in.RecipeIDs {
				r, ok := corpus[id]
				if !ok {
					return similarOut{}, notFound("recipe " + id)
				}
				targets = append(targets, r)
			}
			all := make([]pairing.Recipe, 0, len(corpus))
			for _, r := range corpus {
				all = append(all, r)
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 8
			}
			out := similarOut{Similar: []similarRecipe{}, Note: "Counted from the family's own recipes only."}
			for _, m := range pairing.Similar(targets, all, limit) {
				out.Similar = append(out.Similar, similarRecipe{
					Recipe: sums[m.Recipe.ID], Score: round2(m.Score), SharedIngredients: m.Shared, SharedTags: m.SharedT,
				})
			}
			if len(out.Similar) == 0 {
				out.Note = "Nothing in the family's recipes shares ingredients or tags with these. Suggest from your own knowledge."
			}
			return out, nil
		})

	addTool(srv, "ingredient_pairings", "What goes with an ingredient, according to the family's own recipes: the recipes that use it and the ingredients that most often appear next to it. Use it for 'what do I do with the leftover X'. For pairings beyond the family's recipes, rely on your own knowledge.", true,
		func(ctx context.Context, in pairingsIn) (pairingsOut, error) {
			if strings.TrimSpace(in.Ingredient) == "" {
				return pairingsOut{}, fail(http.StatusBadRequest, "ingredient is required")
			}
			corpus, sums, err := s.corpus(ctx)
			if err != nil {
				return pairingsOut{}, err
			}
			all := make([]pairing.Recipe, 0, len(corpus))
			for _, r := range corpus {
				all = append(all, r)
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 15
			}
			res := pairing.Pairings(in.Ingredient, all, limit)
			out := pairingsOut{
				Ingredient: in.Ingredient, UsedIn: []api.RecipeSummary{}, GoesWith: []partnerView{},
				Note: "Counted from the family's own recipes only.",
			}
			for _, r := range res.Recipes {
				out.UsedIn = append(out.UsedIn, sums[r.ID])
			}
			for _, p := range res.Partners {
				out.GoesWith = append(out.GoesWith, partnerView{Ingredient: p.Ingredient, Recipes: p.Recipes, Examples: p.Examples})
			}
			if len(out.UsedIn) == 0 {
				out.Note = "No recipe in the family's box uses this. Suggest from your own knowledge."
			}
			return out, nil
		})

	srv.AddPrompt(&mcp.Prompt{
		Name:        "import-from-photo",
		Description: "Read a photographed recipe and save it to Impressipes",
	}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return promptText("Read the recipe in the attached photo carefully. " +
			"First call search_recipes with its title to make sure it is not already saved. " +
			"Then call create_recipe with sourceKind 'photo': split ingredients into item, quantity (a number), unit and prepNote, keep section headings, " +
			"write each instruction as its own step with a timerSeconds when it names a wait, and reuse existing tags (list_tags) where they fit. " +
			"Do not invent anything the page does not say; put doubts in notes. Do not attach my photo as the recipe's image: if you can generate images, attach a small generated picture of the finished dish with attach_recipe_image, otherwise skip the image. Finish by telling me what you saved."), nil
	})
	srv.AddPrompt(&mcp.Prompt{
		Name:        "plan-week-of-meals",
		Description: "Plan dinners from the family's recipes and put the groceries on a list",
		Arguments: []*mcp.PromptArgument{
			{Name: "dinners", Description: "how many dinners to plan", Required: false},
			{Name: "list", Description: "the list for the groceries, e.g. Groceries", Required: false},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		n, list := req.Params.Arguments["dinners"], req.Params.Arguments["list"]
		if n == "" {
			n = "5"
		}
		if list == "" {
			list = "the main grocery list"
		}
		return promptText("Plan " + n + " dinners from our saved recipes. Use search_recipes (favourites first) and find_similar_recipes to vary them, " +
			"and avoid repeating a main ingredient on consecutive nights. Show me the plan and wait for my yes before changing anything. " +
			"Then call add_recipe_to_list for each recipe on " + list + ", and tell me what was added and merged."), nil
	})
	return srv
}

func promptText(t string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: t}}}}
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

// ---- helpers ----

// resolveList finds a list by id or by name (case-insensitive).
func (s *server) resolveList(ctx context.Context, ref string) (dbgen.List, error) {
	if _, err := s.member(ctx); err != nil {
		return dbgen.List{}, err
	}
	ref = strings.TrimSpace(ref)
	if id, ok := uuidx.Parse(ref); ok {
		if l, err := s.q.GetList(ctx, id); err == nil {
			return l, nil
		}
	}
	rows, err := s.q.ListLists(ctx)
	if err != nil {
		return dbgen.List{}, err
	}
	var names []string
	for _, l := range rows {
		if strings.EqualFold(l.Name, ref) {
			return l, nil
		}
		names = append(names, l.Name)
	}
	return dbgen.List{}, fail(http.StatusNotFound, "no list named "+ref+"; the lists are: "+strings.Join(names, ", "))
}

func (s *server) listView(ctx context.Context, l dbgen.List) (listView, []itemView, error) {
	cats, err := s.q.ListCategories(ctx, l.ID)
	if err != nil {
		return listView{}, nil, err
	}
	items, err := s.q.ListItems(ctx, l.ID)
	if err != nil {
		return listView{}, nil, err
	}
	v := listView{ID: uuidx.String(l.ID), Name: l.Name, Emoji: l.Emoji, Categories: []string{}}
	catName := map[string]string{}
	for _, c := range cats {
		v.Categories = append(v.Categories, c.Name)
		catName[uuidx.String(c.ID)] = c.Name
	}
	views := make([]itemView, 0, len(items))
	for _, i := range items {
		if !i.CheckedAt.Valid {
			v.OpenItems++
		}
		views = append(views, itemOf(i, catName))
	}
	return v, views, nil
}

func itemOf(i dbgen.ListItem, catName map[string]string) itemView {
	return itemView{
		ID: uuidx.String(i.ID), Name: i.Name, Quantity: i.QuantityText, Note: i.Note,
		Category: catName[uuidx.String(i.CategoryID)], Checked: i.CheckedAt.Valid,
	}
}

func (s *server) categoryNames(ctx context.Context, q *dbgen.Queries, list pgtype.UUID) (map[string]string, error) {
	cats, err := q.ListCategories(ctx, list)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, c := range cats {
		m[uuidx.String(c.ID)] = c.Name
	}
	return m, nil
}

func (s *server) mcpAddItems(ctx context.Context, in addItemsIn) (addItemsOut, error) {
	l, err := s.resolveList(ctx, in.List)
	if err != nil {
		return addItemsOut{}, err
	}
	if len(in.Items) == 0 {
		return addItemsOut{}, fail(http.StatusBadRequest, "items is required")
	}
	m, err := s.member(ctx)
	if err != nil {
		return addItemsOut{}, err
	}
	user, _ := uuidx.Parse(m.ID)
	out := addItemsOut{Added: []itemView{}, Merged: []itemView{}}
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		names, err := s.categoryNames(ctx, q, l.ID)
		if err != nil {
			return err
		}
		byName := map[string]string{}
		for id, n := range names {
			byName[strings.ToLower(n)] = id
		}
		for _, it := range in.Items {
			name := strings.TrimSpace(it.Name)
			if name == "" {
				out.Warnings = append(out.Warnings, "skipped an item with no name")
				continue
			}
			var catID *string
			if c := strings.TrimSpace(it.Category); c != "" {
				if id, ok := byName[strings.ToLower(c)]; ok {
					catID = &id
				} else {
					out.Warnings = append(out.Warnings, "'"+name+"': no category '"+c+"' on this list, so it was left uncategorised or categorised from memory")
				}
			}
			open, err := q.FindOpenItemByName(ctx, dbgen.FindOpenItemByNameParams{ListID: l.ID, Name: listsvc.Normalize(name)})
			if err == nil {
				item, err := q.UpdateItem(ctx, dbgen.UpdateItemParams{
					ID: open.ID, QuantityText: pgtype.Text{String: listsvc.Merge(open.QuantityText, it.Quantity), Valid: true},
				})
				if err != nil {
					return err
				}
				if err := s.record(ctx, q, m, "item_merged", l.ID, pgtype.UUID{}, itemDetail(item, open)); err != nil {
					return err
				}
				out.Merged = append(out.Merged, itemOf(item, names))
				continue
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			item, err := addItem(ctx, q, l.ID, user, uuidx.New(), name, strings.TrimSpace(it.Quantity), strings.TrimSpace(it.Note), catID, pgtype.UUID{})
			if err != nil {
				return err
			}
			if err := s.record(ctx, q, m, "item_added", l.ID, pgtype.UUID{}, map[string]any{"item": item.Name, "itemId": uuidx.String(item.ID)}); err != nil {
				return err
			}
			out.Added = append(out.Added, itemOf(item, names))
		}
		return nil
	})
	out.BatchID = batchString(ctx)
	return out, err
}

// mcpItemsAction checks, unchecks or removes items named by name or id. Changes go
// through the same handlers as the apps, so they are audited and undoable alike.
func (s *server) mcpItemsAction(ctx context.Context, in itemsRefIn, action string) (itemsRefOut, error) {
	l, err := s.resolveList(ctx, in.List)
	if err != nil {
		return itemsRefOut{}, err
	}
	if len(in.Items) == 0 {
		return itemsRefOut{}, fail(http.StatusBadRequest, "items is required")
	}
	items, err := s.q.ListItems(ctx, l.ID)
	if err != nil {
		return itemsRefOut{}, err
	}
	names, err := s.categoryNames(ctx, s.q, l.ID)
	if err != nil {
		return itemsRefOut{}, err
	}
	out := itemsRefOut{Changed: []itemView{}, BatchID: batchString(ctx)}
	used := map[string]bool{}
	for _, ref := range in.Items {
		var hit *dbgen.ListItem
		for i := range items {
			it := &items[i]
			id := uuidx.String(it.ID)
			if used[id] {
				continue
			}
			if action == "check" && it.CheckedAt.Valid || action == "uncheck" && !it.CheckedAt.Valid {
				continue
			}
			if id == strings.TrimSpace(ref) || listsvc.Normalize(it.Name) == listsvc.Normalize(ref) {
				hit = it
				break
			}
		}
		if hit == nil {
			out.NotFound = append(out.NotFound, ref)
			continue
		}
		id := uuidx.String(hit.ID)
		used[id] = true
		view := itemOf(*hit, names)
		if action == "remove" {
			if _, err := s.DeleteItem(ctx, api.DeleteItemRequestObject{ItemId: id}); err != nil {
				return out, err
			}
		} else {
			checked := action == "check"
			if _, err := s.UpdateItem(ctx, api.UpdateItemRequestObject{ItemId: id, Body: &api.ItemPatch{Checked: &checked}}); err != nil {
				return out, err
			}
			view.Checked = checked
		}
		out.Changed = append(out.Changed, view)
	}
	return out, nil
}

var sourceKinds = map[string]bool{"photo": true, "url": true, "manual": true, "ai": true}

func ingredientsOf(in []ingredientArg) []api.Ingredient {
	out := make([]api.Ingredient, 0, len(in))
	for _, i := range in {
		out = append(out, api.Ingredient{
			Item: i.Item, Quantity: i.Quantity, Unit: nonEmpty(i.Unit), PrepNote: nonEmpty(i.PrepNote),
			Section: nonEmpty(i.Section), RawText: nonEmpty(i.RawText),
		})
	}
	return out
}

func stepsOf(in []stepArg) []api.Step {
	out := make([]api.Step, 0, len(in))
	for _, st := range in {
		out = append(out, api.Step{Text: st.Text, TimerSeconds: st.TimerSeconds})
	}
	return out
}

func recipeInput(in recipeArgs) (api.RecipeInput, error) {
	kind := in.SourceKind
	if kind == "" {
		kind = "ai"
	}
	if !sourceKinds[kind] {
		return api.RecipeInput{}, fail(http.StatusBadRequest, "sourceKind must be photo, url, manual or ai")
	}
	sk := api.RecipeInputSourceKind(kind)
	ings, steps, tags := ingredientsOf(in.Ingredients), stepsOf(in.Steps), in.Tags
	return api.RecipeInput{
		Title: in.Title, Summary: nonEmpty(in.Summary), Servings: in.Servings,
		PrepMinutes: in.PrepMinutes, CookMinutes: in.CookMinutes,
		Ingredients: &ings, Steps: &steps, Tags: &tags,
		SourceUrl: nonEmpty(in.SourceURL), SourceKind: &sk, Notes: nonEmpty(in.Notes),
	}, nil
}

func recipeToInput(r api.Recipe) api.RecipeInput {
	sk := api.RecipeInputSourceKind(r.SourceKind)
	ings, steps, tags := r.Ingredients, r.Steps, r.Tags
	return api.RecipeInput{
		Title: r.Title, Summary: &r.Summary, Servings: r.Servings, PrepMinutes: r.PrepMinutes, CookMinutes: r.CookMinutes,
		Ingredients: &ings, Steps: &steps, Tags: &tags, SourceUrl: r.SourceUrl, SourceKind: &sk, Notes: &r.Notes,
	}
}

// corpus loads every live recipe for the similarity and pairing tools.
func (s *server) corpus(ctx context.Context) (map[string]pairing.Recipe, map[string]api.RecipeSummary, error) {
	m, err := s.member(ctx)
	if err != nil {
		return nil, nil, err
	}
	user, _ := uuidx.Parse(m.ID)
	rows, err := s.q.SearchRecipes(ctx, dbgen.SearchRecipesParams{UserID: user})
	if err != nil {
		return nil, nil, err
	}
	ids := make([]pgtype.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	tags, marks, err := recipeLabels(ctx, s.q, user, ids)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.q.RecipeIngredientItems(ctx)
	if err != nil {
		return nil, nil, err
	}
	corpus, sums := map[string]pairing.Recipe{}, map[string]api.RecipeSummary{}
	for _, r := range rows {
		k := uuidx.String(r.ID)
		corpus[k] = pairing.Recipe{ID: k, Title: r.Title, Tags: tags[k]}
		sums[k] = toSummary(r, tags[k], marks[k], nil)
	}
	for _, it := range items {
		k := uuidx.String(it.RecipeID)
		if r, ok := corpus[k]; ok {
			r.Ingredients = append(r.Ingredients, it.Item)
			corpus[k] = r
		}
	}
	return corpus, sums, nil
}
