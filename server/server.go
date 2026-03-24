// Package server implements the GardenSync gRPC service.
package server

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/manifest"
	"github.com/kisom/sgard/sgardpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const chunkSize = 64 * 1024 // 64 KiB

// Server implements the sgardpb.GardenSyncServer interface.
type Server struct {
	sgardpb.UnimplementedGardenSyncServer
	garden          *garden.Garden
	mu              sync.RWMutex
	pendingManifest *manifest.Manifest
}

// New creates a new Server backed by the given Garden.
func New(g *garden.Garden) *Server {
	return &Server{garden: g}
}

// PushManifest compares the client manifest against the server manifest and
// decides whether to accept, reject, or report up-to-date.
func (s *Server) PushManifest(_ context.Context, req *sgardpb.PushManifestRequest) (*sgardpb.PushManifestResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	serverManifest := s.garden.GetManifest()
	clientManifest := ProtoToManifest(req.GetManifest())

	resp := &sgardpb.PushManifestResponse{
		ServerUpdated: timestamppb.New(serverManifest.Updated),
	}

	switch {
	case clientManifest.Updated.After(serverManifest.Updated):
		resp.Decision = sgardpb.PushManifestResponse_ACCEPTED

		var missing []string
		for _, e := range clientManifest.Files {
			if e.Type == "file" && e.Hash != "" && !s.garden.BlobExists(e.Hash) {
				missing = append(missing, e.Hash)
			}
		}
		resp.MissingBlobs = missing
		s.pendingManifest = clientManifest

	case serverManifest.Updated.After(clientManifest.Updated):
		resp.Decision = sgardpb.PushManifestResponse_REJECTED

	default:
		resp.Decision = sgardpb.PushManifestResponse_UP_TO_DATE
	}

	return resp, nil
}

// PushBlobs receives a stream of blob chunks, reassembles them, writes each
// blob to the store, and then applies the pending manifest.
func (s *Server) PushBlobs(stream grpc.ClientStreamingServer[sgardpb.PushBlobsRequest, sgardpb.PushBlobsResponse]) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		currentHash string
		buf         []byte
		blobCount   int32
	)

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "receiving blob chunk: %v", err)
		}

		chunk := req.GetChunk()
		if chunk == nil {
			continue
		}

		if chunk.GetHash() != "" {
			// New blob starting. Write out the previous one if any.
			if currentHash != "" {
				if err := s.writeAndVerify(currentHash, buf); err != nil {
					return err
				}
				blobCount++
			}
			currentHash = chunk.GetHash()
			buf = append([]byte(nil), chunk.GetData()...)
		} else {
			buf = append(buf, chunk.GetData()...)
		}
	}

	// Write the last accumulated blob.
	if currentHash != "" {
		if err := s.writeAndVerify(currentHash, buf); err != nil {
			return err
		}
		blobCount++
	}

	// Apply pending manifest.
	if s.pendingManifest != nil {
		if err := s.garden.ReplaceManifest(s.pendingManifest); err != nil {
			return status.Errorf(codes.Internal, "replacing manifest: %v", err)
		}
		s.pendingManifest = nil
	}

	return stream.SendAndClose(&sgardpb.PushBlobsResponse{
		BlobsReceived: blobCount,
	})
}

// writeAndVerify writes data to the blob store and verifies the hash matches.
func (s *Server) writeAndVerify(expectedHash string, data []byte) error {
	gotHash, err := s.garden.WriteBlob(data)
	if err != nil {
		return status.Errorf(codes.Internal, "writing blob: %v", err)
	}
	if gotHash != expectedHash {
		return status.Errorf(codes.DataLoss, "blob hash mismatch: expected %s, got %s", expectedHash, gotHash)
	}
	return nil
}

// PullManifest returns the server's current manifest.
func (s *Server) PullManifest(_ context.Context, _ *sgardpb.PullManifestRequest) (*sgardpb.PullManifestResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &sgardpb.PullManifestResponse{
		Manifest: ManifestToProto(s.garden.GetManifest()),
	}, nil
}

// PullBlobs streams the requested blobs back to the client in 64 KiB chunks.
func (s *Server) PullBlobs(req *sgardpb.PullBlobsRequest, stream grpc.ServerStreamingServer[sgardpb.PullBlobsResponse]) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, hash := range req.GetHashes() {
		data, err := s.garden.ReadBlob(hash)
		if err != nil {
			return status.Errorf(codes.NotFound, "reading blob %s: %v", hash, err)
		}

		for i := 0; i < len(data); i += chunkSize {
			end := i + chunkSize
			if end > len(data) {
				end = len(data)
			}
			chunk := &sgardpb.BlobChunk{
				Data: data[i:end],
			}
			if i == 0 {
				chunk.Hash = hash
			}
			if err := stream.Send(&sgardpb.PullBlobsResponse{Chunk: chunk}); err != nil {
				return status.Errorf(codes.Internal, "sending blob chunk: %v", err)
			}
		}

		// Handle empty blobs: send a single chunk with the hash.
		if len(data) == 0 {
			if err := stream.Send(&sgardpb.PullBlobsResponse{
				Chunk: &sgardpb.BlobChunk{Hash: hash},
			}); err != nil {
				return status.Errorf(codes.Internal, "sending empty blob chunk: %v", err)
			}
		}
	}

	return nil
}

// Prune removes orphaned blobs that are not referenced by the current manifest.
func (s *Server) Prune(_ context.Context, _ *sgardpb.PruneRequest) (*sgardpb.PruneResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Collect all referenced hashes from the manifest.
	referenced := make(map[string]bool)
	for _, e := range s.garden.GetManifest().Files {
		if e.Type == "file" && e.Hash != "" {
			referenced[e.Hash] = true
		}
	}

	// List all blobs in the store.
	allBlobs, err := s.garden.ListBlobs()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing blobs: %v", err)
	}

	// Delete orphans.
	var removed int32
	for _, hash := range allBlobs {
		if !referenced[hash] {
			if err := s.garden.DeleteBlob(hash); err != nil {
				return nil, status.Errorf(codes.Internal, "deleting blob %s: %v", hash, err)
			}
			removed++
		}
	}

	return &sgardpb.PruneResponse{BlobsRemoved: removed}, nil
}
