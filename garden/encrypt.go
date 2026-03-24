package garden

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/kisom/sgard/manifest"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	dekSize       = 32 // 256-bit DEK
	saltSize      = 16
	algorithmName = "xchacha20-poly1305"

	defaultArgon2Time    = 3
	defaultArgon2Memory  = 64 * 1024 // 64 MiB in KiB
	defaultArgon2Threads = 4
)

// EncryptInit sets up encryption on the repo by generating a DEK and
// wrapping it with a passphrase-derived KEK. The encryption config is
// stored in the manifest.
func (g *Garden) EncryptInit(passphrase string) error {
	if g.manifest.Encryption != nil {
		return fmt.Errorf("encryption already initialized")
	}

	// Generate DEK.
	dek := make([]byte, dekSize)
	if _, err := rand.Read(dek); err != nil {
		return fmt.Errorf("generating DEK: %w", err)
	}

	// Generate salt for passphrase KEK.
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	// Derive KEK from passphrase.
	kek := derivePassphraseKEK(passphrase, salt, defaultArgon2Time, defaultArgon2Memory, defaultArgon2Threads)

	// Wrap DEK.
	wrappedDEK, err := wrapDEK(dek, kek)
	if err != nil {
		return fmt.Errorf("wrapping DEK: %w", err)
	}

	g.manifest.Encryption = &manifest.Encryption{
		Algorithm: algorithmName,
		KekSlots: map[string]*manifest.KekSlot{
			"passphrase": {
				Type:          "passphrase",
				Argon2Time:    defaultArgon2Time,
				Argon2Memory:  defaultArgon2Memory,
				Argon2Threads: defaultArgon2Threads,
				Salt:          base64.StdEncoding.EncodeToString(salt),
				WrappedDEK:    base64.StdEncoding.EncodeToString(wrappedDEK),
			},
		},
	}

	g.dek = dek

	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// UnlockDEK attempts to unwrap the DEK using available KEK slots.
// Resolution order: try all fido2/* slots first (if a device is provided),
// then fall back to the passphrase slot. The DEK is cached on the Garden
// for the duration of the command.
func (g *Garden) UnlockDEK(promptPassphrase func() (string, error), fido2Device ...FIDO2Device) error {
	if g.dek != nil {
		return nil // already unlocked
	}

	enc := g.manifest.Encryption
	if enc == nil {
		return fmt.Errorf("encryption not initialized; run sgard encrypt init")
	}

	// 1. Try FIDO2 slots first.
	if len(fido2Device) > 0 && fido2Device[0] != nil {
		if g.unlockFIDO2(fido2Device[0]) {
			return nil
		}
	}

	// 2. Fall back to passphrase slot.
	if slot, ok := enc.KekSlots["passphrase"]; ok {
		if promptPassphrase == nil {
			return fmt.Errorf("passphrase required but no prompt available")
		}

		passphrase, err := promptPassphrase()
		if err != nil {
			return fmt.Errorf("reading passphrase: %w", err)
		}

		salt, err := base64.StdEncoding.DecodeString(slot.Salt)
		if err != nil {
			return fmt.Errorf("decoding salt: %w", err)
		}

		kek := derivePassphraseKEK(passphrase, salt, slot.Argon2Time, slot.Argon2Memory, slot.Argon2Threads)

		wrappedDEK, err := base64.StdEncoding.DecodeString(slot.WrappedDEK)
		if err != nil {
			return fmt.Errorf("decoding wrapped DEK: %w", err)
		}

		dek, err := unwrapDEK(wrappedDEK, kek)
		if err != nil {
			return fmt.Errorf("wrong passphrase or corrupted DEK: %w", err)
		}

		g.dek = dek
		return nil
	}

	return fmt.Errorf("no usable KEK slot found")
}

// HasEncryption reports whether the repo has encryption configured.
func (g *Garden) HasEncryption() bool {
	return g.manifest.Encryption != nil
}

// NeedsDEK reports whether any of the given entries are encrypted.
func (g *Garden) NeedsDEK(entries []manifest.Entry) bool {
	for _, e := range entries {
		if e.Encrypted {
			return true
		}
	}
	return false
}

// encryptBlob encrypts plaintext with the DEK and returns the ciphertext.
func (g *Garden) encryptBlob(plaintext []byte) ([]byte, error) {
	if g.dek == nil {
		return nil, fmt.Errorf("DEK not unlocked")
	}

	aead, err := chacha20poly1305.NewX(g.dek)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptBlob decrypts ciphertext with the DEK and returns the plaintext.
func (g *Garden) decryptBlob(ciphertext []byte) ([]byte, error) {
	if g.dek == nil {
		return nil, fmt.Errorf("DEK not unlocked")
	}

	aead, err := chacha20poly1305.NewX(g.dek)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	ct := ciphertext[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// plaintextHash computes the SHA-256 hash of plaintext data.
func plaintextHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// derivePassphraseKEK derives a KEK from a passphrase using Argon2id.
func derivePassphraseKEK(passphrase string, salt []byte, time, memory, threads int) []byte {
	return argon2.IDKey([]byte(passphrase), salt, uint32(time), uint32(memory), uint8(threads), dekSize)
}

// wrapDEK encrypts the DEK with the KEK using XChaCha20-Poly1305.
func wrapDEK(dek, kek []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return aead.Seal(nonce, nonce, dek, nil), nil
}

// unwrapDEK decrypts the DEK with the KEK.
func unwrapDEK(wrapped, kek []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	if len(wrapped) < nonceSize {
		return nil, fmt.Errorf("wrapped DEK too short")
	}

	nonce := wrapped[:nonceSize]
	ct := wrapped[nonceSize:]

	return aead.Open(nil, nonce, ct, nil)
}
