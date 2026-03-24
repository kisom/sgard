package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var checkpointMessage string

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Re-hash all tracked files and update the manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.Checkpoint(checkpointMessage); err != nil {
			return err
		}

		fmt.Println("Checkpoint complete.")
		return nil
	},
}

func init() {
	checkpointCmd.Flags().StringVarP(&checkpointMessage, "message", "m", "", "checkpoint message")
	rootCmd.AddCommand(checkpointCmd)
}
