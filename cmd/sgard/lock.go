package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock <path>...",
	Short: "Mark tracked files as locked (repo-authoritative)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.Lock(args); err != nil {
			return err
		}

		fmt.Printf("Locked %d path(s).\n", len(args))
		return nil
	},
}

var unlockCmd = &cobra.Command{
	Use:   "unlock <path>...",
	Short: "Remove locked flag from tracked files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.Unlock(args); err != nil {
			return err
		}

		fmt.Printf("Unlocked %d path(s).\n", len(args))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(unlockCmd)
}
