package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc/credentials"
)

// SSHCredentials implements grpc.PerRPCCredentials using an SSH signer.
type SSHCredentials struct {
	signer ssh.Signer
}

// NewSSHCredentials creates credentials from an SSH signer.
func NewSSHCredentials(signer ssh.Signer) *SSHCredentials {
	return &SSHCredentials{signer: signer}
}

// GetRequestMetadata signs a nonce+timestamp and returns auth metadata.
func (c *SSHCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	tsUnix := time.Now().Unix()
	payload := buildPayload(nonce, tsUnix)

	sig, err := c.signer.Sign(rand.Reader, payload)
	if err != nil {
		return nil, fmt.Errorf("signing payload: %w", err)
	}

	pubkey := c.signer.PublicKey()
	pubkeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubkey)))

	return map[string]string{
		"x-sgard-auth-nonce":     base64.StdEncoding.EncodeToString(nonce),
		"x-sgard-auth-timestamp": strconv.FormatInt(tsUnix, 10),
		"x-sgard-auth-signature": base64.StdEncoding.EncodeToString(ssh.Marshal(sig)),
		"x-sgard-auth-pubkey":    pubkeyStr,
	}, nil
}

// RequireTransportSecurity returns false — auth is via SSH signatures,
// not TLS. Transport security can be added separately.
func (c *SSHCredentials) RequireTransportSecurity() bool {
	return false
}

// Verify that SSHCredentials implements the interface.
var _ credentials.PerRPCCredentials = (*SSHCredentials)(nil)

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

	// Try ssh-agent.
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

	// Try default key paths.
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
