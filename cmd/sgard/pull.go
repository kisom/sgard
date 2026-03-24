package main

import (
	"context"
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull checkpoint from remote server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
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
		} else {
			fmt.Printf("Pulled %d blob(s).\n", pulled)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
