package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve [post-id]",
	Short: "Approve a draft post for publishing",
	Long: `Approve a draft post. The background worker will pick it up
and publish it to the site and syndicate to configured platforms.`,
	RunE: runApprove,
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(approveCmd)
}

func runApprove(cmd *cobra.Command, args []string) error {
	postID := args[0]

	url := fmt.Sprintf("%s/api/drafts/%s/approve", serviceURL(), postID)
	req, err := authedRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("post %s not found", postID)
	}
	if resp.StatusCode == http.StatusConflict {
		var msg string
		json.NewDecoder(resp.Body).Decode(&msg)
		return fmt.Errorf("post %s: %s", postID, msg)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d)", resp.StatusCode)
	}

	fmt.Printf("approved: %s (worker will publish)\n", postID)
	return nil
}
