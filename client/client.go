// Package client provides a gRPC client for the sgard GardenSync service.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/server"
	"github.com/kisom/sgard/sgardpb"
	"google.golang.org/grpc"
)

const chunkSize = 64 * 1024 // 64 KiB

// Client wraps a gRPC connection to a GardenSync server.
type Client struct {
	rpc sgardpb.GardenSyncClient
}

// New creates a Client from an existing gRPC connection.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{rpc: sgardpb.NewGardenSyncClient(conn)}
}

// Push sends the local manifest and any missing blobs to the server.
// Returns the number of blobs sent, or an error. If the server is newer,
// returns ErrServerNewer.
func (c *Client) Push(ctx context.Context, g *garden.Garden) (int, error) {
	localManifest := g.GetManifest()

	// Step 1: send manifest, get decision.
	resp, err := c.rpc.PushManifest(ctx, &sgardpb.PushManifestRequest{
		Manifest: server.ManifestToProto(localManifest),
	})
	if err != nil {
		return 0, fmt.Errorf("push manifest: %w", err)
	}

	switch resp.Decision {
	case sgardpb.PushManifestResponse_REJECTED:
		return 0, ErrServerNewer
	case sgardpb.PushManifestResponse_UP_TO_DATE:
		return 0, nil
	case sgardpb.PushManifestResponse_ACCEPTED:
		// continue
	default:
		return 0, fmt.Errorf("unexpected decision: %v", resp.Decision)
	}

	// Step 2: stream missing blobs.
	if len(resp.MissingBlobs) == 0 {
		// Manifest accepted but no blobs needed — still need to call PushBlobs
		// to trigger manifest replacement on the server.
		stream, err := c.rpc.PushBlobs(ctx)
		if err != nil {
			return 0, fmt.Errorf("push blobs: %w", err)
		}
		_, err = stream.CloseAndRecv()
		if err != nil {
			return 0, fmt.Errorf("close push blobs: %w", err)
		}
		return 0, nil
	}

	stream, err := c.rpc.PushBlobs(ctx)
	if err != nil {
		return 0, fmt.Errorf("push blobs: %w", err)
	}

	for _, hash := range resp.MissingBlobs {
		data, err := g.ReadBlob(hash)
		if err != nil {
			return 0, fmt.Errorf("reading local blob %s: %w", hash, err)
		}

		for i := 0; i < len(data); i += chunkSize {
			end := i + chunkSize
			if end > len(data) {
				end = len(data)
			}
			chunk := &sgardpb.BlobChunk{Data: data[i:end]}
			if i == 0 {
				chunk.Hash = hash
			}
			if err := stream.Send(&sgardpb.PushBlobsRequest{Chunk: chunk}); err != nil {
				return 0, fmt.Errorf("sending blob chunk: %w", err)
			}
		}

		// Handle empty blobs.
		if len(data) == 0 {
			if err := stream.Send(&sgardpb.PushBlobsRequest{
				Chunk: &sgardpb.BlobChunk{Hash: hash},
			}); err != nil {
				return 0, fmt.Errorf("sending empty blob: %w", err)
			}
		}
	}

	blobResp, err := stream.CloseAndRecv()
	if err != nil {
		return 0, fmt.Errorf("close push blobs: %w", err)
	}

	return int(blobResp.BlobsReceived), nil
}

// Pull downloads the server's manifest and any missing blobs to the local garden.
// Returns the number of blobs received, or an error. If the local manifest is
// newer or equal, returns 0 with no error.
func (c *Client) Pull(ctx context.Context, g *garden.Garden) (int, error) {
	// Step 1: get server manifest.
	pullResp, err := c.rpc.PullManifest(ctx, &sgardpb.PullManifestRequest{})
	if err != nil {
		return 0, fmt.Errorf("pull manifest: %w", err)
	}

	serverManifest := server.ProtoToManifest(pullResp.GetManifest())
	localManifest := g.GetManifest()

	// If local is newer or equal, nothing to do.
	if !serverManifest.Updated.After(localManifest.Updated) {
		return 0, nil
	}

	// Step 2: compute missing blobs.
	var missingHashes []string
	for _, e := range serverManifest.Files {
		if e.Type == "file" && e.Hash != "" && !g.BlobExists(e.Hash) {
			missingHashes = append(missingHashes, e.Hash)
		}
	}

	// Step 3: pull missing blobs.
	blobCount := 0
	if len(missingHashes) > 0 {
		stream, err := c.rpc.PullBlobs(ctx, &sgardpb.PullBlobsRequest{
			Hashes: missingHashes,
		})
		if err != nil {
			return 0, fmt.Errorf("pull blobs: %w", err)
		}

		var currentHash string
		var buf []byte

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return 0, fmt.Errorf("receiving blob chunk: %w", err)
			}

			chunk := resp.GetChunk()
			if chunk.GetHash() != "" {
				// New blob starting. Write out the previous one.
				if currentHash != "" {
					if err := writeAndVerify(g, currentHash, buf); err != nil {
						return 0, err
					}
					blobCount++
				}
				currentHash = chunk.GetHash()
				buf = append([]byte(nil), chunk.GetData()...)
			} else {
				buf = append(buf, chunk.GetData()...)
			}
		}

		// Write the last blob.
		if currentHash != "" {
			if err := writeAndVerify(g, currentHash, buf); err != nil {
				return 0, err
			}
			blobCount++
		}
	}

	// Step 4: replace local manifest.
	if err := g.ReplaceManifest(serverManifest); err != nil {
		return 0, fmt.Errorf("replacing local manifest: %w", err)
	}

	return blobCount, nil
}

// Prune requests the server to remove orphaned blobs. Returns the count removed.
func (c *Client) Prune(ctx context.Context) (int, error) {
	resp, err := c.rpc.Prune(ctx, &sgardpb.PruneRequest{})
	if err != nil {
		return 0, fmt.Errorf("prune: %w", err)
	}
	return int(resp.BlobsRemoved), nil
}

func writeAndVerify(g *garden.Garden, expectedHash string, data []byte) error {
	gotHash, err := g.WriteBlob(data)
	if err != nil {
		return fmt.Errorf("writing blob: %w", err)
	}
	if gotHash != expectedHash {
		return fmt.Errorf("blob hash mismatch: expected %s, got %s", expectedHash, gotHash)
	}
	return nil
}

// ErrServerNewer indicates the server's manifest is newer than the local one.
var ErrServerNewer = errors.New("server manifest is newer; pull first")
