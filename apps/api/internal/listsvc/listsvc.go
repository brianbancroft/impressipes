// Package listsvc holds the pure logic behind list items: name normalisation,
// quantity parsing and merging, and scaling a recipe's ingredients.
package listsvc

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Normalize is the key items are matched and remembered by: lower case, trimmed,
// inner whitespace collapsed.
func Normalize(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

// Qty is a quantity text split into an amount and the rest ("2", "cups of").
type Qty struct {
	Amount float64
	Unit   string
	OK     bool // false when the text has no leading number
}

// ParseQty reads "2 cups", "1/2 tsp", "1 1/2 kg", "0.5l" or "3". Anything else is not OK.
func ParseQty(text string) Qty {
	text = strings.TrimSpace(text)
	if text == "" {
		return Qty{}
	}
	i := 0
	for i < len(text) && (unicode.IsDigit(rune(text[i])) || text[i] == '.' || text[i] == '/' || text[i] == ' ') {
		i++
	}
	numPart, unit := strings.TrimSpace(text[:i]), strings.TrimSpace(text[i:])
	if numPart == "" {
		return Qty{}
	}
	total := 0.0
	for _, f := range strings.Fields(numPart) {
		v, ok := parseNumber(f)
		if !ok {
			return Qty{}
		}
		total += v
	}
	return Qty{Amount: total, Unit: unit, OK: true}
}

func parseNumber(f string) (float64, bool) {
	if n, d, ok := strings.Cut(f, "/"); ok {
		a, err1 := strconv.ParseFloat(n, 64)
		b, err2 := strconv.ParseFloat(d, 64)
		if err1 != nil || err2 != nil || b == 0 {
			return 0, false
		}
		return a / b, true
	}
	v, err := strconv.ParseFloat(f, 64)
	return v, err == nil
}

// FormatAmount renders 2, 1.5 or 0.33 without trailing zeros.
func FormatAmount(v float64) string {
	v = math.Round(v*100) / 100
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// Format renders an amount and unit as quantity text ("2 cups"). A zero amount is blank.
func Format(amount float64, unit string) string {
	if amount <= 0 {
		return strings.TrimSpace(unit)
	}
	return strings.TrimSpace(FormatAmount(amount) + " " + unit)
}

// Merge combines the quantity text already on an item with an addition. Amounts in
// the same unit are summed ("1 cup" + "2 cups" is not summed, "2 cups" + "1 cups" is
// only when the unit text is identical); otherwise the two are joined with " + ".
func Merge(existing, add string) string {
	existing, add = strings.TrimSpace(existing), strings.TrimSpace(add)
	switch {
	case add == "":
		return existing
	case existing == "":
		return add
	case strings.EqualFold(existing, add) && !ParseQty(existing).OK:
		return existing
	}
	a, b := ParseQty(existing), ParseQty(add)
	if a.OK && b.OK && strings.EqualFold(a.Unit, b.Unit) {
		return Format(a.Amount+b.Amount, a.Unit)
	}
	return existing + " + " + add
}

// Scale multiplies a recipe quantity from the recipe's servings to the wanted servings.
// Missing or non-positive servings leave the quantity as is.
func Scale(quantity, recipeServings, wantServings float64) float64 {
	if recipeServings <= 0 || wantServings <= 0 {
		return quantity
	}
	return quantity * wantServings / recipeServings
}
