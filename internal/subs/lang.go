package subs

import "strings"

// iso639_2to1 converts a 3-letter ISO 639-2 code to its 2-letter 639-1
// equivalent. Unknown codes pass through unchanged (lowercased).
// OpenSubtitles expects 2-letter codes, but the rest of the app uses
// 3-letter ones (matching mpv/Jellyfin), so this is the bridge.
func iso639_2to1(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if v, ok := map639_2to1[c]; ok {
		return v
	}
	return c
}

// iso639_1to2 converts a 2-letter ISO 639-1 code back to 3-letter form,
// so results from OpenSubtitles merge cleanly with mpv's 3-letter codes.
func iso639_1to2(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if v, ok := map639_1to2[c]; ok {
		return v
	}
	return c
}

var map639_2to1 = map[string]string{
	"eng": "en",
	"fre": "fr", "fra": "fr",
	"spa": "es",
	"ger": "de", "deu": "de",
	"ita": "it",
	"por": "pt",
	"rus": "ru",
	"jpn": "ja",
	"kor": "ko",
	"chi": "zh", "zho": "zh",
	"ara": "ar",
	"hin": "hi",
	"tur": "tr",
	"pol": "pl",
	"dut": "nl", "nld": "nl",
	"swe": "sv",
	"nor": "no",
	"dan": "da",
	"fin": "fi",
	"hun": "hu",
	"ces": "cs", "cze": "cs",
	"rum": "ro", "ron": "ro",
	"gre": "el", "ell": "el",
	"heb": "he",
	"tha": "th",
	"vie": "vi",
	"ind": "id",
	"may": "ms", "msa": "ms",
	"ukr": "uk",
	"bul": "bg",
	"hrv": "hr",
	"srp": "sr",
	"slv": "sl",
	"slk": "sk", "slo": "sk",
	"cat": "ca",
	"fil": "tl",
	"tam": "ta",
	"tel": "te",
	"ben": "bn",
}

var map639_1to2 = func() map[string]string {
	m := make(map[string]string, len(map639_2to1))
	for k, v := range map639_2to1 {
		// First occurrence wins — map eng→en, not fre→fr conflicting with fra→fr.
		if _, exists := m[v]; !exists {
			m[v] = k
		}
	}
	return m
}()
