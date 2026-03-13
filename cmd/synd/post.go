package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	synd "github.com/benaskins/axon-synd"
	"github.com/spf13/cobra"
)

var postCmd = &cobra.Command{
	Use:   "post [text]",
	Short: "Create a draft post via the synd server",
	Long: `Create a post and submit it to the synd server as a draft.

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
	rootCmd.AddCommand(postCmd)
}

func runPost(cmd *cobra.Command, args []string) error {
	isLong, _ := cmd.Flags().GetBool("long")
	imagePath, _ := cmd.Flags().GetString("image")
	title, _ := cmd.Flags().GetString("title")
	abstract, _ := cmd.Flags().GetString("abstract")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	var req createPostRequest

	switch {
	case isLong:
		if len(args) < 1 {
			return fmt.Errorf("--long requires a markdown file path")
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read %s: %w", args[0], err)
		}
		body := string(data)
		if title == "" {
			title = extractTitle(body)
		}
		if abstract == "" {
			abstract = extractAbstract(body)
		}
		req = createPostRequest{
			Kind:     synd.Long,
			Body:     body,
			Title:    title,
			Abstract: abstract,
			Tags:     tags,
		}
	case imagePath != "":
		body := strings.Join(args, " ")
		if body == "" {
			return fmt.Errorf("--image requires caption text")
		}
		req = createPostRequest{
			Kind: synd.Image,
			Body: body,
			Tags: tags,
		}
	default:
		body := strings.Join(args, " ")
		if body == "" {
			return fmt.Errorf("post text required")
		}
		req = createPostRequest{
			Kind: synd.Short,
			Body: body,
			Tags: tags,
		}
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := authedRequest("POST", serviceURL()+"/api/posts", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var msg string
		json.NewDecoder(resp.Body).Decode(&msg)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, msg)
	}

	var result postResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("draft: %s (%s) — awaiting approval\n", result.ID, result.Kind)
	return nil
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
