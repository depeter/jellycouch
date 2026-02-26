package ui

import (
	"sort"
	"strings"
)

// langNames maps ISO 639-2/B language codes to human-readable names.
var langNames = map[string]string{
	"eng": "English",
	"fre": "French",
	"fra": "French",
	"spa": "Spanish",
	"ger": "German",
	"deu": "German",
	"ita": "Italian",
	"por": "Portuguese",
	"rus": "Russian",
	"jpn": "Japanese",
	"kor": "Korean",
	"chi": "Chinese",
	"zho": "Chinese",
	"ara": "Arabic",
	"hin": "Hindi",
	"tur": "Turkish",
	"pol": "Polish",
	"dut": "Dutch",
	"nld": "Dutch",
	"swe": "Swedish",
	"nor": "Norwegian",
	"dan": "Danish",
	"fin": "Finnish",
	"hun": "Hungarian",
	"ces": "Czech",
	"cze": "Czech",
	"rum": "Romanian",
	"ron": "Romanian",
	"gre": "Greek",
	"ell": "Greek",
	"heb": "Hebrew",
	"tha": "Thai",
	"vie": "Vietnamese",
	"ind": "Indonesian",
	"may": "Malay",
	"msa": "Malay",
	"ukr": "Ukrainian",
	"bul": "Bulgarian",
	"hrv": "Croatian",
	"srp": "Serbian",
	"slv": "Slovenian",
	"slk": "Slovak",
	"slo": "Slovak",
	"cat": "Catalan",
	"fil": "Filipino",
	"tam": "Tamil",
	"tel": "Telugu",
	"ben": "Bengali",
	"und": "Unknown",
}

func langDisplayName(code string) string {
	if name, ok := langNames[code]; ok {
		return name
	}
	return code
}

// formatLangDisplay converts a comma-separated code string like "eng,jpn"
// to a display string like "English, Japanese".
func formatLangDisplay(csv string) string {
	if csv == "" {
		return ""
	}
	codes := strings.Split(csv, ",")
	names := make([]string, 0, len(codes))
	for _, c := range codes {
		c = strings.TrimSpace(c)
		if c != "" {
			names = append(names, langDisplayName(c))
		}
	}
	return strings.Join(names, ", ")
}

// langEntry pairs a code with its display name for the editor.
type langEntry struct {
	Code string
	Name string
}

// allLangEntries returns a deduplicated, alphabetically-sorted list of
// language entries using the preferred code for each language.
func allLangEntries() []langEntry {
	// Preferred codes (avoid duplicates like fra/fre, deu/ger, etc.)
	preferred := []string{
		"eng", "fre", "spa", "ger", "ita", "por", "rus", "jpn", "kor",
		"zho", "ara", "hin", "tur", "pol", "nld", "swe", "nor", "dan",
		"fin", "hun", "cze", "ron", "ell", "heb", "tha", "vie", "ind",
		"msa", "ukr", "bul", "hrv", "srp", "slv", "slo", "cat", "fil",
		"tam", "tel", "ben",
	}
	entries := make([]langEntry, len(preferred))
	for i, code := range preferred {
		entries[i] = langEntry{Code: code, Name: langNames[code]}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}
