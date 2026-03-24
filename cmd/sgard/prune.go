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

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove orphaned blobs not referenced by the manifest",
	Long:  "Remove orphaned blobs locally, or on the remote server with --remote.",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := resolveRemote()

		if addr != "" {
			return pruneRemote(addr)
		}
		return pruneLocal()
	},
}

func pruneLocal() error {
	g, err := garden.Open(repoFlag)
	if err != nil {
		return err
	}

	removed, err := g.Prune()
	if err != nil {
		return err
	}

	fmt.Printf("Pruned %d orphaned blob(s).\n", removed)
	return nil
}

func pruneRemote(addr string) error {
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
	removed, err := c.Prune(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Pruned %d orphaned blob(s) on remote.\n", removed)
	return nil
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}
