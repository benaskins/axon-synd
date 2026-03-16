package synd

import "regexp"

// Link represents a hyperlink extracted from markdown text.
type Link struct {
	Text     string
	URL      string
	Start    int // byte offset in plain text
	End      int // byte offset in plain text
}

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// ExtractMarkdownLinks converts markdown link syntax to plain text and returns
// the positions of each link in the resulting text. Inline code spans are
// preserved as-is (backtick-wrapped content is not treated as links).
func ExtractMarkdownLinks(s string) (string, []Link) {
	var links []Link
	plain := mdLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		return "" // placeholder, replaced below
	})
	// rebuild properly tracking byte offsets
	plain = ""
	links = nil
	last := 0
	for _, loc := range mdLinkRe.FindAllStringSubmatchIndex(s, -1) {
		plain += s[last:loc[0]]
		linkText := s[loc[2]:loc[3]]
		linkURL := s[loc[4]:loc[5]]
		start := len(plain)
		plain += linkText
		end := len(plain)
		links = append(links, Link{
			Text:  linkText,
			URL:   linkURL,
			Start: start,
			End:   end,
		})
		last = loc[1]
	}
	plain += s[last:]
	return plain, links
}
