package main

import (
	"context"
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove orphaned blobs not referenced by the manifest",
	Long:  "Remove orphaned blobs locally, or on the remote server with --remote.",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := resolveRemote()

		if addr != "" {
			return pruneRemote()
		}
		return pruneLocal()
	},
}

func pruneLocal() error {
	g, err := garden.Open(repoFlag)
	if err != nil {
		return err
	}

	removed, err := g.Prune()
	if err != nil {
		return err
	}

	fmt.Printf("Pruned %d orphaned blob(s).\n", removed)
	return nil
}

func pruneRemote() error {
	ctx := context.Background()

	c, cleanup, err := dialRemote(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	removed, err := c.Prune(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Pruned %d orphaned blob(s) on remote.\n", removed)
	return nil
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}
