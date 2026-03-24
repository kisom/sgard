package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// Metadata keys for auth.
	metaNonce     = "x-sgard-auth-nonce"
	metaTimestamp = "x-sgard-auth-timestamp"
	metaSignature = "x-sgard-auth-signature"
	metaPubkey    = "x-sgard-auth-pubkey"

	// authWindow is how far the timestamp can deviate from server time.
	authWindow = 5 * time.Minute
)

// AuthInterceptor verifies SSH key signatures on gRPC requests.
type AuthInterceptor struct {
	authorizedKeys map[string]ssh.PublicKey // keyed by fingerprint
}

// NewAuthInterceptor creates an interceptor from an authorized_keys file.
// The file uses the same format as ~/.ssh/authorized_keys.
func NewAuthInterceptor(path string) (*AuthInterceptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading authorized keys: %w", err)
	}

	keys := make(map[string]ssh.PublicKey)
	rest := data
	for len(rest) > 0 {
		var key ssh.PublicKey
		key, _, _, rest, err = ssh.ParseAuthorizedKey(rest)
		if err != nil {
			break
		}
		fp := ssh.FingerprintSHA256(key)
		keys[fp] = key
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no valid keys found in %s", path)
	}

	return &AuthInterceptor{authorizedKeys: keys}, nil
}

// NewAuthInterceptorFromKeys creates an interceptor from pre-parsed keys.
// Intended for testing.
func NewAuthInterceptorFromKeys(keys []ssh.PublicKey) *AuthInterceptor {
	m := make(map[string]ssh.PublicKey, len(keys))
	for _, k := range keys {
		m[ssh.FingerprintSHA256(k)] = k
	}
	return &AuthInterceptor{authorizedKeys: m}
}

// UnaryInterceptor returns a gRPC unary server interceptor.
func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := a.verify(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor.
func (a *AuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := a.verify(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func (a *AuthInterceptor) verify(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	nonceB64 := mdFirst(md, metaNonce)
	tsStr := mdFirst(md, metaTimestamp)
	sigB64 := mdFirst(md, metaSignature)
	pubkeyStr := mdFirst(md, metaPubkey)

	if nonceB64 == "" || tsStr == "" || sigB64 == "" || pubkeyStr == "" {
		return status.Error(codes.Unauthenticated, "missing auth metadata fields")
	}

	// Parse timestamp and check window.
	tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid timestamp")
	}
	ts := time.Unix(tsUnix, 0)
	if time.Since(ts).Abs() > authWindow {
		return status.Error(codes.Unauthenticated, "timestamp outside allowed window")
	}

	// Parse public key and check authorization.
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubkeyStr))
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid public key")
	}
	fp := ssh.FingerprintSHA256(pubkey)
	authorized, ok := a.authorizedKeys[fp]
	if !ok {
		return status.Errorf(codes.PermissionDenied, "key %s not authorized", fp)
	}

	// Decode nonce and signature.
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid nonce encoding")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid signature encoding")
	}

	sig, err := parseSSHSignature(sigBytes)
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid signature format")
	}

	// Build the signed payload: nonce + timestamp bytes.
	payload := buildPayload(nonce, tsUnix)

	// Verify.
	if err := authorized.Verify(payload, sig); err != nil {
		return status.Error(codes.Unauthenticated, "signature verification failed")
	}

	return nil
}

// buildPayload constructs the message that is signed: nonce || timestamp (big-endian int64).
func buildPayload(nonce []byte, tsUnix int64) []byte {
	payload := make([]byte, len(nonce)+8)
	copy(payload, nonce)
	for i := 7; i >= 0; i-- {
		payload[len(nonce)+i] = byte(tsUnix & 0xff)
		tsUnix >>= 8
	}
	return payload
}

// GenerateNonce creates a 32-byte random nonce.
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	return nonce, nil
}

func mdFirst(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// parseSSHSignature deserializes an SSH signature from its wire format.
func parseSSHSignature(data []byte) (*ssh.Signature, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("signature too short")
	}

	// SSH signature wire format: string format, string blob
	formatLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if 4+formatLen > len(data) {
		return nil, fmt.Errorf("invalid format length")
	}
	format := string(data[4 : 4+formatLen])

	rest := data[4+formatLen:]
	if len(rest) < 4 {
		return nil, fmt.Errorf("missing blob length")
	}
	blobLen := int(rest[0])<<24 | int(rest[1])<<16 | int(rest[2])<<8 | int(rest[3])
	if 4+blobLen > len(rest) {
		return nil, fmt.Errorf("invalid blob length")
	}
	blob := rest[4 : 4+blobLen]

	_ = strings.TrimSpace(format) // ensure format is clean

	return &ssh.Signature{
		Format: format,
		Blob:   blob,
	}, nil
}
