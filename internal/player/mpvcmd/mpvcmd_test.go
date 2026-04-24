package mpvcmd

import "testing"

func TestBuildQuoting(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "simple args",
			args: []string{"loadfile", "https://example.com/video.mp4"},
			want: `"loadfile" "https://example.com/video.mp4"`,
		},
		{
			name: "empty arg list",
			args: []string{},
			want: ``,
		},
		{
			name: "single arg",
			args: []string{"show-progress"},
			want: `"show-progress"`,
		},
		{
			name: "arg with embedded double quote",
			args: []string{"set", `title="hello"`},
			want: `"set" "title=\"hello\""`,
		},
		{
			name: "arg with backslash",
			args: []string{"load", `C:\Users\video.mkv`},
			want: `"load" "C:\\Users\\video.mkv"`,
		},
		{
			name: "numeric args",
			args: []string{"overlay-add", "0", "100", "200"},
			want: `"overlay-add" "0" "100" "200"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Build(tt.args...)
			if got != tt.want {
				t.Errorf("Build(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
