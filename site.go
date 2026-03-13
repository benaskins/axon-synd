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
<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
<title>{{.Config.Title}}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/style.css">
<link rel="alternate" type="application/rss+xml" title="{{.Config.Title}}" href="/feed.xml">
</head>
<body>
<header>
<div class="wordmark"><a href="/">{{.Config.Title}}</a></div>
<div class="tagline">by {{.Config.Author}}</div>
</header>
<hr class="divider">
<main>
<div class="lbl">Posts</div>
{{range .Posts}}
<article>
<time datetime="{{formatRFC3339 .CreatedAt}}">{{formatDate .CreatedAt}}</time>
{{if eq .Kind "long"}}<h2><a href="{{postURL .}}">{{.Title}}</a></h2>
<p class="abstract">{{.Abstract}}</p>
{{else if eq .Kind "image"}}<a href="{{postURL .}}"><p>{{nl2br .Body}}</p></a>
{{else}}<p>{{nl2br .Body}}</p>
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
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/style.css">
</head>
<body>
<header>
<div class="wordmark"><a href="/">{{.Config.Title}}</a></div>
</header>
<hr class="divider">
<main>
<article>
<time datetime="{{formatRFC3339 .Post.CreatedAt}}">{{formatDate .Post.CreatedAt}}</time>
{{if .Post.Title}}<h2>{{.Post.Title}}</h2>{{end}}
{{if eq .Post.Kind "long"}}<div class="body long-form">{{renderMarkdown .Post.Body}}</div>
{{else}}<div class="body">{{nl2br .Post.Body}}</div>
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
  --ink: #1C1917;
  --cream: #FAF9F7;
  --rule: #E7E5E4;
  --muted: #A8A29E;
  --mid: #78716C;
  --stone: #57534E;
}

@media (prefers-color-scheme: dark) {
  :root {
    --ink: #FAF9F7;
    --cream: #1C1917;
    --rule: #2E2A27;
    --muted: #A8A29E;
    --mid: #D6D3D1;
    --stone: #E7E5E4;
  }
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  background: var(--cream);
  color: var(--ink);
  font-family: 'DM Sans', 'Helvetica Neue', Helvetica, Arial, sans-serif;
  font-size: 16px;
  font-weight: 400;
  line-height: 1.7;
  transition: background 0.7s ease, color 0.7s ease;
}

header {
  max-width: 640px;
  margin: 0 auto;
  padding: 48px 24px 24px;
}

.wordmark {
  font-size: 24px;
  font-weight: 600;
  letter-spacing: -0.03em;
}

.wordmark a {
  color: var(--ink);
  text-decoration: none;
}

.tagline {
  font-size: 13px;
  font-weight: 400;
  color: var(--muted);
  letter-spacing: 0.06em;
  margin-top: 4px;
}

.divider {
  max-width: 640px;
  margin: 0 auto;
  border: none;
  border-top: 1px solid var(--rule);
}

.lbl {
  font-size: 9.5px;
  font-weight: 500;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted);
}

main {
  max-width: 640px;
  margin: 0 auto;
  padding: 40px 24px;
}

main > .lbl { margin-bottom: 32px; }

a { color: var(--ink); text-decoration: none; }
a:hover { border-bottom: 1px solid var(--ink); }

article {
  margin-bottom: 32px;
  padding-bottom: 32px;
  border-bottom: 1px solid var(--rule);
}

article:last-child { border-bottom: none; padding-bottom: 0; }

article time {
  display: block;
  font-size: 9.5px;
  font-weight: 500;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 8px;
}

article h2 {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.02em;
  margin-bottom: 8px;
}

article h2 a { border-bottom: 1px solid var(--rule); padding-bottom: 1px; transition: border-color 0.2s; }
article h2 a:hover { border-color: var(--ink); }

article p, article .abstract {
  font-size: 15px;
  line-height: 1.6;
  color: var(--mid);
}

.body { white-space: pre-wrap; color: var(--mid); }

.body.long-form {
  white-space: normal;
  color: var(--ink);
}

.long-form h1 {
  font-size: 24px;
  font-weight: 600;
  letter-spacing: -0.02em;
  margin: 48px 0 16px;
  color: var(--ink);
}

.long-form h2 {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.02em;
  margin: 40px 0 12px;
  color: var(--ink);
}

.long-form h3 {
  font-size: 16px;
  font-weight: 600;
  margin: 32px 0 8px;
  color: var(--ink);
}

.long-form p {
  font-size: 16px;
  line-height: 1.7;
  color: var(--mid);
  margin-bottom: 16px;
}

.long-form a {
  color: var(--ink);
  border-bottom: 1px solid var(--rule);
  padding-bottom: 1px;
  transition: border-color 0.2s;
}
.long-form a:hover { border-color: var(--ink); }

.long-form ul, .long-form ol {
  margin: 16px 0;
  padding-left: 24px;
  color: var(--mid);
}

.long-form li {
  font-size: 16px;
  line-height: 1.7;
  margin-bottom: 8px;
}

.long-form pre {
  background: var(--rule);
  border-radius: 4px;
  padding: 16px;
  margin: 24px 0;
  overflow-x: auto;
  font-family: 'IBM Plex Mono', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
  color: var(--ink);
}

.long-form code {
  font-family: 'IBM Plex Mono', 'Courier New', monospace;
  font-size: 14px;
}

.long-form p code {
  background: var(--rule);
  border-radius: 3px;
  padding: 2px 6px;
  font-size: 13px;
}

.long-form blockquote {
  border-left: 2px solid var(--rule);
  padding-left: 16px;
  margin: 24px 0;
  color: var(--stone);
  font-style: italic;
}

.long-form strong { color: var(--ink); font-weight: 600; }

footer {
  max-width: 640px;
  margin: 0 auto;
  padding: 24px 24px 48px;
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

footer a {
  color: var(--muted);
  border-bottom: 1px solid var(--rule);
  padding-bottom: 1px;
  transition: border-color 0.2s;
}
footer a:hover { border-color: var(--muted); }

@media (max-width: 480px) {
  header { padding: 32px 20px 20px; }
  main { padding: 32px 20px; }
  footer { padding: 20px 20px 32px; flex-direction: column; gap: 8px; }
  .wordmark { font-size: 20px; }
}

body { padding-bottom: 40px; }

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
  font-size: 11px;
  letter-spacing: 0.06em;
  background: var(--cream);
  border-top: 1px solid var(--rule);
  z-index: 1000;
}

.webring-name { color: var(--muted); }

.webring a {
  color: var(--muted);
  text-decoration: none;
  border-bottom: none;
  transition: color 0.2s;
}
.webring a:hover { color: var(--mid); border-bottom: none; }
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
