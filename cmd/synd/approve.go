package main

import (
	"fmt"
	"os/user"

	synd "github.com/benaskins/axon-synd"
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve [post-id]",
	Short: "Approve a draft post for publishing",
	Long: `Approve a draft post. The background worker will pick it up
and publish it to the site and syndicate to configured platforms.

Use --publish to also publish immediately (without waiting for the worker).`,
	RunE: runApprove,
	Args: cobra.ExactArgs(1),
}

func init() {
	approveCmd.Flags().Bool("publish", false, "also publish immediately after approving")
	rootCmd.AddCommand(approveCmd)
}

func runApprove(cmd *cobra.Command, args []string) error {
	postID := args[0]

	store, projection := newStoreFromCmd(cmd)
	ctx := cmd.Context()

	post := store.Get(postID)
	if post == nil {
		return fmt.Errorf("post %s not found", postID)
	}

	if post.Status != synd.StatusDraft {
		return fmt.Errorf("post %s is %s, not a draft", postID, post.Status)
	}

	username := "cli"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	if err := store.Approve(ctx, postID, username); err != nil {
		return fmt.Errorf("approve: %w", err)
	}

	fmt.Printf("approved: %s\n", postID)

	doPublish, _ := cmd.Flags().GetBool("publish")
	if doPublish {
		post = store.Get(postID)
		return publishPost(cmd, ctx, store, projection, post)
	}

	return nil
}
