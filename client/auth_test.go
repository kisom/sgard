package client

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestSSHCredentialsMetadata(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	creds := NewSSHCredentials(signer)

	md, err := creds.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetRequestMetadata: %v", err)
	}

	// Verify all required fields are present and non-empty.
	for _, key := range []string{
		"x-sgard-auth-nonce",
		"x-sgard-auth-timestamp",
		"x-sgard-auth-signature",
		"x-sgard-auth-pubkey",
	} {
		val, ok := md[key]
		if !ok || val == "" {
			t.Errorf("missing or empty metadata key %s", key)
		}
	}
}

func TestSSHCredentialsNoTransportSecurity(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	creds := NewSSHCredentials(signer)
	if creds.RequireTransportSecurity() {
		t.Error("RequireTransportSecurity should be false")
	}
}
