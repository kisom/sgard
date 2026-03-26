package main

import (
	"context"
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/manifest"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked files",
	Long:  "List all tracked files locally, or on the remote server with -r.",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _, _, _ := resolveRemoteConfig()
		if addr != "" {
			return listRemote()
		}
		return listLocal()
	},
}

func listLocal() error {
	g, err := garden.Open(repoFlag)
	if err != nil {
		return err
	}

	printEntries(g.List())
	return nil
}

func listRemote() error {
	ctx := context.Background()

	c, cleanup, err := dialRemote(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	entries, err := c.List(ctx)
	if err != nil {
		return err
	}

	printEntries(entries)
	return nil
}

func printEntries(entries []manifest.Entry) {
	for _, e := range entries {
		switch e.Type {
		case "file":
			hash := e.Hash
			if len(hash) > 8 {
				hash = hash[:8]
			}
			fmt.Printf("%-6s %s\t%s\n", "file", e.Path, hash)
		case "link":
			fmt.Printf("%-6s %s\t-> %s\n", "link", e.Path, e.Target)
		case "directory":
			fmt.Printf("%-6s %s\n", "dir", e.Path)
		}
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
