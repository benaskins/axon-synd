package synd

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testConfig() SiteConfig {
	return SiteConfig{
		Title:   "Generative Plane",
		BaseURL: "https://generativeplane.com",
		Author:  "Benjamin Askins",
	}
}

func testPosts() []Post {
	return []Post{
		{
			ID:        "post-003",
			Kind:      Short,
			Body:      "latest thought",
			CreatedAt: time.Date(2026, 3, 13, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:        "post-002",
			Kind:      Long,
			Title:     "On Federation",
			Abstract:  "Why owning your content matters.",
			Body:      "# On Federation\n\nFull article content here.",
			CreatedAt: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:        "post-001",
			Kind:      Image,
			Body:      "studio session",
			ImagePath: "/images/studio.png",
			CreatedAt: time.Date(2026, 3, 11, 8, 0, 0, 0, time.UTC),
		},
	}
}

func TestSiteBuilder_Build(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())

	if err := builder.Build(testPosts(), dir); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Check index exists
	assertFileExists(t, filepath.Join(dir, "index.html"))
	assertFileExists(t, filepath.Join(dir, "style.css"))
	assertFileExists(t, filepath.Join(dir, "feed.xml"))

	// Check post pages
	assertFileExists(t, filepath.Join(dir, "posts", "post-001", "index.html"))
	assertFileExists(t, filepath.Join(dir, "posts", "post-002", "index.html"))
	assertFileExists(t, filepath.Join(dir, "posts", "post-003", "index.html"))
}

func TestSiteBuilder_IndexContent(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())
	builder.Build(testPosts(), dir)

	content := readFile(t, filepath.Join(dir, "index.html"))

	if !strings.Contains(content, "Generative Plane") {
		t.Error("index missing site title")
	}
	if !strings.Contains(content, "latest thought") {
		t.Error("index missing short post body")
	}
	if !strings.Contains(content, "On Federation") {
		t.Error("index missing long post title")
	}
	if !strings.Contains(content, "Why owning your content matters.") {
		t.Error("index missing long post abstract")
	}
	if !strings.Contains(content, "studio session") {
		t.Error("index missing image post caption")
	}
	if !strings.Contains(content, `<link rel="alternate"`) {
		t.Error("index missing RSS link")
	}
}

func TestSiteBuilder_PostPageContent(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())
	builder.Build(testPosts(), dir)

	// Long-form post page
	content := readFile(t, filepath.Join(dir, "posts", "post-002", "index.html"))

	if !strings.Contains(content, "On Federation") {
		t.Error("post page missing title")
	}
	if !strings.Contains(content, `og:title`) {
		t.Error("post page missing OG title")
	}
	if !strings.Contains(content, `og:description`) {
		t.Error("post page missing OG description")
	}
	if !strings.Contains(content, "2026-03-12") {
		t.Error("post page missing date")
	}
}

func TestSiteBuilder_LongPostRendersMarkdown(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())

	posts := []Post{{
		ID:        "md-post",
		Kind:      Long,
		Title:     "Test Article",
		Abstract:  "An article.",
		Body:      "# Heading\n\nA paragraph.\n\n```go\nfmt.Println(\"hello\")\n```\n\n- item one\n- item two",
		CreatedAt: time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC),
	}}
	builder.Build(posts, dir)

	content := readFile(t, filepath.Join(dir, "posts", "md-post", "index.html"))

	if !strings.Contains(content, "<h1>Heading</h1>") {
		t.Error("markdown heading not rendered")
	}
	if !strings.Contains(content, "<p>A paragraph.</p>") {
		t.Error("markdown paragraph not rendered")
	}
	if !strings.Contains(content, "<code") {
		t.Error("markdown code block not rendered")
	}
	if !strings.Contains(content, "<li>item one</li>") {
		t.Error("markdown list not rendered")
	}
}

func TestSiteBuilder_ShortPostNoMarkdown(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())

	posts := []Post{{
		ID:        "short-post",
		Kind:      Short,
		Body:      "just a thought\nwith a newline",
		CreatedAt: time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC),
	}}
	builder.Build(posts, dir)

	content := readFile(t, filepath.Join(dir, "posts", "short-post", "index.html"))

	if !strings.Contains(content, "just a thought<br>with a newline") {
		t.Error("short post should use nl2br, not markdown")
	}
}

func TestSiteBuilder_PostPageOGTags(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())
	builder.Build(testPosts(), dir)

	// Short post should use truncated body for OG
	content := readFile(t, filepath.Join(dir, "posts", "post-003", "index.html"))
	if !strings.Contains(content, `og:description" content="latest thought"`) {
		t.Error("short post OG description should use body")
	}

	// Long post should use abstract for OG
	content = readFile(t, filepath.Join(dir, "posts", "post-002", "index.html"))
	if !strings.Contains(content, `og:description" content="Why owning your content matters."`) {
		t.Error("long post OG description should use abstract")
	}
}

func TestSiteBuilder_Feed(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())
	builder.Build(testPosts(), dir)

	content := readFile(t, filepath.Join(dir, "feed.xml"))

	if !strings.Contains(content, `<?xml version="1.0"`) {
		t.Error("feed missing XML header")
	}
	if !strings.Contains(content, "<rss") {
		t.Error("feed missing rss element")
	}
	if !strings.Contains(content, "Generative Plane") {
		t.Error("feed missing channel title")
	}

	// Verify valid XML
	var rss struct {
		Channel struct {
			Items []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal([]byte(content), &rss); err != nil {
		t.Fatalf("feed is not valid XML: %v", err)
	}

	if len(rss.Channel.Items) != 3 {
		t.Errorf("feed has %d items, want 3", len(rss.Channel.Items))
	}

	if rss.Channel.Items[1].Title != "On Federation" {
		t.Errorf("item[1] title = %q, want %q", rss.Channel.Items[1].Title, "On Federation")
	}

	if !strings.Contains(rss.Channel.Items[0].Link, "post-003") {
		t.Errorf("item[0] link = %q, should contain post-003", rss.Channel.Items[0].Link)
	}
}

func TestSiteBuilder_EmptyPosts(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())

	if err := builder.Build(nil, dir); err != nil {
		t.Fatalf("Build with no posts: %v", err)
	}

	assertFileExists(t, filepath.Join(dir, "index.html"))
	assertFileExists(t, filepath.Join(dir, "feed.xml"))
}

func TestSiteBuilder_Style(t *testing.T) {
	dir := t.TempDir()
	builder := NewSiteBuilder(testConfig())
	builder.Build(testPosts(), dir)

	content := readFile(t, filepath.Join(dir, "style.css"))
	if !strings.Contains(content, "--bg: #111114") {
		t.Error("style missing background color")
	}
	if !strings.Contains(content, "IBM Plex Mono") {
		t.Error("style missing font")
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", path)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
