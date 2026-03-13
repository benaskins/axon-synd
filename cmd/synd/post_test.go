package main

import "testing"

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{"h1 header", "# My Title\n\nBody text", "My Title"},
		{"no header", "Just some text", ""},
		{"h2 ignored", "## Subtitle\n\nText", ""},
		{"h1 after blank", "\n\n# Late Title\n\nBody", "Late Title"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.md)
			if got != tt.want {
				t.Errorf("extractTitle(%q) = %q, want %q", tt.md, got, tt.want)
			}
		})
	}
}

func TestExtractAbstract(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{"first paragraph", "# Title\n\nFirst paragraph here.", "First paragraph here."},
		{"skips headers", "# Title\n## Sub\n\nActual text.", "Actual text."},
		{"skips blanks", "\n\n\nSome text.", "Some text."},
		{"empty", "", ""},
		{"headers only", "# Title\n## Sub", ""},
		{"long truncates", "# Title\n\n" + longString(300), longString(277) + "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAbstract(tt.md)
			if got != tt.want {
				t.Errorf("extractAbstract() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateForCommit(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"short", 50, "short"},
		{"has\nnewlines\nin it", 50, "has newlines in it"},
		{"a long string that exceeds the limit", 10, "a long str..."},
	}

	for _, tt := range tests {
		got := truncateForCommit(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncateForCommit(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
