// Package mpvcmd builds quoted command strings for mpv_command_string.
// Split into its own package so the quoting logic can be tested without
// pulling in go-mpv's cgo init (which requires libmpv at load time).
package mpvcmd

import "strings"

// Build returns a properly quoted argument list for mpv_command_string.
// This avoids go-mpv's Command() which has a missing NULL terminator in
// its nocgo char** array on Windows.
func Build(args ...string) string {
	var b strings.Builder
	for i, arg := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('"')
		for _, c := range arg {
			if c == '"' || c == '\\' {
				b.WriteByte('\\')
			}
			b.WriteRune(c)
		}
		b.WriteByte('"')
	}
	return b.String()
}
