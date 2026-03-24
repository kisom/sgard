package main

import (
	"fmt"
	"net"
	"os"

	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/server"
	"github.com/kisom/sgard/sgardpb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	listenAddr     string
	repoPath       string
	authKeysPath   string
)

var rootCmd = &cobra.Command{
	Use:   "sgardd",
	Short: "sgard gRPC sync daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoPath)
		if err != nil {
			return fmt.Errorf("opening repo: %w", err)
		}

		var opts []grpc.ServerOption
		var srvInstance *server.Server

		if authKeysPath != "" {
			auth, err := server.NewAuthInterceptor(authKeysPath, repoPath)
			if err != nil {
				return fmt.Errorf("loading authorized keys: %w", err)
			}
			opts = append(opts,
				grpc.UnaryInterceptor(auth.UnaryInterceptor()),
				grpc.StreamInterceptor(auth.StreamInterceptor()),
			)
			srvInstance = server.NewWithAuth(g, auth)
			fmt.Printf("Auth enabled: %s\n", authKeysPath)
		} else {
			srvInstance = server.New(g)
			fmt.Println("WARNING: no --authorized-keys specified, running without authentication")
		}

		srv := grpc.NewServer(opts...)
		sgardpb.RegisterGardenSyncServer(srv, srvInstance)

		lis, err := net.Listen("tcp", listenAddr)
		if err != nil {
			return fmt.Errorf("listening on %s: %w", listenAddr, err)
		}

		fmt.Printf("sgardd serving on %s (repo: %s)\n", listenAddr, repoPath)
		return srv.Serve(lis)
	},
}

func main() {
	rootCmd.Flags().StringVar(&listenAddr, "listen", ":9473", "gRPC listen address")
	rootCmd.Flags().StringVar(&repoPath, "repo", "/srv/sgard", "path to sgard repository")
	rootCmd.Flags().StringVar(&authKeysPath, "authorized-keys", "", "path to authorized SSH public keys file")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
