package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <path>...",
	Short: "Stop tracking files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.Remove(args); err != nil {
			return err
		}

		fmt.Printf("Removed %d path(s)\n", len(args))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
