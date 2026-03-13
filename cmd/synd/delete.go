package main

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [post-id]",
	Short: "Delete a post",
	Long:  `Delete a post. It will be removed from the site on the next publish cycle.`,
	RunE:  runDelete,
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	postID := args[0]

	url := fmt.Sprintf("%s/api/posts/%s", serviceURL(), postID)
	req, err := authedRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("post %s not found", postID)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d)", resp.StatusCode)
	}

	fmt.Printf("deleted: %s\n", postID)
	return nil
}
