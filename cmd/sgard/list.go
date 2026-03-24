package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked files",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		entries := g.List()
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

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
