package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kisom/sgard/sgardpb"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	metaToken  = "x-sgard-auth-token"
	authWindow = 5 * time.Minute
	tokenTTL   = 30 * 24 * time.Hour // 30 days
)

// AuthInterceptor verifies JWT tokens or SSH key signatures on gRPC requests.
type AuthInterceptor struct {
	authorizedKeys map[string]ssh.PublicKey // keyed by fingerprint
	jwtKey         []byte                  // HMAC-SHA256 signing key
}

// NewAuthInterceptor creates an interceptor from an authorized_keys file
// and a repository path (for the JWT secret key).
func NewAuthInterceptor(authorizedKeysPath, repoPath string) (*AuthInterceptor, error) {
	data, err := os.ReadFile(authorizedKeysPath)
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
		return nil, fmt.Errorf("no valid keys found in %s", authorizedKeysPath)
	}

	jwtKey, err := loadOrGenerateJWTKey(repoPath)
	if err != nil {
		return nil, fmt.Errorf("loading JWT key: %w", err)
	}

	return &AuthInterceptor{authorizedKeys: keys, jwtKey: jwtKey}, nil
}

// NewAuthInterceptorFromKeys creates an interceptor from pre-parsed keys
// and a provided JWT key. Intended for testing.
func NewAuthInterceptorFromKeys(keys []ssh.PublicKey, jwtKey []byte) *AuthInterceptor {
	m := make(map[string]ssh.PublicKey, len(keys))
	for _, k := range keys {
		m[ssh.FingerprintSHA256(k)] = k
	}
	return &AuthInterceptor{authorizedKeys: m, jwtKey: jwtKey}
}

// UnaryInterceptor returns a gRPC unary server interceptor.
func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Authenticate RPC is exempt from auth — it's how you get a token.
		if strings.HasSuffix(info.FullMethod, "/Authenticate") {
			return handler(ctx, req)
		}
		if err := a.verifyToken(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor.
func (a *AuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := a.verifyToken(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// Authenticate verifies an SSH-signed challenge and issues a JWT.
func (a *AuthInterceptor) Authenticate(_ context.Context, req *sgardpb.AuthenticateRequest) (*sgardpb.AuthenticateResponse, error) {
	pubkeyStr := req.GetPublicKey()
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubkeyStr))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid public key")
	}

	fp := ssh.FingerprintSHA256(pubkey)
	authorized, ok := a.authorizedKeys[fp]
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "key %s not authorized", fp)
	}

	// Verify timestamp window.
	tsUnix := req.GetTimestamp()
	ts := time.Unix(tsUnix, 0)
	if time.Since(ts).Abs() > authWindow {
		return nil, status.Error(codes.Unauthenticated, "timestamp outside allowed window")
	}

	// Verify signature.
	payload := buildPayload(req.GetNonce(), tsUnix)
	sig, err := parseSSHSignature(req.GetSignature())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid signature format")
	}
	if err := authorized.Verify(payload, sig); err != nil {
		return nil, status.Error(codes.Unauthenticated, "signature verification failed")
	}

	// Issue JWT.
	token, err := a.issueToken(fp)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "issuing token: %v", err)
	}

	return &sgardpb.AuthenticateResponse{Token: token}, nil
}

func (a *AuthInterceptor) verifyToken(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokenStr := mdFirst(md, metaToken)
	if tokenStr == "" {
		return status.Error(codes.Unauthenticated, "missing auth token")
	}

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.jwtKey, nil
	})

	if err != nil || !token.Valid {
		// Check if the token is expired but otherwise valid.
		if a.isExpiredButValid(tokenStr, claims) {
			return a.reauthError()
		}
		return status.Error(codes.Unauthenticated, "invalid token")
	}

	// Verify the fingerprint is still authorized.
	fp := claims.Subject
	if _, ok := a.authorizedKeys[fp]; !ok {
		return status.Errorf(codes.PermissionDenied, "key %s no longer authorized", fp)
	}

	return nil
}

// isExpiredButValid checks if a token has a valid signature and the
// fingerprint is still in authorized_keys, but the token is expired.
func (a *AuthInterceptor) isExpiredButValid(tokenStr string, claims *jwt.RegisteredClaims) bool {
	// Re-parse without time validation.
	reClaims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, reClaims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return a.jwtKey, nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return false
	}

	fp := reClaims.Subject
	_, authorized := a.authorizedKeys[fp]
	return authorized
}

// reauthError returns an Unauthenticated error with a ReauthChallenge
// embedded in the error details.
func (a *AuthInterceptor) reauthError() error {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return status.Error(codes.Internal, "generating reauth nonce")
	}

	challenge := &sgardpb.ReauthChallenge{
		Nonce:     nonce,
		Timestamp: time.Now().Unix(),
	}

	st, err := status.New(codes.Unauthenticated, "token expired").
		WithDetails(challenge)
	if err != nil {
		return status.Error(codes.Unauthenticated, "token expired")
	}
	return st.Err()
}

func (a *AuthInterceptor) issueToken(fingerprint string) (string, error) {
	now := time.Now()
	claims := &jwt.RegisteredClaims{
		Subject:   fingerprint,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtKey)
}

func loadOrGenerateJWTKey(repoPath string) ([]byte, error) {
	keyPath := filepath.Join(repoPath, "jwt.key")

	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) >= 32 {
		return data[:32], nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating JWT key: %w", err)
	}

	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("writing JWT key: %w", err)
	}

	return key, nil
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

	return &ssh.Signature{
		Format: format,
		Blob:   blob,
	}, nil
}
