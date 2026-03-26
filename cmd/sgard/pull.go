package main

import (
	"context"
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull checkpoint from remote server and restore files",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		g, err := garden.Open(repoFlag)
		if err != nil {
			// Repo doesn't exist yet — init it so pull can populate it.
			g, err = garden.Init(repoFlag)
			if err != nil {
				return fmt.Errorf("init repo for pull: %w", err)
			}
		}

		c, cleanup, err := dialRemote(ctx)
		if err != nil {
			return err
		}
		defer cleanup()

		pulled, err := c.Pull(ctx, g)
		if err != nil {
			return err
		}

		if pulled == 0 {
			fmt.Println("Already up to date.")
			return nil
		}

		fmt.Printf("Pulled %d blob(s).\n", pulled)

		if g.HasEncryption() && g.NeedsDEK(g.List()) {
			if err := unlockDEK(g); err != nil {
				return err
			}
		}

		if err := g.Restore(nil, true, nil); err != nil {
			return fmt.Errorf("restore after pull: %w", err)
		}

		fmt.Println("Restore complete.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
