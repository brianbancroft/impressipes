package listsvc

import "testing"

func TestNormalize(t *testing.T) {
	if got := Normalize("  Olive   Oil "); got != "olive oil" {
		t.Fatalf("got %q", got)
	}
}

func TestParseQty(t *testing.T) {
	tests := []struct {
		in   string
		want Qty
	}{
		{"2 cups", Qty{2, "cups", true}},
		{"1/2 tsp", Qty{0.5, "tsp", true}},
		{"1 1/2 kg", Qty{1.5, "kg", true}},
		{"0.5l", Qty{0.5, "l", true}},
		{"3", Qty{3, "", true}},
		{"a pinch", Qty{}},
		{"", Qty{}},
	}
	for _, tt := range tests {
		if got := ParseQty(tt.in); got != tt.want {
			t.Errorf("ParseQty(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestMerge(t *testing.T) {
	tests := []struct{ a, b, want string }{
		{"2 cups", "1 cups", "3 cups"},
		{"2", "3", "5"},
		{"2 cups", "1 tbsp", "2 cups + 1 tbsp"},
		{"", "1 cup", "1 cup"},
		{"1 cup", "", "1 cup"},
		{"a pinch", "a pinch", "a pinch"},
	}
	for _, tt := range tests {
		if got := Merge(tt.a, tt.b); got != tt.want {
			t.Errorf("Merge(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestScaleAndFormat(t *testing.T) {
	if got := Format(Scale(1.5, 4, 6), "cups"); got != "2.25 cups" {
		t.Fatalf("got %q", got)
	}
	if got := Scale(2, 0, 6); got != 2 {
		t.Fatalf("unscaled got %v", got)
	}
	if got := Format(0.333333, "kg"); got != "0.33 kg" {
		t.Fatalf("got %q", got)
	}
}
