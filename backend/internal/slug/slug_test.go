package slug

import "testing"

func TestMake(t *testing.T) {
	year := 2008
	cases := []struct {
		name string
		year *int
		want string
	}{
		{"The Dark Knight", &year, "the-dark-knight-2008"},
		{"Amélie", nil, "amelie"},
		{"  WALL·E  ", nil, "wall-e"},
		{"1984", nil, "1984"},
		{"???", nil, "title"},
		{"Šíleně smutná princezna", nil, "silene-smutna-princezna"},
	}
	for _, c := range cases {
		if got := Make(c.name, c.year); got != c.want {
			t.Errorf("Make(%q, %v) = %q, want %q", c.name, c.year, got, c.want)
		}
	}
}
