package client

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
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

const bufSize = 1024 * 1024

// setupTest creates a gRPC client, server garden, and client garden
// connected via in-process bufconn.
func setupTest(t *testing.T) (*Client, *garden.Garden, *garden.Garden) {
	t.Helper()

	serverDir := t.TempDir()
	serverGarden, err := garden.Init(serverDir)
	if err != nil {
		t.Fatalf("init server garden: %v", err)
	}

	clientDir := t.TempDir()
	clientGarden, err := garden.Init(clientDir)
	if err != nil {
		t.Fatalf("init client garden: %v", err)
	}

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	sgardpb.RegisterGardenSyncServer(srv, server.New(serverGarden))
	t.Cleanup(func() { srv.Stop() })

	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	c := New(conn)
	return c, serverGarden, clientGarden
}

func TestPushAndPull(t *testing.T) {
	c, serverGarden, clientGarden := setupTest(t)
	ctx := context.Background()

	// Create files in a temp directory and add them to the client garden.
	root := t.TempDir()
	bashrc := filepath.Join(root, "bashrc")
	gitconfig := filepath.Join(root, "gitconfig")
	if err := os.WriteFile(bashrc, []byte("export PS1='$ '\n"), 0o644); err != nil {
		t.Fatalf("writing bashrc: %v", err)
	}
	if err := os.WriteFile(gitconfig, []byte("[user]\n\tname = test\n"), 0o644); err != nil {
		t.Fatalf("writing gitconfig: %v", err)
	}

	if err := clientGarden.Add([]string{bashrc, gitconfig}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := clientGarden.Checkpoint("initial"); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	// Push from client to server.
	pushed, err := c.Push(ctx, clientGarden)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if pushed != 2 {
		t.Errorf("pushed %d blobs, want 2", pushed)
	}

	// Verify server has the blobs.
	clientManifest := clientGarden.GetManifest()
	for _, e := range clientManifest.Files {
		if e.Type == "file" && !serverGarden.BlobExists(e.Hash) {
			t.Errorf("server missing blob for %s", e.Path)
		}
	}

	// Verify server manifest matches.
	serverManifest := serverGarden.GetManifest()
	if len(serverManifest.Files) != len(clientManifest.Files) {
		t.Errorf("server has %d entries, want %d", len(serverManifest.Files), len(clientManifest.Files))
	}

	// Pull into a fresh garden. Backdate its manifest so the server is "newer".
	freshDir := t.TempDir()
	freshGarden, err := garden.Init(freshDir)
	if err != nil {
		t.Fatalf("init fresh garden: %v", err)
	}
	oldManifest := freshGarden.GetManifest()
	oldManifest.Updated = oldManifest.Updated.Add(-2 * time.Hour)
	if err := freshGarden.ReplaceManifest(oldManifest); err != nil {
		t.Fatalf("backdate fresh manifest: %v", err)
	}

	pulled, err := c.Pull(ctx, freshGarden)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if pulled != 2 {
		t.Errorf("pulled %d blobs, want 2", pulled)
	}

	// Verify fresh garden has the correct manifest and blobs.
	freshManifest := freshGarden.GetManifest()
	if len(freshManifest.Files) != len(clientManifest.Files) {
		t.Fatalf("fresh garden has %d entries, want %d", len(freshManifest.Files), len(clientManifest.Files))
	}
	for _, e := range freshManifest.Files {
		if e.Type == "file" && !freshGarden.BlobExists(e.Hash) {
			t.Errorf("fresh garden missing blob for %s", e.Path)
		}
	}
}

func TestPushServerNewer(t *testing.T) {
	c, serverGarden, clientGarden := setupTest(t)
	ctx := context.Background()

	// Make server newer by checkpointing it.
	root := t.TempDir()
	f := filepath.Join(root, "file")
	if err := os.WriteFile(f, []byte("server file"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	if err := serverGarden.Add([]string{f}); err != nil {
		t.Fatalf("server Add: %v", err)
	}
	if err := serverGarden.Checkpoint("server ahead"); err != nil {
		t.Fatalf("server Checkpoint: %v", err)
	}

	_, err := c.Push(ctx, clientGarden)
	if !errors.Is(err, ErrServerNewer) {
		t.Errorf("expected ErrServerNewer, got %v", err)
	}
}

func TestPushUpToDate(t *testing.T) {
	c, _, clientGarden := setupTest(t)
	ctx := context.Background()

	// Both gardens are freshly initialized with same timestamp (approximately).
	// Push should return 0 blobs.
	pushed, err := c.Push(ctx, clientGarden)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if pushed != 0 {
		t.Errorf("pushed %d blobs, want 0 for up-to-date", pushed)
	}
}

func TestPullUpToDate(t *testing.T) {
	c, _, clientGarden := setupTest(t)
	ctx := context.Background()

	pulled, err := c.Pull(ctx, clientGarden)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if pulled != 0 {
		t.Errorf("pulled %d blobs, want 0 for up-to-date", pulled)
	}
}

func TestPrune(t *testing.T) {
	c, serverGarden, _ := setupTest(t)
	ctx := context.Background()

	// Write an orphan blob to the server.
	_, err := serverGarden.WriteBlob([]byte("orphan"))
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}

	removed, err := c.Prune(ctx)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d blobs, want 1", removed)
	}
}

var testJWTKey = []byte("test-jwt-secret-key-32-bytes!!")

func TestTokenAuthIntegration(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	serverDir := t.TempDir()
	serverGarden, err := garden.Init(serverDir)
	if err != nil {
		t.Fatalf("init server garden: %v", err)
	}

	auth := server.NewAuthInterceptorFromKeys([]ssh.PublicKey{signer.PublicKey()}, testJWTKey)
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryInterceptor()),
		grpc.StreamInterceptor(auth.StreamInterceptor()),
	)
	sgardpb.RegisterGardenSyncServer(srv, server.NewWithAuth(serverGarden, auth))
	t.Cleanup(func() { srv.Stop() })
	go func() { _ = srv.Serve(lis) }()

	// Client with token auth + auto-renewal.
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

	// No token yet — EnsureAuth should authenticate via SSH.
	ctx := context.Background()
	if err := c.EnsureAuth(ctx); err != nil {
		t.Fatalf("EnsureAuth: %v", err)
	}

	// Now requests should work.
	_, err = c.Pull(ctx, serverGarden)
	if err != nil {
		t.Fatalf("authenticated Pull should succeed: %v", err)
	}
}

func TestAuthRejectsUnauthenticated(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	serverDir := t.TempDir()
	serverGarden, err := garden.Init(serverDir)
	if err != nil {
		t.Fatalf("init server garden: %v", err)
	}

	auth := server.NewAuthInterceptorFromKeys([]ssh.PublicKey{signer.PublicKey()}, testJWTKey)
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryInterceptor()),
		grpc.StreamInterceptor(auth.StreamInterceptor()),
	)
	sgardpb.RegisterGardenSyncServer(srv, server.NewWithAuth(serverGarden, auth))
	t.Cleanup(func() { srv.Stop() })
	go func() { _ = srv.Serve(lis) }()

	// Client WITHOUT credentials — no token, no signer.
	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	c := New(conn)

	_, err = c.Pull(context.Background(), serverGarden)
	if err == nil {
		t.Fatal("unauthenticated Pull should fail")
	}
}
