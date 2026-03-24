package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of tracked files: ok, modified, or missing",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		statuses, err := g.Status()
		if err != nil {
			return err
		}

		for _, s := range statuses {
			fmt.Printf("%-10s %s\n", s.State, s.Path)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
