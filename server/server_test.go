package server

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/manifest"
	"github.com/kisom/sgard/sgardpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// setupTest creates a client-server pair using in-process bufconn.
// It returns a gRPC client, the server Garden, and a client Garden.
func setupTest(t *testing.T) (sgardpb.GardenSyncClient, *garden.Garden, *garden.Garden) {
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
	sgardpb.RegisterGardenSyncServer(srv, New(serverGarden))
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

	client := sgardpb.NewGardenSyncClient(conn)
	return client, serverGarden, clientGarden
}

func TestPushManifest_Accepted(t *testing.T) {
	client, serverGarden, _ := setupTest(t)
	ctx := context.Background()

	// Server has an old manifest (default init time).
	// Client has a newer manifest with a file entry.
	now := time.Now().UTC()
	clientManifest := &manifest.Manifest{
		Version: 1,
		Created: now,
		Updated: now.Add(time.Hour),
		Files: []manifest.Entry{
			{
				Path:    "~/.bashrc",
				Hash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Type:    "file",
				Mode:    "0644",
				Updated: now,
			},
		},
	}

	resp, err := client.PushManifest(ctx, &sgardpb.PushManifestRequest{
		Manifest: ManifestToProto(clientManifest),
	})
	if err != nil {
		t.Fatalf("PushManifest: %v", err)
	}

	if resp.Decision != sgardpb.PushManifestResponse_ACCEPTED {
		t.Errorf("decision: got %v, want ACCEPTED", resp.Decision)
	}

	// The blob doesn't exist on server, so it should be in missing_blobs.
	if len(resp.MissingBlobs) != 1 {
		t.Fatalf("missing_blobs count: got %d, want 1", len(resp.MissingBlobs))
	}
	if resp.MissingBlobs[0] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("missing_blobs[0]: got %s, want aaaa...", resp.MissingBlobs[0])
	}

	// Write the blob to server and try again: it should not be missing.
	_, err = serverGarden.WriteBlob([]byte("test data"))
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
}

func TestPushManifest_Rejected(t *testing.T) {
	client, serverGarden, _ := setupTest(t)
	ctx := context.Background()

	// Make the server manifest newer.
	serverManifest := serverGarden.GetManifest()
	serverManifest.Updated = time.Now().UTC().Add(2 * time.Hour)
	if err := serverGarden.ReplaceManifest(serverManifest); err != nil {
		t.Fatalf("ReplaceManifest: %v", err)
	}

	// Client manifest is at default init time (older).
	clientManifest := &manifest.Manifest{
		Version: 1,
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
		Files:   []manifest.Entry{},
	}

	resp, err := client.PushManifest(ctx, &sgardpb.PushManifestRequest{
		Manifest: ManifestToProto(clientManifest),
	})
	if err != nil {
		t.Fatalf("PushManifest: %v", err)
	}

	if resp.Decision != sgardpb.PushManifestResponse_REJECTED {
		t.Errorf("decision: got %v, want REJECTED", resp.Decision)
	}
}

func TestPushManifest_UpToDate(t *testing.T) {
	client, serverGarden, _ := setupTest(t)
	ctx := context.Background()

	// Set both to the same timestamp.
	ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	serverManifest := serverGarden.GetManifest()
	serverManifest.Updated = ts
	if err := serverGarden.ReplaceManifest(serverManifest); err != nil {
		t.Fatalf("ReplaceManifest: %v", err)
	}

	clientManifest := &manifest.Manifest{
		Version: 1,
		Created: ts,
		Updated: ts,
		Files:   []manifest.Entry{},
	}

	resp, err := client.PushManifest(ctx, &sgardpb.PushManifestRequest{
		Manifest: ManifestToProto(clientManifest),
	})
	if err != nil {
		t.Fatalf("PushManifest: %v", err)
	}

	if resp.Decision != sgardpb.PushManifestResponse_UP_TO_DATE {
		t.Errorf("decision: got %v, want UP_TO_DATE", resp.Decision)
	}
}

func TestPushAndPullBlobs(t *testing.T) {
	client, serverGarden, _ := setupTest(t)
	ctx := context.Background()

	// Write some test data as blobs directly to simulate a client garden.
	blob1Data := []byte("hello world from bashrc")
	blob2Data := []byte("vimrc content here")

	// We need the actual hashes for our manifest entries.
	// Write to a throwaway garden to get hashes.
	tmpDir := t.TempDir()
	tmpGarden, err := garden.Init(tmpDir)
	if err != nil {
		t.Fatalf("init tmp garden: %v", err)
	}
	hash1, err := tmpGarden.WriteBlob(blob1Data)
	if err != nil {
		t.Fatalf("WriteBlob 1: %v", err)
	}
	hash2, err := tmpGarden.WriteBlob(blob2Data)
	if err != nil {
		t.Fatalf("WriteBlob 2: %v", err)
	}

	now := time.Now().UTC().Add(time.Hour)
	clientManifest := &manifest.Manifest{
		Version: 1,
		Created: now,
		Updated: now,
		Files: []manifest.Entry{
			{Path: "~/.bashrc", Hash: hash1, Type: "file", Mode: "0644", Updated: now},
			{Path: "~/.vimrc", Hash: hash2, Type: "file", Mode: "0644", Updated: now},
			{Path: "~/.config", Type: "directory", Mode: "0755", Updated: now},
		},
	}

	// Step 1: PushManifest.
	pushResp, err := client.PushManifest(ctx, &sgardpb.PushManifestRequest{
		Manifest: ManifestToProto(clientManifest),
	})
	if err != nil {
		t.Fatalf("PushManifest: %v", err)
	}
	if pushResp.Decision != sgardpb.PushManifestResponse_ACCEPTED {
		t.Fatalf("decision: got %v, want ACCEPTED", pushResp.Decision)
	}
	if len(pushResp.MissingBlobs) != 2 {
		t.Fatalf("missing_blobs: got %d, want 2", len(pushResp.MissingBlobs))
	}

	// Step 2: PushBlobs.
	stream, err := client.PushBlobs(ctx)
	if err != nil {
		t.Fatalf("PushBlobs: %v", err)
	}

	// Send blob1.
	if err := stream.Send(&sgardpb.PushBlobsRequest{
		Chunk: &sgardpb.BlobChunk{Hash: hash1, Data: blob1Data},
	}); err != nil {
		t.Fatalf("Send blob1: %v", err)
	}

	// Send blob2.
	if err := stream.Send(&sgardpb.PushBlobsRequest{
		Chunk: &sgardpb.BlobChunk{Hash: hash2, Data: blob2Data},
	}); err != nil {
		t.Fatalf("Send blob2: %v", err)
	}

	blobResp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if blobResp.BlobsReceived != 2 {
		t.Errorf("blobs_received: got %d, want 2", blobResp.BlobsReceived)
	}

	// Verify blobs exist on server.
	if !serverGarden.BlobExists(hash1) {
		t.Error("blob1 not found on server")
	}
	if !serverGarden.BlobExists(hash2) {
		t.Error("blob2 not found on server")
	}

	// Verify manifest was applied on server.
	sm := serverGarden.GetManifest()
	if len(sm.Files) != 3 {
		t.Fatalf("server manifest files: got %d, want 3", len(sm.Files))
	}

	// Step 3: PullManifest from the server.
	pullMResp, err := client.PullManifest(ctx, &sgardpb.PullManifestRequest{})
	if err != nil {
		t.Fatalf("PullManifest: %v", err)
	}
	pulledManifest := ProtoToManifest(pullMResp.GetManifest())
	if len(pulledManifest.Files) != 3 {
		t.Fatalf("pulled manifest files: got %d, want 3", len(pulledManifest.Files))
	}

	// Step 4: PullBlobs from the server.
	pullBResp, err := client.PullBlobs(ctx, &sgardpb.PullBlobsRequest{
		Hashes: []string{hash1, hash2},
	})
	if err != nil {
		t.Fatalf("PullBlobs: %v", err)
	}

	// Reassemble blobs from the stream.
	pulledBlobs := make(map[string][]byte)
	var currentHash string
	for {
		resp, err := pullBResp.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("PullBlobs Recv: %v", err)
		}
		chunk := resp.GetChunk()
		if chunk.GetHash() != "" {
			currentHash = chunk.GetHash()
		}
		pulledBlobs[currentHash] = append(pulledBlobs[currentHash], chunk.GetData()...)
	}

	if string(pulledBlobs[hash1]) != string(blob1Data) {
		t.Errorf("blob1 data mismatch: got %q, want %q", pulledBlobs[hash1], blob1Data)
	}
	if string(pulledBlobs[hash2]) != string(blob2Data) {
		t.Errorf("blob2 data mismatch: got %q, want %q", pulledBlobs[hash2], blob2Data)
	}
}

func TestPrune(t *testing.T) {
	client, serverGarden, _ := setupTest(t)
	ctx := context.Background()

	// Write a blob to the server.
	blobData := []byte("orphan blob data")
	hash, err := serverGarden.WriteBlob(blobData)
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}

	// The manifest does NOT reference this blob, so it is orphaned.
	if !serverGarden.BlobExists(hash) {
		t.Fatal("blob should exist before prune")
	}

	resp, err := client.Prune(ctx, &sgardpb.PruneRequest{})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if resp.BlobsRemoved != 1 {
		t.Errorf("blobs_removed: got %d, want 1", resp.BlobsRemoved)
	}

	if serverGarden.BlobExists(hash) {
		t.Error("orphan blob should be deleted after prune")
	}
}
