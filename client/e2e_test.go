package client

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/server"
	"github.com/kisom/sgard/sgardpb"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// TestE2EPushPullCycle tests the full lifecycle:
// init two repos → add files to client → checkpoint → push → pull into fresh repo → verify
func TestE2EPushPullCycle(t *testing.T) {
	// Generate auth key.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	// Set up server.
	serverDir := t.TempDir()
	serverGarden, err := garden.Init(serverDir)
	if err != nil {
		t.Fatalf("init server: %v", err)
	}

	jwtKey := []byte("e2e-test-jwt-secret-key-32bytes!")
	auth := server.NewAuthInterceptorFromKeys([]ssh.PublicKey{signer.PublicKey()}, jwtKey)
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryInterceptor()),
		grpc.StreamInterceptor(auth.StreamInterceptor()),
	)
	sgardpb.RegisterGardenSyncServer(srv, server.NewWithAuth(serverGarden, auth))
	t.Cleanup(func() { srv.Stop() })
	go func() { _ = srv.Serve(lis) }()

	dial := func(t *testing.T) *Client {
		t.Helper()
		creds := NewTokenCredentials("")
		conn, err := grpc.NewClient("passthrough:///bufconn",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithPerRPCCredentials(creds),
		)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		c := NewWithAuth(conn, creds, signer)
		if err := c.EnsureAuth(context.Background()); err != nil {
			t.Fatalf("EnsureAuth: %v", err)
		}
		return c
	}

	ctx := context.Background()

	// --- Client A: add files and push ---
	clientADir := t.TempDir()
	clientA, err := garden.Init(clientADir)
	if err != nil {
		t.Fatalf("init client A: %v", err)
	}

	// Create test dotfiles.
	root := t.TempDir()
	bashrc := filepath.Join(root, "bashrc")
	sshConfig := filepath.Join(root, "ssh_config")
	if err := os.WriteFile(bashrc, []byte("export PS1='$ '\n"), 0o644); err != nil {
		t.Fatalf("writing bashrc: %v", err)
	}
	if err := os.WriteFile(sshConfig, []byte("Host *\n  AddKeysToAgent yes\n"), 0o600); err != nil {
		t.Fatalf("writing ssh_config: %v", err)
	}

	if err := clientA.Add([]string{bashrc, sshConfig}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := clientA.Checkpoint("from machine A"); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	c := dial(t)
	pushed, err := c.Push(ctx, clientA)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if pushed != 2 {
		t.Errorf("pushed %d blobs, want 2", pushed)
	}

	// --- Client B: pull from server ---
	clientBDir := t.TempDir()
	clientB, err := garden.Init(clientBDir)
	if err != nil {
		t.Fatalf("init client B: %v", err)
	}
	// Backdate so server is newer.
	bm := clientB.GetManifest()
	bm.Updated = bm.Updated.Add(-2 * time.Hour)
	if err := clientB.ReplaceManifest(bm); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	c2 := dial(t)
	pulled, err := c2.Pull(ctx, clientB)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if pulled != 2 {
		t.Errorf("pulled %d blobs, want 2", pulled)
	}

	// Verify client B has the same manifest and blobs as client A.
	manifestA := clientA.GetManifest()
	manifestB := clientB.GetManifest()

	if len(manifestB.Files) != len(manifestA.Files) {
		t.Fatalf("client B has %d entries, want %d", len(manifestB.Files), len(manifestA.Files))
	}

	for _, e := range manifestB.Files {
		if e.Type == "file" {
			dataA, err := clientA.ReadBlob(e.Hash)
			if err != nil {
				t.Fatalf("read blob from A: %v", err)
			}
			dataB, err := clientB.ReadBlob(e.Hash)
			if err != nil {
				t.Fatalf("read blob from B: %v", err)
			}
			if string(dataA) != string(dataB) {
				t.Errorf("blob %s content mismatch between A and B", e.Hash)
			}
		}
	}

	// Verify manifest message survived.
	if manifestB.Message != "from machine A" {
		t.Errorf("message = %q, want 'from machine A'", manifestB.Message)
	}
}
