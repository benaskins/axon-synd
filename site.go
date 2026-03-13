package synd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
)

// SiteConfig holds settings for static site generation.
type SiteConfig struct {
	Title   string
	BaseURL string
	Author  string
}

// SiteBuilder generates a static site from posts.
type SiteBuilder struct {
	config    SiteConfig
	templates *template.Template
}

// NewSiteBuilder creates a builder with the given config.
func NewSiteBuilder(config SiteConfig) *SiteBuilder {
	md := goldmark.New()

	funcMap := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"formatRFC3339": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"nl2br": func(s string) template.HTML {
			return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(s), "\n", "<br>"))
		},
		"renderMarkdown": func(s string) template.HTML {
			var buf bytes.Buffer
			md.Convert([]byte(s), &buf)
			return template.HTML(buf.String())
		},
		"postURL": func(p Post) string {
			return fmt.Sprintf("/posts/%s", p.ID)
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(
		indexTemplate + postTemplate + feedTemplate + styleTemplate,
	))

	return &SiteBuilder{
		config:    config,
		templates: tmpl,
	}
}

// Build generates the full static site into outputDir.
func (b *SiteBuilder) Build(posts []Post, outputDir string) error {
	dirs := []string{
		outputDir,
		filepath.Join(outputDir, "posts"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	// Index page
	if err := b.renderFile(filepath.Join(outputDir, "index.html"), "index", map[string]any{
		"Config": b.config,
		"Posts":  posts,
	}); err != nil {
		return fmt.Errorf("render index: %w", err)
	}

	// Individual post pages
	for _, post := range posts {
		dir := filepath.Join(outputDir, "posts", post.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
		if err := b.renderFile(filepath.Join(dir, "index.html"), "post", map[string]any{
			"Config": b.config,
			"Post":   post,
		}); err != nil {
			return fmt.Errorf("render post %s: %w", post.ID, err)
		}
	}

	// RSS feed
	if err := b.buildFeed(posts, filepath.Join(outputDir, "feed.xml")); err != nil {
		return fmt.Errorf("render feed: %w", err)
	}

	// Style
	if err := b.renderFile(filepath.Join(outputDir, "style.css"), "style", nil); err != nil {
		return fmt.Errorf("render style: %w", err)
	}

	return nil
}

func (b *SiteBuilder) renderFile(path, tmplName string, data any) error {
	var buf bytes.Buffer
	if err := b.templates.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func (b *SiteBuilder) buildFeed(posts []Post, path string) error {
	type feedItem struct {
		XMLName     xml.Name `xml:"item"`
		Title       string   `xml:"title"`
		Link        string   `xml:"link"`
		Description string   `xml:"description"`
		PubDate     string   `xml:"pubDate"`
		GUID        string   `xml:"guid"`
	}

	type feedChannel struct {
		Title       string     `xml:"title"`
		Link        string     `xml:"link"`
		Description string     `xml:"description"`
		LastBuild   string     `xml:"lastBuildDate"`
		Items       []feedItem `xml:"item"`
	}

	type rss struct {
		XMLName xml.Name    `xml:"rss"`
		Version string      `xml:"version,attr"`
		Channel feedChannel `xml:"channel"`
	}

	items := make([]feedItem, 0, len(posts))
	for _, p := range posts {
		title := p.Title
		if title == "" {
			title = truncateText(p.Body, 80)
		}
		desc := p.Body
		if p.Kind == Long && p.Abstract != "" {
			desc = p.Abstract
		}
		pubDate := p.PublishedAt
		if pubDate.IsZero() {
			pubDate = p.CreatedAt
		}

		items = append(items, feedItem{
			Title:       title,
			Link:        fmt.Sprintf("%s/posts/%s", b.config.BaseURL, p.ID),
			Description: desc,
			PubDate:     pubDate.Format(time.RFC1123Z),
			GUID:        fmt.Sprintf("%s/posts/%s", b.config.BaseURL, p.ID),
		})
	}

	now := time.Now().UTC()
	feed := rss{
		Version: "2.0",
		Channel: feedChannel{
			Title:       b.config.Title,
			Link:        b.config.BaseURL,
			Description: fmt.Sprintf("Posts by %s", b.config.Author),
			LastBuild:   now.Format(time.RFC1123Z),
			Items:       items,
		},
	}

	output, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return err
	}

	header := []byte(xml.Header)
	return os.WriteFile(path, append(header, output...), 0o644)
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Templates

var indexTemplate = `{{define "index"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Config.Title}}</title>
<link rel="stylesheet" href="/style.css">
<link rel="alternate" type="application/rss+xml" title="{{.Config.Title}}" href="/feed.xml">
</head>
<body>
<header>
<h1><a href="/">{{.Config.Title}}</a></h1>
</header>
<main>
{{range .Posts}}
<article>
<time datetime="{{formatRFC3339 .CreatedAt}}">{{formatDate .CreatedAt}}</time>
{{if eq .Kind "long"}}<h2><a href="{{postURL .}}">{{.Title}}</a></h2>
<p>{{.Abstract}}</p>
{{else if eq .Kind "image"}}<a href="{{postURL .}}"><p>{{nl2br .Body}}</p></a>
{{else}}<p>{{nl2br .Body}}</p>
{{end}}
</article>
{{end}}
</main>
</body>
</html>{{end}}`

var postTemplate = `{{define "post"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{if .Post.Title}}{{.Post.Title}} — {{end}}{{.Config.Title}}</title>
<meta property="og:title" content="{{if .Post.Title}}{{.Post.Title}}{{else}}{{truncate .Post.Body 80}}{{end}}">
<meta property="og:description" content="{{if .Post.Abstract}}{{.Post.Abstract}}{{else}}{{truncate .Post.Body 200}}{{end}}">
<meta property="og:url" content="{{.Config.BaseURL}}/posts/{{.Post.ID}}">
<meta property="og:type" content="article">
<link rel="stylesheet" href="/style.css">
</head>
<body>
<header>
<h1><a href="/">{{.Config.Title}}</a></h1>
</header>
<main>
<article>
<time datetime="{{formatRFC3339 .Post.CreatedAt}}">{{formatDate .Post.CreatedAt}}</time>
{{if .Post.Title}}<h2>{{.Post.Title}}</h2>{{end}}
{{if eq .Post.Kind "long"}}<div class="body">{{renderMarkdown .Post.Body}}</div>
{{else}}<div class="body">{{nl2br .Post.Body}}</div>
{{end}}
</article>
</main>
</body>
</html>{{end}}`

var feedTemplate = `{{define "feed"}}{{end}}`

var styleTemplate = `{{define "style"}}:root {
  --bg: #111114;
  --fg: #e0e0e0;
  --dim: #888;
  --accent: #7ec8e3;
  --font: 'IBM Plex Mono', 'Courier New', monospace;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  background: var(--bg);
  color: var(--fg);
  font-family: var(--font);
  font-size: 16px;
  line-height: 1.6;
  max-width: 640px;
  margin: 0 auto;
  padding: 2rem 1rem;
}

a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }

header { margin-bottom: 2rem; }
header h1 { font-size: 1.2rem; font-weight: 400; }
header h1 a { color: var(--fg); }

article {
  margin-bottom: 2rem;
  padding-bottom: 2rem;
  border-bottom: 1px solid #222;
}

article:last-child { border-bottom: none; }

article time {
  display: block;
  color: var(--dim);
  font-size: 0.85rem;
  margin-bottom: 0.5rem;
}

article h2 {
  font-size: 1.1rem;
  font-weight: 500;
  margin-bottom: 0.5rem;
}

article p { margin-bottom: 0.5rem; }

.body { white-space: pre-wrap; }
{{end}}`
