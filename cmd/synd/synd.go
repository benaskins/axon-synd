package main

import (
	"context"
	"fmt"
	"os"

	synd "github.com/benaskins/axon-synd"
	"github.com/spf13/cobra"
)

var syndCmd = &cobra.Command{
	Use:   "synd [post-id]",
	Short: "Syndicate a post to social platforms",
	Long: `Syndicate a post to configured social platforms.

  synd synd <post-id>                    # all platforms
  synd synd <post-id> --platform bluesky # one platform`,
	RunE: runSynd,
	Args: cobra.ExactArgs(1),
}

func init() {
	syndCmd.Flags().String("platform", "", "syndicate to a specific platform only")
	rootCmd.AddCommand(syndCmd)
}

func runSynd(cmd *cobra.Command, args []string) error {
	postID := args[0]
	platform, _ := cmd.Flags().GetString("platform")

	store, _ := newStore()
	post := store.Get(postID)
	if post == nil {
		return fmt.Errorf("post %s not found", postID)
	}

	ctx := cmd.Context()

	if platform == "" || platform == "bluesky" {
		config, err := blueskyConfigFromEnv()
		if err != nil {
			return err
		}
		if err := syndicateToBluesky(ctx, store, post, baseURL(cmd), config); err != nil {
			return fmt.Errorf("bluesky: %w", err)
		}
	}

	return nil
}

func blueskyConfigFromEnv() (synd.BlueskyConfig, error) {
	handle := os.Getenv("SYND_BLUESKY_HANDLE")
	password := os.Getenv("SYND_BLUESKY_PASSWORD")
	if handle == "" || password == "" {
		return synd.BlueskyConfig{}, fmt.Errorf("SYND_BLUESKY_HANDLE and SYND_BLUESKY_PASSWORD must be set")
	}
	return synd.BlueskyConfig{
		Handle:   handle,
		Password: password,
	}, nil
}

// syndicateToBluesky posts to Bluesky and records the syndication event.
// Extracted from the command handler so it's testable and reusable from runPost.
func syndicateToBluesky(ctx context.Context, store *synd.PostStore, post *synd.Post, siteBaseURL string, config synd.BlueskyConfig) error {
	if post.ImportedFrom == string(synd.Bluesky) {
		return nil
	}

	client := synd.NewBlueskyClient(config)

	if err := client.Authenticate(ctx); err != nil {
		return err
	}

	var uri, cid string
	var err error

	switch post.Kind {
	case synd.Long:
		text := post.Abstract
		if text == "" {
			text = post.Title
		}
		url := fmt.Sprintf("%s/posts/%s", siteBaseURL, post.ID)
		uri, cid, err = client.PostWithLink(ctx, text, url, url)

	case synd.Image:
		if post.ImagePath != "" {
			uri, cid, err = client.PostWithImage(ctx, post.Body, post.ImagePath, post.Body)
		} else {
			uri, cid, err = client.Post(ctx, post.Body)
		}

	default:
		if len([]rune(post.Body)) <= 300 {
			uri, cid, err = client.Post(ctx, post.Body)
		} else {
			url := fmt.Sprintf("%s/posts/%s", siteBaseURL, post.ID)
			truncated := string([]rune(post.Body)[:250]) + "..."
			uri, cid, err = client.PostWithLink(ctx, truncated, url, url)
		}
	}

	if err != nil {
		return err
	}

	_ = cid
	remoteURL := synd.BlueskyPostURL(config.Handle, uri)

	if err := store.Syndicate(ctx, post.ID, synd.Bluesky, uri, remoteURL); err != nil {
		return fmt.Errorf("record syndication: %w", err)
	}

	fmt.Printf("bluesky: %s\n", remoteURL)
	return nil
}
