package synd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	Title       string
	BaseURL     string
	Author      string
	Description string
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
		"truncate": truncateText,
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
		"ghostWord": func(p Post) string {
			if p.Title != "" {
				words := strings.Fields(p.Title)
				if len(words) > 3 {
					return strings.ToUpper(strings.Join(words[:3], " "))
				}
				return strings.ToUpper(p.Title)
			}
			words := strings.Fields(p.Body)
			if len(words) > 3 {
				return strings.ToUpper(strings.Join(words[:3], " "))
			}
			return strings.ToUpper(p.Body)
		},
		"isOdd": func(i int) bool {
			return i%2 != 0
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(
		indexTemplate + postTemplate + feedTemplate + styleTemplate + webringTemplate,
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

	// Style — render first so we can hash it for cache busting
	if err := b.renderFile(filepath.Join(outputDir, "style.css"), "style", nil); err != nil {
		return fmt.Errorf("render style: %w", err)
	}
	styleHash, err := fileHash(filepath.Join(outputDir, "style.css"))
	if err != nil {
		return fmt.Errorf("hash style: %w", err)
	}

	// Index page
	if err := b.renderFile(filepath.Join(outputDir, "index.html"), "index", map[string]any{
		"Config":    b.config,
		"Posts":     posts,
		"StyleHash": styleHash,
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
			"Config":    b.config,
			"Post":      post,
			"StyleHash": styleHash,
		}); err != nil {
			return fmt.Errorf("render post %s: %w", post.ID, err)
		}
	}

	// RSS feed
	if err := b.buildFeed(posts, filepath.Join(outputDir, "feed.xml")); err != nil {
		return fmt.Errorf("render feed: %w", err)
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

func fileHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:12], nil
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
<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
<title>{{.Config.Title}}</title>
{{if .Config.Description}}<meta name="description" content="{{.Config.Description}}">
<meta property="og:type" content="website">
<meta property="og:title" content="{{.Config.Title}}">
<meta property="og:description" content="{{.Config.Description}}">
<meta property="og:image" content="{{.Config.BaseURL}}/og-image.png">
<meta property="og:url" content="{{.Config.BaseURL}}">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{{.Config.Title}}">
<meta name="twitter:description" content="{{.Config.Description}}">
<meta name="twitter:image" content="{{.Config.BaseURL}}/og-image.png">
<link rel="canonical" href="{{.Config.BaseURL}}">{{end}}
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Archivo+Black&family=Space+Mono:wght@400;700&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/style.css?v={{.StyleHash}}">
<link rel="alternate" type="application/rss+xml" title="{{.Config.Title}}" href="/feed.xml">
</head>
<body>
<header>
<div class="ghost-text">GENERATIVE PLANE</div>
<div class="header-meta">
<span class="corner-mark">GP//2026</span>
<span class="corner-mark">{{.Config.Author}}</span>
</div>
<div class="wordmark"><a href="/">{{.Config.Title}}</a></div>
<div class="accent-stripe"></div>
<div class="tagline">by {{.Config.Author}}</div>
</header>
<hr class="divider">
<main>
<div class="lbl">Posts</div>
{{range $i, $p := .Posts}}
<article>
<div class="article-ghost {{if isOdd $i}}article-ghost-left{{else}}article-ghost-right{{end}}">{{ghostWord $p}}</div>
<time datetime="{{formatRFC3339 $p.CreatedAt}}">{{formatDate $p.CreatedAt}}</time>
{{if eq $p.Kind "long"}}<h2><a href="{{postURL $p}}">{{$p.Title}}</a></h2>
<p class="abstract">{{$p.Abstract}}</p>
{{else if eq $p.Kind "image"}}<a href="{{postURL $p}}"><p>{{nl2br $p.Body}}</p></a>
{{else}}<div class="body short-form">{{renderMarkdown $p.Body}}</div>
{{end}}
</article>
{{end}}
</main>
<hr class="divider">
<footer>
<div class="lbl">{{.Config.Title}}</div>
<div class="lbl">&copy; {{.Config.Author}} 2026</div>
</footer>
{{template "webring"}}
</body>
</html>{{end}}`

var postTemplate = `{{define "post"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
<title>{{if .Post.Title}}{{.Post.Title}} — {{end}}{{.Config.Title}}</title>
<meta property="og:title" content="{{if .Post.Title}}{{.Post.Title}}{{else}}{{truncate .Post.Body 80}}{{end}}">
<meta property="og:description" content="{{if .Post.Abstract}}{{.Post.Abstract}}{{else}}{{truncate .Post.Body 200}}{{end}}">
<meta property="og:url" content="{{.Config.BaseURL}}/posts/{{.Post.ID}}">
<meta property="og:type" content="article">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Archivo+Black&family=Space+Mono:wght@400;700&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/style.css?v={{.StyleHash}}">
</head>
<body>
<header>
<div class="ghost-text">{{if .Post.Title}}{{.Post.Title}}{{else}}GP{{end}}</div>
<div class="header-meta">
<span class="corner-mark">GP//2026</span>
<span class="corner-mark">{{.Config.Author}}</span>
</div>
<div class="wordmark"><a href="/">{{.Config.Title}}</a></div>
<div class="accent-stripe"></div>
</header>
<hr class="divider">
<main>
<article>
<time datetime="{{formatRFC3339 .Post.CreatedAt}}">{{formatDate .Post.CreatedAt}}</time>
{{if and .Post.Title (ne .Post.Kind "long")}}<h2>{{.Post.Title}}</h2>{{end}}
{{if eq .Post.Kind "long"}}<div class="body long-form">{{renderMarkdown .Post.Body}}</div>
{{else}}<div class="body short-form">{{renderMarkdown .Post.Body}}</div>
{{end}}
</article>
</main>
<hr class="divider">
<footer>
<div class="lbl">{{.Config.Title}}</div>
<div class="lbl"><a href="/">all posts</a></div>
</footer>
{{template "webring"}}
</body>
</html>{{end}}`

var feedTemplate = `{{define "feed"}}{{end}}`

var styleTemplate = `{{define "style"}}:root {
  --bg: #0c0c0c;
  --fg: #e8e8e8;
  --dim: #444;
  --muted: #777;
  --accent: #e04020;
  --accent-dim: #802010;
  --rule: rgba(255, 255, 255, 0.08);
  --panel-bg: rgba(255, 255, 255, 0.03);
  --ghost: rgba(255, 255, 255, 0.02);
  color-scheme: dark light;
}

@media (prefers-color-scheme: light) {
  :root {
    --bg: #f0ebe4;
    --fg: #1a1410;
    --dim: #a08a78;
    --muted: #5c4d42;
    --accent: #c04020;
    --accent-dim: #d4704a;
    --rule: rgba(160, 138, 120, 0.3);
    --panel-bg: rgba(192, 72, 32, 0.04);
    --ghost: rgba(192, 72, 32, 0.04);
  }
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  background: var(--bg);
  color: var(--fg);
  font-family: 'Space Mono', monospace;
  font-weight: 400;
  font-size: 14px;
  line-height: 1.7;
  min-height: 100vh;
  overflow-x: hidden;
  padding-bottom: 40px;
}

/* --- Header / Hero --- */

header {
  position: relative;
  max-width: 640px;
  margin: 0 auto;
  padding: 48px 24px 24px;
  overflow: hidden;
}

.ghost-text {
  position: absolute;
  top: -0.1em;
  right: -0.02em;
  font-family: 'Archivo Black', sans-serif;
  font-size: 12rem;
  line-height: 0.85;
  color: var(--ghost);
  text-transform: uppercase;
  pointer-events: none;
  white-space: nowrap;
  letter-spacing: -0.04em;
}

.corner-mark {
  font-family: 'Space Mono', monospace;
  font-size: 9px;
  letter-spacing: 0.2em;
  color: var(--dim);
  text-transform: uppercase;
}

.header-meta {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 16px;
}

.wordmark {
  font-family: 'Archivo Black', sans-serif;
  font-size: 28px;
  text-transform: uppercase;
  letter-spacing: -0.02em;
  position: relative;
  z-index: 1;
}

.wordmark a {
  color: var(--fg);
  text-decoration: none;
}

.accent-stripe {
  height: 3px;
  background: var(--accent);
  margin: 12px 0;
  position: relative;
  z-index: 1;
}

.tagline {
  font-family: 'Space Mono', monospace;
  font-size: 10px;
  font-weight: 400;
  color: var(--dim);
  letter-spacing: 0.12em;
  position: relative;
  z-index: 1;
}

.divider {
  max-width: 640px;
  margin: 0 auto;
  border: none;
  border-top: 1px solid var(--dim);
}

/* --- Labels --- */

.lbl {
  font-family: 'Space Mono', monospace;
  font-size: 9px;
  font-weight: 400;
  letter-spacing: 0.25em;
  text-transform: uppercase;
  color: var(--dim);
}

/* --- Main --- */

main {
  max-width: 640px;
  margin: 0 auto;
  padding: 40px 24px;
}

main > .lbl { margin-bottom: 32px; }

a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }

/* --- Article list --- */

article {
  margin-bottom: 32px;
  padding-bottom: 32px;
  border-bottom: 1px solid var(--rule);
  position: relative;
}

article:last-child { border-bottom: none; padding-bottom: 0; }

.article-ghost {
  position: absolute;
  font-family: 'Archivo Black', sans-serif;
  font-size: 5rem;
  line-height: 0.85;
  color: var(--ghost);
  text-transform: uppercase;
  pointer-events: none;
  white-space: nowrap;
  letter-spacing: -0.04em;
  top: -0.2em;
  z-index: 0;
}
.article-ghost-right { right: -0.5em; }
.article-ghost-left { left: -0.5em; }

article time {
  display: block;
  font-family: 'Space Mono', monospace;
  font-size: 9px;
  font-weight: 400;
  letter-spacing: 0.25em;
  text-transform: uppercase;
  color: var(--dim);
  margin-bottom: 8px;
}

article h2 {
  font-size: 18px;
  font-weight: 400;
  letter-spacing: -0.01em;
  margin-bottom: 8px;
}

article h2 a {
  color: var(--fg);
  text-decoration: none;
  transition: color 0.2s;
}
article h2 a:hover { color: var(--accent); text-decoration: none; }

article p, article .abstract {
  font-size: 13px;
  line-height: 1.6;
  color: var(--muted);
}

.body { white-space: pre-wrap; color: var(--muted); }

/* --- Long-form posts --- */

.body.long-form {
  white-space: normal;
  color: var(--fg);
}

.long-form h1 {
  font-family: 'Archivo Black', sans-serif;
  font-size: 28px;
  font-weight: 400;
  text-transform: uppercase;
  letter-spacing: -0.02em;
  margin: 48px 0 16px;
  color: var(--fg);
}

.long-form h2 {
  font-size: 16px;
  font-weight: 400;
  letter-spacing: -0.01em;
  margin: 40px 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--rule);
  color: var(--fg);
}

.long-form h3 {
  font-size: 15px;
  font-weight: 400;
  margin: 32px 0 8px;
  color: var(--fg);
}

.long-form p {
  font-size: 14px;
  line-height: 1.75;
  color: var(--muted);
  margin-bottom: 16px;
}

.long-form a {
  color: var(--accent);
  transition: color 0.2s;
}
.long-form a:hover { color: var(--accent-dim); text-decoration: none; }

.long-form ul, .long-form ol {
  margin: 16px 0;
  padding-left: 24px;
  color: var(--muted);
}

.long-form li {
  font-size: 14px;
  line-height: 1.75;
  margin-bottom: 8px;
}

.long-form pre {
  background: var(--panel-bg);
  border: 1px solid var(--rule);
  padding: 16px;
  margin: 24px 0;
  overflow-x: auto;
  font-family: 'Space Mono', monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--fg);
}

.long-form code {
  font-family: 'Space Mono', monospace;
  font-size: 13px;
}

.long-form p code, .long-form li code {
  background: var(--panel-bg);
  border: 1px solid var(--rule);
  padding: 1px 5px;
  font-size: 12px;
}

.long-form blockquote {
  border-left: 3px solid var(--accent);
  padding-left: 16px;
  margin: 24px 0;
  color: var(--muted);
  font-style: italic;
}

.long-form strong { color: var(--fg); font-weight: 400; }

.long-form hr {
  border: none;
  height: 3px;
  background: var(--accent);
  margin: 48px 0;
}

/* --- Footer --- */

footer {
  max-width: 640px;
  margin: 0 auto;
  padding: 24px 24px 48px;
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  border-top: 1px solid var(--dim);
}

footer a {
  font-family: 'Space Mono', monospace;
  font-size: 10px;
  letter-spacing: 0.1em;
  color: var(--dim);
  text-decoration: none;
  text-transform: uppercase;
  border: 1px solid var(--dim);
  padding: 4px 10px;
  transition: all 0.2s;
}
footer a:hover { color: var(--fg); border-color: var(--fg); text-decoration: none; }

@media (max-width: 480px) {
  header { padding: 32px 20px 20px; }
  .ghost-text { font-size: 6rem; }
  .article-ghost { font-size: 3rem; }
  main { padding: 32px 20px; }
  footer { padding: 20px 20px 32px; flex-direction: column; gap: 8px; }
}

/* --- Webring --- */

.webring {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 8px 0 calc(8px + env(safe-area-inset-bottom, 0px));
  font-family: 'Space Mono', monospace;
  font-size: 10px;
  letter-spacing: 0.12em;
  background: var(--bg);
  border-top: 1px solid var(--dim);
  z-index: 1000;
}

.webring-name { color: var(--dim); }

.webring a {
  color: var(--dim);
  text-decoration: none;
  transition: color 0.2s;
}
.webring a:hover { color: var(--accent); text-decoration: none; }
{{end}}`

var webringTemplate = `{{define "webring"}}
<nav class="webring">
  <a class="webring-prev" href="#">&#8592;</a>
  <span class="webring-name">generativeplane</span>
  <a class="webring-next" href="#">&#8594;</a>
</nav>
<script>
(function() {
  var ring = [
    { name: 'benjaminaskins', url: 'https://benjaminaskins.com' },
    { name: 'genlevel', url: 'https://genlevel.com' },
    { name: 'generativeplane', url: 'https://generativeplane.com' },
    { name: 'isitconscious', url: 'https://isitconscious.xyz' }
  ];
  var host = location.hostname.replace('www.', '');
  var idx = ring.findIndex(function(s) { return host.indexOf(s.name) !== -1; });
  if (idx === -1) idx = 0;
  var prev = ring[(idx - 1 + ring.length) % ring.length];
  var next = ring[(idx + 1) % ring.length];
  var nav = document.querySelector('.webring');
  nav.querySelector('.webring-prev').href = prev.url;
  nav.querySelector('.webring-prev').textContent = '\u2190 ' + prev.name;
  nav.querySelector('.webring-next').href = next.url;
  nav.querySelector('.webring-next').textContent = next.name + ' \u2192';
  nav.querySelector('.webring-name').textContent = ring[idx].name;
})();
</script>
{{end}}`
