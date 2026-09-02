package palette

import "testing"

func TestNamesAndLookup(t *testing.T) {
	want := []string{"rgb", "fire", "ocean", "grayscale", "sunset", "neon", "viridis", "electric"}
	got := Names()
	if len(got) != len(want) {
		t.Fatalf("got %d palettes; want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("palette %d = %q; want %q", i, got[i], name)
		}
		if _, found := Lookup(name); !found {
			t.Errorf("palette %q not found", name)
		}
	}
}
