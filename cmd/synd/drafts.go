package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type draftItem struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Body string `json:"body"`
}

func runDrafts(cmd *cobra.Command, args []string) error {
	resp, err := http.Get(serviceURL() + "/api/drafts")
	if err != nil {
		return fmt.Errorf("list drafts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d)", resp.StatusCode)
	}

	var drafts []draftItem
	if err := json.NewDecoder(resp.Body).Decode(&drafts); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	if len(drafts) == 0 {
		fmt.Println("no drafts")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tKIND\tBODY\n")
	for _, d := range drafts {
		body := d.Body
		if len(body) > 60 {
			body = body[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.ID, d.Kind, body)
	}
	w.Flush()

	return nil
}
