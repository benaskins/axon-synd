package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var draftsCmd = &cobra.Command{
	Use:   "drafts",
	Short: "List posts awaiting approval",
	RunE:  runDrafts,
}

func init() {
	rootCmd.AddCommand(draftsCmd)
}

func runDrafts(cmd *cobra.Command, args []string) error {
	store, _ := newStoreFromCmd(cmd)
	drafts := store.Projection().Drafts()

	if len(drafts) == 0 {
		fmt.Println("no drafts")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tKIND\tBODY\n")
	for _, d := range drafts {
		body := d.Body
		if d.Kind == "long" && d.Title != "" {
			body = d.Title
		}
		if len(body) > 60 {
			body = body[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.ID, d.Kind, body)
	}
	w.Flush()

	return nil
}
