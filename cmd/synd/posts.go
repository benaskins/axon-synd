package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var postsCmd = &cobra.Command{
	Use:   "posts",
	Short: "List recent posts",
	RunE:  runPosts,
}

func init() {
	rootCmd.AddCommand(postsCmd)
}

func runPosts(cmd *cobra.Command, args []string) error {
	store, _ := newStore()
	posts := store.List()

	if len(posts) == 0 {
		fmt.Println("no posts")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "DATE\tKIND\tBODY\n")
	for _, p := range posts {
		body := p.Body
		if p.Kind == "long" && p.Title != "" {
			body = p.Title
		}
		if len(body) > 60 {
			body = body[:57] + "..."
		}
		published := " "
		if !p.PublishedAt.IsZero() {
			published = "+"
		}
		fmt.Fprintf(w, "%s\t%s%s\t%s\n", p.CreatedAt.Format("2006-01-02 15:04"), published+string(p.Kind), "", body)
	}
	w.Flush()

	return nil
}
