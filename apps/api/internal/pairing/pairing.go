// Package pairing answers "what is like this" and "what goes with that" from the
// family's own recipes. It is plain counting, with no model behind it: the AI
// client does the reasoning and uses these numbers as evidence.
package pairing

import (
	"sort"
	"strings"
)

// Recipe is the part of a recipe the maths needs.
type Recipe struct {
	ID          string
	Title       string
	Ingredients []string // item names as written, e.g. "yellow onions"
	Tags        []string
}

// staples are in nearly every recipe, so sharing one says nothing.
var staples = map[string]bool{
	"salt": true, "pepper": true, "black pepper": true, "water": true, "oil": true,
	"olive oil": true, "vegetable oil": true, "sugar": true, "flour": true, "butter": true,
	"kosher salt": true, "sea salt": true, "cooking spray": true,
}

// descriptors are dropped from the front of an ingredient so "fresh chopped parsley"
// and "parsley" count as the same thing.
var descriptors = map[string]bool{
	"fresh": true, "dried": true, "chopped": true, "minced": true, "diced": true, "sliced": true,
	"large": true, "small": true, "medium": true, "ripe": true, "frozen": true, "canned": true,
	"whole": true, "grated": true, "crushed": true, "finely": true, "roughly": true, "big": true,
}

// Key reduces an ingredient name to what it is matched on: lower case, descriptors
// dropped, and a simple singular.
func Key(item string) string {
	words := strings.Fields(strings.ToLower(item))
	out := words[:0]
	for _, w := range words {
		w = strings.Trim(w, ",.;:()")
		if w == "" || descriptors[w] {
			continue
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		return ""
	}
	out[len(out)-1] = singular(out[len(out)-1])
	return strings.Join(out, " ")
}

func singular(w string) string {
	switch {
	case w == "molasses" || w == "swiss":
		return w
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "oes") && len(w) > 4:
		return w[:len(w)-2]
	case strings.HasSuffix(w, "ss"), strings.HasSuffix(w, "us"):
		return w
	case strings.HasSuffix(w, "s") && len(w) > 3:
		return w[:len(w)-1]
	}
	return w
}

// Staple reports whether a key is a pantry staple.
func Staple(key string) bool { return staples[key] }

// keys is the set of meaningful ingredient keys in a recipe.
func keys(r Recipe) map[string]bool {
	out := map[string]bool{}
	for _, in := range r.Ingredients {
		if k := Key(in); k != "" && !staples[k] {
			out[k] = true
		}
	}
	return out
}

// Match is a recipe that resembles the targets.
type Match struct {
	Recipe  Recipe
	Score   float64  // 0 to 1; higher is closer
	Shared  []string // ingredient keys shared with the targets
	SharedT []string // tags shared with the targets
}

// Similar ranks every recipe outside targets by how much it overlaps with them: the
// Jaccard overlap of the ingredient sets (targets pooled), plus a nudge for shared
// tags. Recipes sharing nothing are left out. limit <= 0 returns all matches.
func Similar(targets, corpus []Recipe, limit int) []Match {
	pool, tags, skip := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, t := range targets {
		skip[t.ID] = true
		for k := range keys(t) {
			pool[k] = true
		}
		for _, tag := range t.Tags {
			tags[strings.ToLower(tag)] = true
		}
	}
	var out []Match
	for _, r := range corpus {
		if skip[r.ID] {
			continue
		}
		rk := keys(r)
		var shared, sharedTags []string
		for k := range rk {
			if pool[k] {
				shared = append(shared, k)
			}
		}
		for _, tag := range r.Tags {
			if tags[strings.ToLower(tag)] {
				sharedTags = append(sharedTags, strings.ToLower(tag))
			}
		}
		if len(shared) == 0 && len(sharedTags) == 0 {
			continue
		}
		union := len(pool) + len(rk) - len(shared)
		score := 0.0
		if union > 0 {
			score = float64(len(shared)) / float64(union)
		}
		score = score*0.85 + 0.15*tagOverlap(len(sharedTags), len(tags))
		sort.Strings(shared)
		sort.Strings(sharedTags)
		out = append(out, Match{Recipe: r, Score: score, Shared: shared, SharedT: sharedTags})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Recipe.Title < out[j].Recipe.Title
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func tagOverlap(shared, wanted int) float64 {
	if wanted == 0 {
		return 0
	}
	return float64(shared) / float64(wanted)
}

// Partner is an ingredient that turns up alongside the one asked about.
type Partner struct {
	Ingredient string
	Recipes    int      // how many of the recipes using the ingredient also use this
	Examples   []string // titles of up to three of them
}

// Result is what the family's recipes say about one ingredient.
type Result struct {
	Ingredient string
	Recipes    []Recipe // recipes that use it
	Partners   []Partner
}

// Pairings finds the recipes that use an ingredient (a substring match on the
// normalised name, so "tomato" finds "canned tomatoes") and counts what else those
// recipes use. Staples are left out unless asked about directly. limit caps partners.
func Pairings(ingredient string, corpus []Recipe, limit int) Result {
	q := Key(ingredient)
	res := Result{Ingredient: ingredient}
	if q == "" {
		return res
	}
	counts, examples := map[string]int{}, map[string][]string{}
	for _, r := range corpus {
		rk := keys(r)
		uses := false
		for k := range rk {
			if strings.Contains(k, q) {
				uses = true
				break
			}
		}
		if !uses && !staples[q] {
			continue
		}
		if !uses {
			// A staple is not in keys(); check the raw list.
			for _, in := range r.Ingredients {
				if strings.Contains(Key(in), q) {
					uses = true
					break
				}
			}
			if !uses {
				continue
			}
		}
		res.Recipes = append(res.Recipes, r)
		for k := range rk {
			if strings.Contains(k, q) {
				continue
			}
			counts[k]++
			if len(examples[k]) < 3 {
				examples[k] = append(examples[k], r.Title)
			}
		}
	}
	for k, n := range counts {
		res.Partners = append(res.Partners, Partner{Ingredient: k, Recipes: n, Examples: examples[k]})
	}
	sort.Slice(res.Partners, func(i, j int) bool {
		if res.Partners[i].Recipes != res.Partners[j].Recipes {
			return res.Partners[i].Recipes > res.Partners[j].Recipes
		}
		return res.Partners[i].Ingredient < res.Partners[j].Ingredient
	})
	if limit > 0 && len(res.Partners) > limit {
		res.Partners = res.Partners[:limit]
	}
	sort.Slice(res.Recipes, func(i, j int) bool { return res.Recipes[i].Title < res.Recipes[j].Title })
	return res
}
