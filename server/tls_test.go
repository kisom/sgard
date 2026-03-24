package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/manifest"
	"github.com/kisom/sgard/sgardpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// generateSelfSignedCert creates a self-signed TLS certificate for testing.
func generateSelfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sgard-test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("loading key pair: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)

	return cert, pool
}

// setupTLSTest creates a TLS-secured client-server pair.
func setupTLSTest(t *testing.T) (sgardpb.GardenSyncClient, *garden.Garden, *garden.Garden) {
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

	cert, caPool := generateSelfSignedCert(t)

	// Server with TLS.
	serverCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	srv := grpc.NewServer(grpc.Creds(serverCreds))
	sgardpb.RegisterGardenSyncServer(srv, New(serverGarden))
	t.Cleanup(func() { srv.Stop() })

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		_ = srv.Serve(lis)
	}()

	// Client with TLS, trusting the self-signed CA.
	clientCreds := credentials.NewTLS(&tls.Config{
		RootCAs:    caPool,
		MinVersion: tls.VersionTLS12,
	})
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(clientCreds),
	)
	if err != nil {
		t.Fatalf("dial TLS: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := sgardpb.NewGardenSyncClient(conn)
	return client, serverGarden, clientGarden
}

func TestTLS_PushPullCycle(t *testing.T) {
	client, serverGarden, _ := setupTLSTest(t)
	ctx := context.Background()

	// Write test blobs to get real hashes.
	tmpDir := t.TempDir()
	tmpGarden, err := garden.Init(tmpDir)
	if err != nil {
		t.Fatalf("init tmp garden: %v", err)
	}
	blobData := []byte("TLS test blob content")
	hash, err := tmpGarden.WriteBlob(blobData)
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}

	now := time.Now().UTC().Add(time.Hour)
	clientManifest := &manifest.Manifest{
		Version: 1,
		Created: now,
		Updated: now,
		Files: []manifest.Entry{
			{Path: "~/.tlstest", Hash: hash, Type: "file", Mode: "0644", Updated: now},
		},
	}

	// Push manifest over TLS.
	pushResp, err := client.PushManifest(ctx, &sgardpb.PushManifestRequest{
		Manifest: ManifestToProto(clientManifest),
	})
	if err != nil {
		t.Fatalf("PushManifest over TLS: %v", err)
	}
	if pushResp.Decision != sgardpb.PushManifestResponse_ACCEPTED {
		t.Fatalf("decision: got %v, want ACCEPTED", pushResp.Decision)
	}

	// Push blob over TLS.
	stream, err := client.PushBlobs(ctx)
	if err != nil {
		t.Fatalf("PushBlobs over TLS: %v", err)
	}
	if err := stream.Send(&sgardpb.PushBlobsRequest{
		Chunk: &sgardpb.BlobChunk{Hash: hash, Data: blobData},
	}); err != nil {
		t.Fatalf("Send blob: %v", err)
	}
	blobResp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if blobResp.BlobsReceived != 1 {
		t.Errorf("blobs_received: got %d, want 1", blobResp.BlobsReceived)
	}

	// Verify blob arrived on server.
	if !serverGarden.BlobExists(hash) {
		t.Error("blob not found on server after TLS push")
	}

	// Pull manifest back over TLS.
	pullResp, err := client.PullManifest(ctx, &sgardpb.PullManifestRequest{})
	if err != nil {
		t.Fatalf("PullManifest over TLS: %v", err)
	}
	pulledManifest := ProtoToManifest(pullResp.GetManifest())
	if len(pulledManifest.Files) != 1 {
		t.Fatalf("pulled manifest files: got %d, want 1", len(pulledManifest.Files))
	}
	if pulledManifest.Files[0].Path != "~/.tlstest" {
		t.Errorf("pulled path: got %q, want %q", pulledManifest.Files[0].Path, "~/.tlstest")
	}
}

func TestTLS_RejectsPlaintextClient(t *testing.T) {
	cert, _ := generateSelfSignedCert(t)

	serverDir := t.TempDir()
	serverGarden, err := garden.Init(serverDir)
	if err != nil {
		t.Fatalf("init server garden: %v", err)
	}

	serverCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	srv := grpc.NewServer(grpc.Creds(serverCreds))
	sgardpb.RegisterGardenSyncServer(srv, New(serverGarden))
	t.Cleanup(func() { srv.Stop() })

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		_ = srv.Serve(lis)
	}()

	// Try to connect without TLS — should fail.
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			// No RootCAs — won't trust the self-signed cert.
			MinVersion: tls.VersionTLS12,
		})),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := sgardpb.NewGardenSyncClient(conn)
	_, err = client.PullManifest(context.Background(), &sgardpb.PullManifestRequest{})
	if err == nil {
		t.Fatal("expected error when connecting without trusted CA to TLS server")
	}
}
