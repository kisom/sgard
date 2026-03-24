package integration

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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kisom/sgard/client"
	"github.com/kisom/sgard/garden"
	"github.com/kisom/sgard/server"
	"github.com/kisom/sgard/sgardpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func generateSelfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sgard-e2e"},
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

// TestE2E_Phase4 exercises TLS + encryption + locked files in a push/pull cycle.
func TestE2E_Phase4(t *testing.T) {
	// --- Setup TLS server ---
	cert, caPool := generateSelfSignedCert(t)

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
	sgardpb.RegisterGardenSyncServer(srv, server.New(serverGarden))
	t.Cleanup(func() { srv.Stop() })

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()

	clientCreds := credentials.NewTLS(&tls.Config{
		RootCAs:    caPool,
		MinVersion: tls.VersionTLS12,
	})

	// --- Build source garden with encryption + locked files ---
	srcRoot := t.TempDir()
	srcRepoDir := filepath.Join(srcRoot, "repo")
	srcGarden, err := garden.Init(srcRepoDir)
	if err != nil {
		t.Fatalf("init source garden: %v", err)
	}

	if err := srcGarden.EncryptInit("test-passphrase"); err != nil {
		t.Fatalf("EncryptInit: %v", err)
	}

	plainFile := filepath.Join(srcRoot, "plain")
	secretFile := filepath.Join(srcRoot, "secret")
	lockedFile := filepath.Join(srcRoot, "locked")
	encLockedFile := filepath.Join(srcRoot, "enc-locked")

	if err := os.WriteFile(plainFile, []byte("plain data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("secret data"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(lockedFile, []byte("locked data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(encLockedFile, []byte("enc+locked data"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := srcGarden.Add([]string{plainFile}); err != nil {
		t.Fatalf("Add plain: %v", err)
	}
	if err := srcGarden.Add([]string{secretFile}, garden.AddOptions{Encrypt: true}); err != nil {
		t.Fatalf("Add encrypted: %v", err)
	}
	if err := srcGarden.Add([]string{lockedFile}, garden.AddOptions{Lock: true}); err != nil {
		t.Fatalf("Add locked: %v", err)
	}
	if err := srcGarden.Add([]string{encLockedFile}, garden.AddOptions{Encrypt: true, Lock: true}); err != nil {
		t.Fatalf("Add encrypted+locked: %v", err)
	}

	// Bump timestamp so push wins.
	srcManifest := srcGarden.GetManifest()
	srcManifest.Updated = time.Now().UTC().Add(time.Hour)
	if err := srcGarden.ReplaceManifest(srcManifest); err != nil {
		t.Fatalf("ReplaceManifest: %v", err)
	}

	// --- Push over TLS ---
	ctx := context.Background()

	pushConn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(clientCreds),
	)
	if err != nil {
		t.Fatalf("dial for push: %v", err)
	}
	defer func() { _ = pushConn.Close() }()

	pushClient := client.New(pushConn)
	pushed, err := pushClient.Push(ctx, srcGarden)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if pushed < 2 {
		t.Errorf("expected at least 2 blobs pushed, got %d", pushed)
	}

	// --- Pull to a fresh garden over TLS ---
	dstRoot := t.TempDir()
	dstRepoDir := filepath.Join(dstRoot, "repo")
	dstGarden, err := garden.Init(dstRepoDir)
	if err != nil {
		t.Fatalf("init dest garden: %v", err)
	}

	pullConn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(clientCreds),
	)
	if err != nil {
		t.Fatalf("dial for pull: %v", err)
	}
	defer func() { _ = pullConn.Close() }()

	pullClient := client.New(pullConn)
	pulled, err := pullClient.Pull(ctx, dstGarden)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if pulled < 2 {
		t.Errorf("expected at least 2 blobs pulled, got %d", pulled)
	}

	// --- Verify the pulled manifest ---
	dstManifest := dstGarden.GetManifest()
	if len(dstManifest.Files) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(dstManifest.Files))
	}

	type entryInfo struct {
		encrypted bool
		locked    bool
	}
	entryMap := make(map[string]entryInfo)
	for _, e := range dstManifest.Files {
		entryMap[e.Path] = entryInfo{e.Encrypted, e.Locked}
	}

	// Verify flags survived round trip.
	for path, info := range entryMap {
		switch {
		case path == toTilde(secretFile):
			if !info.encrypted {
				t.Errorf("%s should be encrypted", path)
			}
		case path == toTilde(lockedFile):
			if !info.locked {
				t.Errorf("%s should be locked", path)
			}
		case path == toTilde(encLockedFile):
			if !info.encrypted || !info.locked {
				t.Errorf("%s should be encrypted+locked", path)
			}
		case path == toTilde(plainFile):
			if info.encrypted || info.locked {
				t.Errorf("%s should be plain", path)
			}
		}
	}

	// Verify encryption config survived.
	if dstManifest.Encryption == nil {
		t.Fatal("encryption config should survive push/pull")
	}
	if dstManifest.Encryption.Algorithm != "xchacha20-poly1305" {
		t.Errorf("algorithm = %s, want xchacha20-poly1305", dstManifest.Encryption.Algorithm)
	}
	if _, ok := dstManifest.Encryption.KekSlots["passphrase"]; !ok {
		t.Error("passphrase slot should survive push/pull")
	}

	// Verify all blobs arrived.
	for _, e := range dstManifest.Files {
		if e.Hash != "" && !dstGarden.BlobExists(e.Hash) {
			t.Errorf("blob missing for %s (hash %s)", e.Path, e.Hash)
		}
	}

	// Unlock on dest and verify DEK works.
	if err := dstGarden.UnlockDEK(func() (string, error) { return "test-passphrase", nil }); err != nil {
		t.Fatalf("UnlockDEK on dest: %v", err)
	}
}

func toTilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || len(rel) > 0 && rel[0] == '.' {
		return path
	}
	return "~/" + rel
}
