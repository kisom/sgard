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

// resolveRemoteConfig returns the effective remote address, TLS flag, and CA
// path by merging CLI flags, environment, and the saved remote.yaml config.
// CLI flags take precedence, then env, then the saved config.
func resolveRemoteConfig() (addr string, useTLS bool, caPath string, err error) {
	// Start with saved config as baseline.
	saved, _ := loadRemoteConfig()

	// Address: flag > env > saved > legacy file.
	addr = remoteFlag
	if addr == "" {
		addr = os.Getenv("SGARD_REMOTE")
	}
	if addr == "" && saved != nil {
		addr = saved.Addr
	}
	if addr == "" {
		data, ferr := os.ReadFile(filepath.Join(repoFlag, "remote"))
		if ferr == nil {
			addr = strings.TrimSpace(string(data))
		}
	}
	if addr == "" {
		return "", false, "", fmt.Errorf("no remote configured; use 'sgard remote set' or --remote")
	}

	// TLS: flag wins if explicitly set, otherwise use saved.
	useTLS = tlsFlag
	if !useTLS && saved != nil {
		useTLS = saved.TLS
	}

	// CA: flag wins if set, otherwise use saved.
	caPath = tlsCAFlag
	if caPath == "" && saved != nil {
		caPath = saved.TLSCA
	}

	return addr, useTLS, caPath, nil
}

// dialRemote creates a gRPC client with token-based auth and auto-renewal.
func dialRemote(ctx context.Context) (*client.Client, func(), error) {
	addr, useTLS, caPath, err := resolveRemoteConfig()
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
	if useTLS {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if caPath != "" {
			caPEM, err := os.ReadFile(caPath)
			if err != nil {
				return nil, nil, fmt.Errorf("reading CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, nil, fmt.Errorf("failed to parse CA cert %s", caPath)
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
	rootCmd.PersistentFlags().StringVarP(&remoteFlag, "remote", "r", "", "gRPC server address (host:port)")
	rootCmd.PersistentFlags().StringVar(&sshKeyFlag, "ssh-key", "", "path to SSH private key")
	rootCmd.PersistentFlags().BoolVar(&tlsFlag, "tls", false, "use TLS for remote connection")
	rootCmd.PersistentFlags().StringVar(&tlsCAFlag, "tls-ca", "", "path to CA certificate for TLS verification")
	rootCmd.PersistentFlags().StringVar(&fido2PinFlag, "fido2-pin", "", "PIN for FIDO2 device (if PIN-protected)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
