package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "synd",
	Short:   "Personal syndication engine — publish once, syndicate everywhere",
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().String("site-dir", "", "path to site repo (default: SYND_SITE_DIR env)")
	rootCmd.PersistentFlags().String("base-url", "", "site base URL (default: SYND_BASE_URL env or https://generativeplane.com)")
}

func siteDir(cmd *cobra.Command) string {
	if d, _ := cmd.Flags().GetString("site-dir"); d != "" {
		return d
	}
	if d := os.Getenv("SYND_SITE_DIR"); d != "" {
		return d
	}
	return ""
}

func baseURL(cmd *cobra.Command) string {
	if u, _ := cmd.Flags().GetString("base-url"); u != "" {
		return u
	}
	if u := os.Getenv("SYND_BASE_URL"); u != "" {
		return u
	}
	return "https://generativeplane.com"
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
