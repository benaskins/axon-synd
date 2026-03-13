package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	fact "github.com/benaskins/axon-fact"
	synd "github.com/benaskins/axon-synd"
	"github.com/spf13/cobra"
)

var postCmd = &cobra.Command{
	Use:   "post [text]",
	Short: "Create and publish a post",
	Long: `Create a post and publish it to the static site.

  synd post "short thought"
  synd post --long path/to/article.md
  synd post --image path/to/photo.png "caption text"
  synd post --long article.md --title "Custom Title" --abstract "Override"`,
	RunE: runPost,
	Args: cobra.ArbitraryArgs,
}

func init() {
	postCmd.Flags().Bool("long", false, "long-form post from a markdown file")
	postCmd.Flags().String("image", "", "path to image file")
	postCmd.Flags().String("title", "", "title for long-form posts")
	postCmd.Flags().String("abstract", "", "abstract for long-form posts")
	postCmd.Flags().StringSlice("tags", nil, "tags for the post")
	postCmd.Flags().Bool("no-publish", false, "create post without publishing to site")
	postCmd.Flags().Bool("syndicate", false, "syndicate to all configured platforms after publishing")
	rootCmd.AddCommand(postCmd)
}

func runPost(cmd *cobra.Command, args []string) error {
	dir := siteDir(cmd)
	if dir == "" {
		return fmt.Errorf("site directory required: set --site-dir or SYND_SITE_DIR")
	}

	isLong, _ := cmd.Flags().GetBool("long")
	imagePath, _ := cmd.Flags().GetString("image")
	title, _ := cmd.Flags().GetString("title")
	abstract, _ := cmd.Flags().GetString("abstract")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	noPublish, _ := cmd.Flags().GetBool("no-publish")

	store, projection := newStore()
	ctx := cmd.Context()

	var post *synd.Post
	var err error

	switch {
	case isLong:
		if len(args) < 1 {
			return fmt.Errorf("--long requires a markdown file path")
		}
		post, err = createLongPost(ctx, store, args[0], title, abstract, tags)
	case imagePath != "":
		body := strings.Join(args, " ")
		if body == "" {
			return fmt.Errorf("--image requires caption text")
		}
		post, err = store.Create(ctx, synd.Image, body,
			synd.WithImagePath(imagePath),
			synd.WithTags(tags...),
		)
	default:
		body := strings.Join(args, " ")
		if body == "" {
			return fmt.Errorf("post text required")
		}
		post, err = store.Create(ctx, synd.Short, body,
			synd.WithTags(tags...),
		)
	}

	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}

	fmt.Printf("created: %s (%s)\n", post.ID, post.Kind)

	if noPublish {
		return nil
	}

	url := fmt.Sprintf("%s/posts/%s", baseURL(cmd), post.ID)
	if err := store.Publish(ctx, post.ID, url); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	builder := synd.NewSiteBuilder(synd.SiteConfig{
		Title:   "Generative Plane",
		BaseURL: baseURL(cmd),
		Author:  "Benjamin Askins",
	})

	posts := projection.List()
	if err := builder.Build(posts, dir); err != nil {
		return fmt.Errorf("build site: %w", err)
	}

	changed, err := synd.GitPublish(dir, fmt.Sprintf("post: %s", truncateForCommit(post.Body, 50)))
	if err != nil {
		return fmt.Errorf("git publish: %w", err)
	}

	if changed {
		fmt.Printf("published: %s\n", url)
	} else {
		fmt.Println("published (no site changes)")
	}

	// Syndicate if requested
	doSyndicate, _ := cmd.Flags().GetBool("syndicate")
	if doSyndicate {
		// Refresh post after publish
		post = store.Get(post.ID)

		bskyConfig, err := blueskyConfigFromEnv()
		if err == nil {
			if pds := os.Getenv("SYND_BLUESKY_PDS"); pds != "" {
				bskyConfig.PDS = pds
			}
			if err := syndicateToBluesky(ctx, store, post, baseURL(cmd), bskyConfig); err != nil {
				return fmt.Errorf("bluesky: %w", err)
			}
		}

		mastoConfig, err := mastodonConfigFromEnv()
		if err == nil {
			if err := syndicateToMastodon(ctx, store, post, baseURL(cmd), mastoConfig); err != nil {
				return fmt.Errorf("mastodon: %w", err)
			}
		}
	}

	return nil
}

func createLongPost(ctx context.Context, store *synd.PostStore, path, title, abstract string, tags []string) (*synd.Post, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	body := string(data)

	if title == "" {
		title = extractTitle(body)
	}
	if abstract == "" {
		abstract = extractAbstract(body)
	}

	return store.Create(ctx, synd.Long, body,
		synd.WithTitle(title),
		synd.WithAbstract(abstract),
		synd.WithTags(tags...),
	)
}

func extractTitle(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func extractAbstract(md string) string {
	lines := strings.Split(md, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 280 {
			return line[:277] + "..."
		}
		return line
	}
	return ""
}

func truncateForCommit(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func newStore() (*synd.PostStore, *synd.PostProjection) {
	store := synd.NewPostStore(nil)
	projection := store.Projection()
	events := fact.NewMemoryStore(fact.WithProjector(projection))
	store.SetEventStore(events)
	return store, projection
}
