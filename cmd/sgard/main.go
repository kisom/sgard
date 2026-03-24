package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kisom/sgard/client"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	repoFlag   string
	remoteFlag string
	sshKeyFlag string
	tlsFlag    bool
	tlsCAFlag  string
)

var rootCmd = &cobra.Command{
	Use:   "sgard",
	Short: "Shimmering Clarity Gardener: dotfile management",
}

func defaultRepo() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sgard"
	}
	return filepath.Join(home, ".sgard")
}

// resolveRemote returns the remote address from flag, env, or repo config file.
func resolveRemote() (string, error) {
	if remoteFlag != "" {
		return remoteFlag, nil
	}
	if env := os.Getenv("SGARD_REMOTE"); env != "" {
		return env, nil
	}
	// Try <repo>/remote file.
	data, err := os.ReadFile(filepath.Join(repoFlag, "remote"))
	if err == nil {
		addr := strings.TrimSpace(string(data))
		if addr != "" {
			return addr, nil
		}
	}
	return "", fmt.Errorf("no remote configured; use --remote, SGARD_REMOTE, or create %s/remote", repoFlag)
}

// dialRemote creates a gRPC client with token-based auth and auto-renewal.
func dialRemote(ctx context.Context) (*client.Client, func(), error) {
	addr, err := resolveRemote()
	if err != nil {
		return nil, nil, err
	}

	signer, err := client.LoadSigner(sshKeyFlag)
	if err != nil {
		return nil, nil, err
	}

	cachedToken := client.LoadCachedToken()
	creds := client.NewTokenCredentials(cachedToken)

	var transportCreds grpc.DialOption
	if tlsFlag {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if tlsCAFlag != "" {
			caPEM, err := os.ReadFile(tlsCAFlag)
			if err != nil {
				return nil, nil, fmt.Errorf("reading CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, nil, fmt.Errorf("failed to parse CA cert %s", tlsCAFlag)
			}
			tlsCfg.RootCAs = pool
		}
		transportCreds = grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))
	} else {
		transportCreds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.NewClient(addr,
		transportCreds,
		grpc.WithPerRPCCredentials(creds),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}

	c := client.NewWithAuth(conn, creds, signer)

	// Ensure we have a valid token before proceeding.
	if err := c.EnsureAuth(ctx); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("authentication: %w", err)
	}

	cleanup := func() { _ = conn.Close() }
	return c, cleanup, nil
}

func main() {
	rootCmd.PersistentFlags().StringVar(&repoFlag, "repo", defaultRepo(), "path to sgard repository")
	rootCmd.PersistentFlags().StringVar(&remoteFlag, "remote", "", "gRPC server address (host:port)")
	rootCmd.PersistentFlags().StringVar(&sshKeyFlag, "ssh-key", "", "path to SSH private key")
	rootCmd.PersistentFlags().BoolVar(&tlsFlag, "tls", false, "use TLS for remote connection")
	rootCmd.PersistentFlags().StringVar(&tlsCAFlag, "tls-ca", "", "path to CA certificate for TLS verification")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
