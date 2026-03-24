package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/kisom/sgard/client"
	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local checkpoint to remote server",
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
		pushed, err := c.Push(context.Background(), g)
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
