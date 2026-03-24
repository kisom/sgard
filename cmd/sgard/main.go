package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var repoFlag string

var rootCmd = &cobra.Command{
	Use:   "sgard",
	Short: "Shimmering Clarity Gardener: dotfile management",
}

func defaultRepo() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sgard"
	}
	return filepath.Join(home, ".sgard")
}

func main() {
	rootCmd.PersistentFlags().StringVar(&repoFlag, "repo", defaultRepo(), "path to sgard repository")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
