package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/benaskins/axon"
	fact "github.com/benaskins/axon-fact"
	synd "github.com/benaskins/axon-synd"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server and background publish worker",
	Long: `Start the review web server and background worker.

The web server handles draft review, revision, and approval.
The worker polls for approved posts and publishes + syndicates them.`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().String("addr", ":8094", "listen address")
	serveCmd.Flags().Duration("poll-interval", 10*time.Second, "worker poll interval")
	serveCmd.Flags().Bool("syndicate", true, "syndicate to platforms after publishing")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
	doSyndicate, _ := cmd.Flags().GetBool("syndicate")

	store, projection := newStoreFromCmd(cmd)
	dir := siteDir(cmd)
	if dir == "" {
		return fmt.Errorf("site directory required: set --site-dir or SYND_SITE_DIR")
	}

	// Start background publish worker
	w := &publishWorker{
		store:      store,
		projection: projection,
		siteDir:    dir,
		baseURL:    baseURL(cmd),
		interval:   pollInterval,
	}

	if doSyndicate {
		w.bluesky = blueskyConfigPtr()
		w.mastodon = mastodonConfigPtr()
	}

	go w.run(cmd.Context())
	slog.Info("worker started", "interval", pollInterval, "syndicate", doSyndicate)

	// Start web server
	h := newWebHandler(store)
	api := newAPIHandler(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /api/posts", api.CreatePost)
	mux.HandleFunc("GET /api/drafts", api.ListDrafts)
	mux.HandleFunc("POST /api/drafts/{id}/approve", api.ApprovePost)
	mux.HandleFunc("GET /drafts/{id}", h.ShowDraft)
	mux.HandleFunc("POST /drafts/{id}/revise", h.ReviseDraft)
	mux.HandleFunc("POST /drafts/{id}/approve", h.ApproveDraft)

	slog.Info("serving", "addr", addr)
	fmt.Printf("listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func blueskyConfigPtr() *synd.BlueskyConfig {
	cfg, err := blueskyConfigFromEnv()
	if err != nil {
		return nil
	}
	return &cfg
}

func mastodonConfigPtr() *synd.MastodonConfig {
	cfg, err := mastodonConfigFromEnv()
	if err != nil {
		return nil
	}
	return &cfg
}

func newStoreFromCmd(cmd *cobra.Command) (*synd.PostStore, *synd.PostProjection) {
	dsn := databaseURL(cmd)
	if dsn != "" {
		return newPersistentStore(cmd.Context(), dsn)
	}
	return newMemoryStore()
}

func newPersistentStore(ctx context.Context, dsn string) (*synd.PostStore, *synd.PostProjection) {
	db, err := axon.OpenDB(dsn, "synd")
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	if err := axon.RunMigrations(db, synd.Migrations); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}

	store := synd.NewPostStore(nil)
	projection := store.Projection()
	events := synd.NewPostgresEventStore(db, synd.WithPgProjector(projection))
	store.SetEventStore(events)

	if err := events.Replay(ctx); err != nil {
		slog.Error("replay events", "error", err)
		os.Exit(1)
	}

	return store, projection
}

func newMemoryStore() (*synd.PostStore, *synd.PostProjection) {
	store := synd.NewPostStore(nil)
	projection := store.Projection()
	events := fact.NewMemoryStore(fact.WithProjector(projection))
	store.SetEventStore(events)
	return store, projection
}
