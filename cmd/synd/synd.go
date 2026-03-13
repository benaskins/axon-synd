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
		if err := syndicateBluesky(ctx, store, post, cmd); err != nil {
			return fmt.Errorf("bluesky: %w", err)
		}
	}

	return nil
}

func syndicateBluesky(ctx context.Context, store *synd.PostStore, post *synd.Post, cmd *cobra.Command) error {
	handle := os.Getenv("SYND_BLUESKY_HANDLE")
	password := os.Getenv("SYND_BLUESKY_PASSWORD")
	if handle == "" || password == "" {
		return fmt.Errorf("SYND_BLUESKY_HANDLE and SYND_BLUESKY_PASSWORD must be set")
	}

	client := synd.NewBlueskyClient(synd.BlueskyConfig{
		Handle:   handle,
		Password: password,
	})

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
		url := fmt.Sprintf("%s/posts/%s", baseURL(cmd), post.ID)
		uri, cid, err = client.PostWithLink(ctx, text, url, url)

	case synd.Image:
		if post.ImagePath != "" {
			uri, cid, err = client.PostWithImage(ctx, post.Body, post.ImagePath, post.Body)
		} else {
			uri, cid, err = client.Post(ctx, post.Body)
		}

	default:
		// Short posts: if under 300 graphemes, post directly. Otherwise treat like long-form.
		if len([]rune(post.Body)) <= 300 {
			uri, cid, err = client.Post(ctx, post.Body)
		} else {
			url := fmt.Sprintf("%s/posts/%s", baseURL(cmd), post.ID)
			truncated := string([]rune(post.Body)[:250]) + "..."
			uri, cid, err = client.PostWithLink(ctx, truncated, url, url)
		}
	}

	if err != nil {
		return err
	}

	_ = cid
	remoteURL := synd.BlueskyPostURL(handle, uri)

	if err := store.Syndicate(ctx, post.ID, synd.Bluesky, uri, remoteURL); err != nil {
		return fmt.Errorf("record syndication: %w", err)
	}

	fmt.Printf("bluesky: %s\n", remoteURL)
	return nil
}
