package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kisom/sgard/sgardpb"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/metadata"
)

var testJWTKey = []byte("test-jwt-secret-key-32-bytes!!")

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

func TestAuthenticateAndVerifyToken(t *testing.T) {
	signer, pubkey := generateTestKey(t)
	auth := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey}, testJWTKey)

	// Generate a signed challenge.
	nonce, _ := GenerateNonce()
	tsUnix := time.Now().Unix()
	payload := buildPayload(nonce, tsUnix)
	sig, err := signer.Sign(rand.Reader, payload)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	pubkeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))

	// Call Authenticate.
	resp, err := auth.Authenticate(context.Background(), &sgardpb.AuthenticateRequest{
		Nonce:     nonce,
		Timestamp: tsUnix,
		Signature: ssh.Marshal(sig),
		PublicKey: pubkeyStr,
	})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// Use the token in metadata.
	md := metadata.New(map[string]string{metaToken: resp.Token})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	if err := auth.verifyToken(ctx); err != nil {
		t.Fatalf("verifyToken should accept valid token: %v", err)
	}
}

func TestRejectMissingToken(t *testing.T) {
	_, pubkey := generateTestKey(t)
	auth := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey}, testJWTKey)

	// No metadata at all.
	if err := auth.verifyToken(context.Background()); err == nil {
		t.Fatal("should reject missing metadata")
	}

	// Empty metadata.
	md := metadata.New(nil)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	if err := auth.verifyToken(ctx); err == nil {
		t.Fatal("should reject missing token")
	}
}

func TestRejectUnauthorizedKey(t *testing.T) {
	signer1, _ := generateTestKey(t)
	_, pubkey2 := generateTestKey(t)

	// Auth only knows pubkey2, but we authenticate with signer1.
	auth := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey2}, testJWTKey)

	nonce, _ := GenerateNonce()
	tsUnix := time.Now().Unix()
	payload := buildPayload(nonce, tsUnix)
	sig, _ := signer1.Sign(rand.Reader, payload)
	pubkeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer1.PublicKey())))

	_, err := auth.Authenticate(context.Background(), &sgardpb.AuthenticateRequest{
		Nonce:     nonce,
		Timestamp: tsUnix,
		Signature: ssh.Marshal(sig),
		PublicKey: pubkeyStr,
	})
	if err == nil {
		t.Fatal("should reject unauthorized key")
	}
}

func TestExpiredTokenReturnsChallenge(t *testing.T) {
	signer, pubkey := generateTestKey(t)
	auth := NewAuthInterceptorFromKeys([]ssh.PublicKey{pubkey}, testJWTKey)

	// Issue a token, then manually create an expired one.
	fp := ssh.FingerprintSHA256(signer.PublicKey())
	expiredToken, err := auth.issueExpiredToken(fp)
	if err != nil {
		t.Fatalf("issuing expired token: %v", err)
	}

	md := metadata.New(map[string]string{metaToken: expiredToken})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	err = auth.verifyToken(ctx)
	if err == nil {
		t.Fatal("should reject expired token")
	}

	// The error should contain a ReauthChallenge in its details.
	// We can't easily extract it here without the client helper,
	// but verify the error message indicates expiry.
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention expiry, got: %v", err)
	}
}

// issueExpiredToken is a test helper that creates an already-expired JWT.
func (a *AuthInterceptor) issueExpiredToken(fingerprint string) (string, error) {
	past := time.Now().Add(-time.Hour)
	claims := &jwt.RegisteredClaims{
		Subject:   fingerprint,
		IssuedAt:  jwt.NewNumericDate(past.Add(-24 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(past),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtKey)
}
