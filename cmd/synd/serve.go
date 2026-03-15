package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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

	w.cloudflare = cloudflareConfigPtr()

	if doSyndicate {
		w.bluesky = blueskyConfigPtr()
		w.mastodon = mastodonConfigPtr()
		w.threads = threadsConfigPtr()
	}

	go w.run(cmd.Context())
	slog.Info("worker started", "interval", pollInterval, "syndicate", doSyndicate)

	// Auth middleware
	authURL := os.Getenv("SYND_AUTH_URL")
	if authURL == "" {
		return fmt.Errorf("SYND_AUTH_URL must be set")
	}
	authClient := axon.NewAuthClientPlain(authURL)

	mux := buildMux(store, authClient, withRebuild(w.rebuildSite))

	slog.Info("serving", "addr", addr, "auth_url", authURL)
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

func threadsConfigPtr() *synd.ThreadsConfig {
	cfg, err := threadsConfigFromEnv()
	if err != nil {
		return nil
	}
	return &cfg
}

func cloudflareConfigPtr() *synd.CloudflareConfig {
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	projectName := os.Getenv("CLOUDFLARE_PROJECT_NAME")
	if accountID == "" || apiToken == "" || projectName == "" {
		return nil
	}
	return &synd.CloudflareConfig{
		AccountID:   accountID,
		APIToken:    apiToken,
		ProjectName: projectName,
	}
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

// muxConfig holds optional configuration for buildMux.
type muxConfig struct {
	rebuildFn func() error
}

// muxOption configures buildMux.
type muxOption func(*muxConfig)

// withRebuild registers a site rebuild function for the POST /api/site/rebuild endpoint.
func withRebuild(fn func() error) muxOption {
	return func(c *muxConfig) { c.rebuildFn = fn }
}

// buildMux creates the HTTP mux with auth middleware on all routes except /health.
// API routes return 401 for unauthenticated requests.
// Web routes redirect to the login page for unauthenticated requests.
func buildMux(store *synd.PostStore, sv axon.SessionValidator, opts ...muxOption) *http.ServeMux {
	var cfg muxConfig
	for _, o := range opts {
		o(&cfg)
	}

	requireAuth := axon.RequireAuth(sv)
	loginURL := authLoginURL()
	webAuth := webAuthRedirect(sv, loginURL)

	h := newWebHandler(store)
	api := newAPIHandler(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("POST /api/posts", requireAuth(http.HandlerFunc(api.CreatePost)))
	mux.Handle("GET /api/drafts", requireAuth(http.HandlerFunc(api.ListDrafts)))
	mux.Handle("POST /api/drafts/{id}/approve", requireAuth(http.HandlerFunc(api.ApprovePost)))
	mux.Handle("PUT /api/posts/{id}", requireAuth(http.HandlerFunc(api.RevisePost)))
	mux.Handle("DELETE /api/posts/{id}", requireAuth(http.HandlerFunc(api.DeletePost)))
	mux.Handle("GET /drafts/{id}", webAuth(http.HandlerFunc(h.ShowDraft)))
	mux.Handle("POST /drafts/{id}/revise", webAuth(http.HandlerFunc(h.ReviseDraft)))
	mux.Handle("POST /drafts/{id}/approve", webAuth(http.HandlerFunc(h.ApproveDraft)))

	if cfg.rebuildFn != nil {
		rebuildFn := cfg.rebuildFn
		mux.Handle("POST /api/site/rebuild", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := rebuildFn(); err != nil {
				slog.Error("site rebuild failed", "error", err)
				http.Error(w, "rebuild failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "rebuilt"})
		})))
	}

	return mux
}

func authLoginURL() string {
	authURL := os.Getenv("SYND_AUTH_URL")
	if authURL == "" {
		return "/login"
	}
	return authURL + "/login"
}

// webAuthRedirect wraps session validation to redirect unauthenticated browser
// requests to the login page instead of returning 401.
func webAuthRedirect(sv axon.SessionValidator, loginURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := r.Cookie("session")
			if err != nil || token.Value == "" {
				redirectToLogin(w, r, loginURL)
				return
			}

			session, err := sv.ValidateSession(token.Value)
			if err != nil {
				redirectToLogin(w, r, loginURL)
				return
			}

			ctx := context.WithValue(r.Context(), axon.SessionInfoKey, session)
			ctx = context.WithValue(ctx, axon.UserIDKey, session.UserID())
			ctx = context.WithValue(ctx, axon.UsernameKey, session.Username())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func redirectToLogin(w http.ResponseWriter, r *http.Request, loginURL string) {
	redirect := r.URL.String()
	target := loginURL + "?redirect=" + url.QueryEscape(redirect)
	http.Redirect(w, r, target, http.StatusFound)
}
