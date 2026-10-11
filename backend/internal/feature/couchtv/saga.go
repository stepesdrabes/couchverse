package couchtv

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// sagaGroup is a franchise: its films in order.
type sagaGroup struct {
	name   string
	movies []*Movie
}

// sagaGroups finds the franchises Saga Sunday works through (D30): movies of one TMDB
// collection, and among the rest movies whose names share a prefix ("Iron Man",
// "Iron Man 2"). Films run in release order, franchises by their first release.
func sagaGroups(movies []*Movie) []sagaGroup {
	collections := map[int]*sagaGroup{}
	var loose []*Movie
	for _, m := range movies {
		if m.CollectionID == 0 {
			loose = append(loose, m)
			continue
		}
		g := collections[m.CollectionID]
		if g == nil {
			g = &sagaGroup{name: m.CollectionName}
			collections[m.CollectionID] = g
		}
		g.movies = append(g.movies, m)
	}
	var groups []sagaGroup
	for _, g := range collections {
		groups = append(groups, *g)
	}

	// union-find over the loose movies by shared name prefix
	parent := make([]int, len(loose))
	for i := range parent {
		parent[i] = i
	}
	root := func(i int) int {
		for parent[i] != i {
			i = parent[i]
		}
		return i
	}
	words := make([][]string, len(loose))
	for i, m := range loose {
		words[i] = nameWords(m.Name)
	}
	for i := range loose {
		for j := i + 1; j < len(loose); j++ {
			if sameSaga(words[i], words[j]) {
				parent[root(j)] = root(i)
			}
		}
	}
	byRoot := map[int]*sagaGroup{}
	for i, m := range loose {
		r := root(i)
		g := byRoot[r]
		if g == nil {
			g = &sagaGroup{name: loose[r].Name}
			byRoot[r] = g
		}
		g.movies = append(g.movies, m)
	}
	for _, g := range byRoot {
		groups = append(groups, *g)
	}

	groups = slices.DeleteFunc(groups, func(g sagaGroup) bool { return len(g.movies) < 2 })
	for _, g := range groups {
		slices.SortStableFunc(g.movies, func(a, b *Movie) int {
			if c := a.Released.Compare(b.Released); c != 0 {
				return c
			}
			return byName(a.Name, a.ID, b.Name, b.ID)
		})
	}
	slices.SortFunc(groups, func(a, b sagaGroup) int {
		if c := a.movies[0].Released.Compare(b.movies[0].Released); c != 0 {
			return c
		}
		if c := cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name)); c != 0 {
			return c
		}
		return cmp.Compare(a.movies[0].ID, b.movies[0].ID)
	})
	return groups
}

// nameWords splits a name into lowercase words without punctuation or a leading
// English article.
func nameWords(name string) []string {
	ws := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) && r != '\''
	})
	if len(ws) > 1 && (ws[0] == "the" || ws[0] == "a" || ws[0] == "an") {
		ws = ws[1:]
	}
	return ws
}

// sameSaga reports whether two names read as one franchise: one is the other's
// leading words ("Alien", "Alien 3"), or they share at least two leading words making
// at least 8 letters ("Harry Potter a ..."), which keeps "The Dark Knight" and "The
// Dark Tower" apart.
func sameSaga(a, b []string) bool {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	if n == 0 {
		return false
	}
	if n == len(a) || n == len(b) {
		return len(strings.Join(a[:n], " ")) >= 4
	}
	return n >= 2 && len(strings.Join(a[:n], " ")) >= 8
}

// nextSagaFilm is the film after the latest saga airing in its franchise; once a
// franchise is done, the first film of the one shown longest ago (never shown first).
// allowed narrows the franchises; nil allows every one.
func (p *planner) nextSagaFilm(allowed func(sagaGroup) bool) *Movie {
	var groups []sagaGroup
	for _, g := range p.groups {
		if allowed == nil || allowed(g) {
			groups = append(groups, g)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	if p.sagaMark.id != "" {
		for _, g := range groups {
			i := slices.IndexFunc(g.movies, func(m *Movie) bool { return m.ID == p.sagaMark.id })
			if i >= 0 && i+1 < len(g.movies) {
				return g.movies[i+1]
			}
		}
	}
	best, bestAt := groups[0], p.sagaShown(groups[0])
	for _, g := range groups[1:] {
		if at := p.sagaShown(g); at.Before(bestAt) {
			best, bestAt = g, at
		}
	}
	return best.movies[0]
}

// sagaShown is the franchise's latest saga airing.
func (p *planner) sagaShown(g sagaGroup) time.Time {
	var at time.Time
	for _, m := range g.movies {
		if t := p.lastSaga[m.ID]; t.After(at) {
			at = t
		}
	}
	return at
}
