package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var (
	targetOnlyFlag  []string
	targetNeverFlag []string
	targetClearFlag bool
)

var targetCmd = &cobra.Command{
	Use:   "target <path>",
	Short: "Set or clear targeting labels on a tracked entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if len(targetOnlyFlag) > 0 && len(targetNeverFlag) > 0 {
			return fmt.Errorf("--only and --never are mutually exclusive")
		}

		if err := g.SetTargeting(args[0], targetOnlyFlag, targetNeverFlag, targetClearFlag); err != nil {
			return err
		}

		if targetClearFlag {
			fmt.Printf("Cleared targeting for %s.\n", args[0])
		} else {
			fmt.Printf("Updated targeting for %s.\n", args[0])
		}
		return nil
	},
}

func init() {
	targetCmd.Flags().StringSliceVar(&targetOnlyFlag, "only", nil, "only apply on matching machines")
	targetCmd.Flags().StringSliceVar(&targetNeverFlag, "never", nil, "never apply on matching machines")
	targetCmd.Flags().BoolVar(&targetClearFlag, "clear", false, "remove all targeting labels")
	rootCmd.AddCommand(targetCmd)
}
