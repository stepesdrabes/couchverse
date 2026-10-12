package couchtv

import (
	"slices"
	"testing"
)

func TestSoundParts(t *testing.T) {
	// clips every 10 s (250 frames), at the lengths of the breaks that outran the call
	// timeout on the Pi
	spec := func(n int) BreakSpec {
		s := BreakSpec{Template: "evening", Loudness: -16}
		for i := range n {
			s.Items = append(s.Items, BreakItem{At: frameSeconds(i * 250)})
		}
		return s
	}
	for _, tc := range []struct {
		name          string
		items, frames int
		wantFrames    []int // each part's frames
		wantItems     []int // each part's items
	}{
		{"short break", 2, 376, []int{376}, []int{2}},
		{"117 s, one part", 12, 2921, []int{2921}, []int{12}},
		{"333 s, three parts", 34, 8322, []int{3000, 3000, 2322}, []int{12, 12, 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := soundParts(spec(tc.items), tc.frames)
			var frames, items []int
			total := 0
			for _, p := range parts {
				frames = append(frames, p.frames)
				items = append(items, len(p.spec.Items))
				total += p.frames
				if p.spec.Template != "evening" || p.spec.Loudness != -16 {
					t.Errorf("a part lost the break's template or loudness: %+v", p.spec)
				}
				if p.spec.Items[0].At != 0 {
					t.Errorf("a part starts its first item at %g, not 0", p.spec.Items[0].At)
				}
			}
			if !slices.Equal(frames, tc.wantFrames) || !slices.Equal(items, tc.wantItems) {
				t.Errorf("parts of %v frames with %v items, want %v with %v", frames, items, tc.wantFrames, tc.wantItems)
			}
			if total != tc.frames {
				t.Errorf("the parts cover %d frames, the break is %d", total, tc.frames)
			}
		})
	}
	if parts := soundParts(BreakSpec{}, 0); parts != nil {
		t.Errorf("a break without items has parts %v", parts)
	}
}
