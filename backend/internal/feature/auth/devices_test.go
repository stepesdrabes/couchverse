package auth

import (
	"strings"
	"testing"
)

func TestUserCodesAreEightConsonants(t *testing.T) {
	for range 200 {
		code := newUserCode()
		if len(code) != userCodeLength {
			t.Fatalf("%q has %d characters", code, len(code))
		}
		for _, r := range code {
			if !strings.ContainsRune(userCodeAlphabet, r) {
				t.Fatalf("%q contains %q", code, r)
			}
		}
	}
}

func TestTypedUserCodesNormalize(t *testing.T) {
	for _, typed := range []string{"WDJB-MJHT", "wdjb mjht", " WdJbMjHt ", "wdjb-mjht\n"} {
		if got := normalizeUserCode(typed); got != "WDJBMJHT" {
			t.Errorf("normalizeUserCode(%q) = %q", typed, got)
		}
	}
	if got := formatUserCode("WDJBMJHT"); got != "WDJB-MJHT" {
		t.Errorf("formatUserCode = %q", got)
	}
}

func TestBrowserNames(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 15_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15":                      "Safari on macOS",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0":           "Edge on Windows",
		"Mozilla/5.0 (X11; Linux x86_64; rv:142.0) Gecko/20100101 Firefox/142.0":                                                                  "Firefox on Linux",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 27_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/27.0 Mobile/15E148 Safari/604.1": "Safari on iOS",
		"Mozilla/5.0 (Linux; Android 16) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36":                            "Chrome on Android",
		"Mozilla/5.0 (Linux; TitanOS/2.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36":                                  "Chrome on Titan OS",
		"curl/8.7.1": "Browser",
	}
	for ua, want := range cases {
		if got := browserName(ua); got != want {
			t.Errorf("browserName(%q) = %q, want %q", ua, got, want)
		}
	}
}
