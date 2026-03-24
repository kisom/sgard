package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kisom/sgard/sgardpb"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// TokenCredentials implements grpc.PerRPCCredentials using a cached JWT token.
// It is safe for concurrent use.
type TokenCredentials struct {
	mu    sync.RWMutex
	token string
}

// NewTokenCredentials creates credentials with an initial token (may be empty).
func NewTokenCredentials(token string) *TokenCredentials {
	return &TokenCredentials{token: token}
}

// SetToken updates the cached token.
func (c *TokenCredentials) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

// GetRequestMetadata returns the token as gRPC metadata.
func (c *TokenCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.token == "" {
		return nil, nil
	}
	return map[string]string{"x-sgard-auth-token": c.token}, nil
}

// RequireTransportSecurity returns false.
func (c *TokenCredentials) RequireTransportSecurity() bool {
	return false
}

var _ credentials.PerRPCCredentials = (*TokenCredentials)(nil)

// TokenPath returns the XDG-compliant path for the token cache.
// Uses $XDG_STATE_HOME/sgard/token, falling back to ~/.local/state/sgard/token.
func TokenPath() (string, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "sgard", "token"), nil
}

// LoadCachedToken reads the token from the XDG state path.
// Returns empty string if the file doesn't exist.
func LoadCachedToken() string {
	path, err := TokenPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SaveToken writes the token to the XDG state path with 0600 permissions.
func SaveToken(token string) error {
	path, err := TokenPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

// Authenticate calls the server's Authenticate RPC with an SSH-signed challenge.
// If challenge is non-nil (reauth fast path), uses the server-provided nonce.
// Otherwise generates a fresh nonce.
func Authenticate(ctx context.Context, rpc sgardpb.GardenSyncClient, signer ssh.Signer, challenge *sgardpb.ReauthChallenge) (string, error) {
	var nonce []byte
	var tsUnix int64

	if challenge != nil {
		nonce = challenge.GetNonce()
		tsUnix = challenge.GetTimestamp()
	} else {
		var err error
		nonce = make([]byte, 32)
		if _, err = rand.Read(nonce); err != nil {
			return "", fmt.Errorf("generating nonce: %w", err)
		}
		tsUnix = time.Now().Unix()
	}

	payload := buildPayload(nonce, tsUnix)
	sig, err := signer.Sign(rand.Reader, payload)
	if err != nil {
		return "", fmt.Errorf("signing challenge: %w", err)
	}

	pubkeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))

	resp, err := rpc.Authenticate(ctx, &sgardpb.AuthenticateRequest{
		Nonce:     nonce,
		Timestamp: tsUnix,
		Signature: ssh.Marshal(sig),
		PublicKey: pubkeyStr,
	})
	if err != nil {
		return "", fmt.Errorf("authenticate RPC: %w", err)
	}

	return resp.GetToken(), nil
}

// ExtractReauthChallenge extracts a ReauthChallenge from a gRPC error's
// details, if present. Returns nil if not found.
func ExtractReauthChallenge(err error) *sgardpb.ReauthChallenge {
	st, ok := status.FromError(err)
	if !ok {
		return nil
	}
	for _, detail := range st.Details() {
		if challenge, ok := detail.(*sgardpb.ReauthChallenge); ok {
			return challenge
		}
	}
	return nil
}

// buildPayload constructs nonce || timestamp (big-endian int64).
func buildPayload(nonce []byte, tsUnix int64) []byte {
	payload := make([]byte, len(nonce)+8)
	copy(payload, nonce)
	for i := 7; i >= 0; i-- {
		payload[len(nonce)+i] = byte(tsUnix & 0xff)
		tsUnix >>= 8
	}
	return payload
}

// LoadSigner loads an SSH signer. Resolution order:
// 1. keyPath (if non-empty)
// 2. SSH agent (if SSH_AUTH_SOCK is set)
// 3. Default key paths: ~/.ssh/id_ed25519, ~/.ssh/id_rsa
func LoadSigner(keyPath string) (ssh.Signer, error) {
	if keyPath != "" {
		return loadSignerFromFile(keyPath)
	}

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			ag := agent.NewClient(conn)
			signers, err := ag.Signers()
			if err == nil && len(signers) > 0 {
				return signers[0], nil
			}
			_ = conn.Close()
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("no SSH key found: %w", err)
	}

	for _, name := range []string{"id_ed25519", "id_rsa"} {
		path := home + "/.ssh/" + name
		signer, err := loadSignerFromFile(path)
		if err == nil {
			return signer, nil
		}
	}

	return nil, fmt.Errorf("no SSH key found (tried --ssh-key, agent, ~/.ssh/id_ed25519, ~/.ssh/id_rsa)")
}

func loadSignerFromFile(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading key %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parsing key %s: %w", path, err)
	}
	return signer, nil
}

// SSHCredentials is kept for backward compatibility in tests.
// It signs every request with SSH (the old approach).
type SSHCredentials struct {
	signer ssh.Signer
}

func NewSSHCredentials(signer ssh.Signer) *SSHCredentials {
	return &SSHCredentials{signer: signer}
}

func (c *SSHCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	tsUnix := time.Now().Unix()
	payload := buildPayload(nonce, tsUnix)
	sig, err := c.signer.Sign(rand.Reader, payload)
	if err != nil {
		return nil, fmt.Errorf("signing: %w", err)
	}
	pubkeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(c.signer.PublicKey())))

	// Send as both token-style metadata (won't work) AND the old SSH fields
	// for the Authenticate RPC. But this is only used in legacy tests.
	return map[string]string{
		"x-sgard-auth-nonce":     base64.StdEncoding.EncodeToString(nonce),
		"x-sgard-auth-timestamp": fmt.Sprintf("%d", tsUnix),
		"x-sgard-auth-signature": base64.StdEncoding.EncodeToString(ssh.Marshal(sig)),
		"x-sgard-auth-pubkey":    pubkeyStr,
	}, nil
}

func (c *SSHCredentials) RequireTransportSecurity() bool { return false }

var _ credentials.PerRPCCredentials = (*SSHCredentials)(nil)
