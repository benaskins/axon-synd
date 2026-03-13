package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server for draft review",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().String("addr", ":8094", "listen address")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	store, _ := newStoreFromCmd(cmd)

	h := newWebHandler(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /drafts/{id}", h.ShowDraft)
	mux.HandleFunc("POST /drafts/{id}/revise", h.ReviseDraft)
	mux.HandleFunc("POST /drafts/{id}/approve", h.ApproveDraft)

	slog.Info("serving", "addr", addr)
	fmt.Printf("listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}
