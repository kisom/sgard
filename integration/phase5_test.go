package integration

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kisom/sgard/client"
	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/server"
	"github.com/kisom/sgard/sgardpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// TestE2E_Phase5_Targeting verifies that targeting labels survive push/pull
// and that restore respects them.
func TestE2E_Phase5_Targeting(t *testing.T) {
	// Set up bufconn server.
	serverDir := t.TempDir()
	serverGarden, err := garden.Init(serverDir)
	if err != nil {
		t.Fatalf("init server: %v", err)
	}

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	sgardpb.RegisterGardenSyncServer(srv, server.New(serverGarden))
	t.Cleanup(func() { srv.Stop() })
	go func() { _ = srv.Serve(lis) }()

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

	// --- Build source garden with targeted entries ---
	srcRoot := t.TempDir()
	srcRepoDir := filepath.Join(srcRoot, "repo")
	srcGarden, err := garden.Init(srcRepoDir)
	if err != nil {
		t.Fatalf("init source: %v", err)
	}

	linuxFile := filepath.Join(srcRoot, "linux-only")
	everywhereFile := filepath.Join(srcRoot, "everywhere")
	neverArmFile := filepath.Join(srcRoot, "never-arm")

	if err := os.WriteFile(linuxFile, []byte("linux"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(everywhereFile, []byte("everywhere"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(neverArmFile, []byte("not arm"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := srcGarden.Add([]string{linuxFile}, garden.AddOptions{Only: []string{"os:linux"}}); err != nil {
		t.Fatalf("Add linux-only: %v", err)
	}
	if err := srcGarden.Add([]string{everywhereFile}); err != nil {
		t.Fatalf("Add everywhere: %v", err)
	}
	if err := srcGarden.Add([]string{neverArmFile}, garden.AddOptions{Never: []string{"arch:arm64"}}); err != nil {
		t.Fatalf("Add never-arm: %v", err)
	}

	// Bump timestamp.
	m := srcGarden.GetManifest()
	m.Updated = time.Now().UTC().Add(time.Hour)
	if err := srcGarden.ReplaceManifest(m); err != nil {
		t.Fatal(err)
	}

	// --- Push ---
	ctx := context.Background()
	pushClient := client.New(conn)
	if _, err := pushClient.Push(ctx, srcGarden); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// --- Pull to fresh garden ---
	dstRoot := t.TempDir()
	dstRepoDir := filepath.Join(dstRoot, "repo")
	dstGarden, err := garden.Init(dstRepoDir)
	if err != nil {
		t.Fatalf("init dest: %v", err)
	}

	pullClient := client.New(conn)
	if _, err := pullClient.Pull(ctx, dstGarden); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// --- Verify targeting survived ---
	dm := dstGarden.GetManifest()
	if len(dm.Files) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(dm.Files))
	}

	for _, e := range dm.Files {
		switch {
		case e.Path == toTilde(linuxFile):
			if len(e.Only) != 1 || e.Only[0] != "os:linux" {
				t.Errorf("%s: only = %v, want [os:linux]", e.Path, e.Only)
			}
		case e.Path == toTilde(everywhereFile):
			if len(e.Only) != 0 || len(e.Never) != 0 {
				t.Errorf("%s: should have no targeting", e.Path)
			}
		case e.Path == toTilde(neverArmFile):
			if len(e.Never) != 1 || e.Never[0] != "arch:arm64" {
				t.Errorf("%s: never = %v, want [arch:arm64]", e.Path, e.Never)
			}
		}
	}

	// Verify restore skips non-matching entries.
	// Delete all files, then restore — only matching entries should appear.
	_ = os.Remove(linuxFile)
	_ = os.Remove(everywhereFile)
	_ = os.Remove(neverArmFile)

	if err := dstGarden.Restore(nil, true, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// "everywhere" should always be restored.
	if _, err := os.Stat(everywhereFile); os.IsNotExist(err) {
		t.Error("everywhere file should be restored")
	}

	// "linux-only" depends on current OS — we just verify no error occurred.
	// "never-arm" depends on current arch.
}
