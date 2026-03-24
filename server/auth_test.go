package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/metadata"
)

func generateTestKey(t *testing.T) (ssh.Signer, ssh.PublicKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}
	return signer, signer.PublicKey()
}

func signedContext(t *testing.T, signer ssh.Signer) context.Context {
	t.Helper()

	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("generating nonce: %v", err)
	}
	tsUnix := time.Now().Unix()
	payload := buildPayload(nonce, tsUnix)

	sig, err := signer.Sign(rand.Reader, payload)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	pubkeyStr := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))

	md := metadata.New(map[string]string{
		metaNonce:     base64.StdEncoding.EncodeToString(nonce),
		metaTimestamp: strconv.FormatInt(tsUnix, 10),
		metaSignature: base64.StdEncoding.EncodeToString(ssh.Marshal(sig)),
		metaPubkey:    pubkeyStr,
	})
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestAuthVerifyValid(t *testing.T) {
	signer, pubkey := generateTestKey(t)
	interceptor := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey})

	ctx := signedContext(t, signer)
	if err := interceptor.verify(ctx); err != nil {
		t.Fatalf("verify should succeed: %v", err)
	}
}

func TestAuthRejectUnauthenticated(t *testing.T) {
	_, pubkey := generateTestKey(t)
	interceptor := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey})

	// No metadata at all.
	ctx := context.Background()
	if err := interceptor.verify(ctx); err == nil {
		t.Fatal("verify should reject missing metadata")
	}
}

func TestAuthRejectUnauthorizedKey(t *testing.T) {
	signer1, _ := generateTestKey(t)
	_, pubkey2 := generateTestKey(t)

	// Interceptor knows key2 but request is signed by key1.
	interceptor := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey2})

	ctx := signedContext(t, signer1)
	if err := interceptor.verify(ctx); err == nil {
		t.Fatal("verify should reject unauthorized key")
	}
}

func TestAuthRejectExpiredTimestamp(t *testing.T) {
	signer, pubkey := generateTestKey(t)
	interceptor := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey})

	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("generating nonce: %v", err)
	}
	// Timestamp 10 minutes ago — outside the 5-minute window.
	tsUnix := time.Now().Add(-10 * time.Minute).Unix()
	payload := buildPayload(nonce, tsUnix)

	sig, err := signer.Sign(rand.Reader, payload)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	pubkeyStr := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))

	md := metadata.New(map[string]string{
		metaNonce:     base64.StdEncoding.EncodeToString(nonce),
		metaTimestamp: strconv.FormatInt(tsUnix, 10),
		metaSignature: base64.StdEncoding.EncodeToString(ssh.Marshal(sig)),
		metaPubkey:    pubkeyStr,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	if err := interceptor.verify(ctx); err == nil {
		t.Fatal("verify should reject expired timestamp")
	}
}
