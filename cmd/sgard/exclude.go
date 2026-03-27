package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var listExclude bool

var excludeCmd = &cobra.Command{
	Use:   "exclude [path...]",
	Short: "Exclude paths from tracking",
	Long:  "Add paths to the exclusion list. Excluded paths are skipped during add and mirror operations. Use --list to show current exclusions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if listExclude {
			for _, p := range g.GetManifest().Exclude {
				fmt.Println(p)
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide paths to exclude, or use --list")
		}

		if err := g.Exclude(args); err != nil {
			return err
		}

		fmt.Printf("Excluded %d path(s)\n", len(args))
		return nil
	},
}

var includeCmd = &cobra.Command{
	Use:   "include <path>...",
	Short: "Remove paths from the exclusion list",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.Include(args); err != nil {
			return err
		}

		fmt.Printf("Included %d path(s)\n", len(args))
		return nil
	},
}

func init() {
	excludeCmd.Flags().BoolVarP(&listExclude, "list", "l", false, "list current exclusions")
	rootCmd.AddCommand(excludeCmd)
	rootCmd.AddCommand(includeCmd)
}
