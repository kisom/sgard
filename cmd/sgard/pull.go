package main

import (
	"context"
	"fmt"

	"github.com/kisom/sgard/client"
	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull checkpoint from remote server",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := resolveRemote()
		if err != nil {
			return err
		}

		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		signer, err := client.LoadSigner(sshKeyFlag)
		if err != nil {
			return err
		}

		creds := client.NewSSHCredentials(signer)
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithPerRPCCredentials(creds),
		)
		if err != nil {
			return fmt.Errorf("connecting to %s: %w", addr, err)
		}
		defer func() { _ = conn.Close() }()

		c := client.New(conn)
		pulled, err := c.Pull(context.Background(), g)
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
