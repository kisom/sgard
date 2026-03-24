package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <path>",
	Short: "Show differences between stored and current file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		d, err := g.Diff(args[0])
		if err != nil {
			return err
		}

		if d == "" {
			fmt.Println("No changes.")
		} else {
			fmt.Print(d)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
