package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var reviseCmd = &cobra.Command{
	Use:   "revise [post-id]",
	Short: "Revise a post",
	Long: `Revise a post's content. Use --file to read the body from a markdown file.
After revising, use --rebuild to trigger a site rebuild.`,
	RunE: runRevise,
	Args: cobra.ExactArgs(1),
}

func init() {
	reviseCmd.Flags().StringP("file", "f", "", "read body from a markdown file")
	reviseCmd.Flags().String("title", "", "new title")
	reviseCmd.Flags().String("abstract", "", "new abstract")
	reviseCmd.Flags().Bool("rebuild", false, "rebuild the site after revising")
	rootCmd.AddCommand(reviseCmd)
}

func runRevise(cmd *cobra.Command, args []string) error {
	postID := args[0]
	file, _ := cmd.Flags().GetString("file")
	title, _ := cmd.Flags().GetString("title")
	abstract, _ := cmd.Flags().GetString("abstract")
	rebuild, _ := cmd.Flags().GetBool("rebuild")

	var body string
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		body = string(data)
	}

	if body == "" && title == "" && abstract == "" {
		return fmt.Errorf("nothing to revise: provide --file, --title, or --abstract")
	}

	// Revise the post
	payload := map[string]any{}
	if body != "" {
		payload["body"] = body
	}
	if title != "" {
		payload["title"] = title
	}
	if abstract != "" {
		payload["abstract"] = abstract
	}

	jsonBody, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/posts/%s", serviceURL(), postID)
	req, err := authedRequest("PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revise: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("post %s not found", postID)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d)", resp.StatusCode)
	}

	fmt.Printf("revised: %s\n", postID)

	// Rebuild if requested
	if rebuild {
		rebuildURL := fmt.Sprintf("%s/api/site/rebuild", serviceURL())
		req, err := authedRequest("POST", rebuildURL, nil)
		if err != nil {
			return fmt.Errorf("auth: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("rebuild: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("rebuild failed (%d)", resp.StatusCode)
		}

		fmt.Println("site rebuilt")
	}

	return nil
}
