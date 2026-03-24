package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <path>...",
	Short: "Track files, directories, or symlinks",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.Add(args); err != nil {
			return err
		}

		fmt.Printf("Added %d path(s)\n", len(args))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
