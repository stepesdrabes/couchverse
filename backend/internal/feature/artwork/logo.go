package artwork

// PickLogo chooses which of an owner's logos to show: the first of langs that
// has one (empty entries are skipped), else the logo not tied to a language,
// else any. Nil when there is no logo at all.
func PickLogo(items []Artwork, langs ...string) *Artwork {
	var neutral, other *Artwork
	byLang := map[string]*Artwork{}
	for i := range items {
		a := &items[i]
		switch {
		case a.Kind != "logo":
		case a.Lang == nil:
			neutral = a
		default:
			byLang[*a.Lang] = a
			if other == nil || *a.Lang < *other.Lang {
				other = a
			}
		}
	}
	for _, lang := range langs {
		if a := byLang[lang]; lang != "" && a != nil {
			return a
		}
	}
	if neutral != nil {
		return neutral
	}
	return other
}
