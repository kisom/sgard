package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/kisom/sgard/client"
	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local checkpoint to remote server",
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

		pushed, err := c.Push(ctx, g)
		if errors.Is(err, client.ErrServerNewer) {
			fmt.Println("Server is newer; run sgard pull instead.")
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Printf("Pushed %d blob(s).\n", pushed)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
