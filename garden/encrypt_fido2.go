package garden

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/kisom/sgard/manifest"
)

// FIDO2Device abstracts the hardware interaction with a FIDO2 authenticator.
// The real implementation requires libfido2 (CGo); tests use a mock.
type FIDO2Device interface {
	// Register creates a new credential with the hmac-secret extension.
	// Returns the credential ID and the HMAC-secret output for the given salt.
	Register(salt []byte) (credentialID []byte, hmacSecret []byte, err error)

	// Derive computes HMAC(device_secret, salt) for an existing credential.
	// Requires user touch.
	Derive(credentialID []byte, salt []byte) (hmacSecret []byte, err error)

	// Available reports whether a FIDO2 device is connected.
	Available() bool

	// MatchesCredential reports whether the connected device holds the
	// given credential (by ID). This allows skipping devices that can't
	// unwrap a particular slot without requiring a touch.
	MatchesCredential(credentialID []byte) bool
}

// AddFIDO2Slot adds a FIDO2 KEK slot to an encrypted repo. The DEK must
// already be unlocked (via passphrase or another FIDO2 slot). The label
// defaults to "fido2/<hostname>" but can be overridden.
func (g *Garden) AddFIDO2Slot(device FIDO2Device, label string) error {
	if g.dek == nil {
		return fmt.Errorf("DEK not unlocked; unlock via passphrase first")
	}
	if g.manifest.Encryption == nil {
		return fmt.Errorf("encryption not initialized")
	}
	if !device.Available() {
		return fmt.Errorf("no FIDO2 device connected")
	}

	// Normalize label.
	if label == "" {
		label = defaultFIDO2Label()
	}
	if !strings.HasPrefix(label, "fido2/") {
		label = "fido2/" + label
	}

	if _, exists := g.manifest.Encryption.KekSlots[label]; exists {
		return fmt.Errorf("slot %q already exists", label)
	}

	// Generate salt for this FIDO2 credential.
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	// Register credential and get HMAC-secret (the KEK).
	credID, kek, err := device.Register(salt)
	if err != nil {
		return fmt.Errorf("FIDO2 registration: %w", err)
	}

	if len(kek) < dekSize {
		return fmt.Errorf("FIDO2 HMAC-secret too short: got %d bytes, need %d", len(kek), dekSize)
	}
	kek = kek[:dekSize]

	// Wrap DEK with the FIDO2-derived KEK.
	wrappedDEK, err := wrapDEK(g.dek, kek)
	if err != nil {
		return fmt.Errorf("wrapping DEK: %w", err)
	}

	g.manifest.Encryption.KekSlots[label] = &manifest.KekSlot{
		Type:         "fido2",
		CredentialID: base64.StdEncoding.EncodeToString(credID),
		Salt:         base64.StdEncoding.EncodeToString(salt),
		WrappedDEK:   base64.StdEncoding.EncodeToString(wrappedDEK),
	}

	if err := g.manifest.Save(g.manifestPath); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// unlockFIDO2 attempts to unlock the DEK using any available fido2/* slot.
// Returns true if successful.
func (g *Garden) unlockFIDO2(device FIDO2Device) bool {
	if device == nil || !device.Available() {
		return false
	}

	enc := g.manifest.Encryption
	for name, slot := range enc.KekSlots {
		if slot.Type != "fido2" || !strings.HasPrefix(name, "fido2/") {
			continue
		}

		credID, err := base64.StdEncoding.DecodeString(slot.CredentialID)
		if err != nil {
			continue
		}

		// Check if the connected device holds this credential.
		if !device.MatchesCredential(credID) {
			continue
		}

		salt, err := base64.StdEncoding.DecodeString(slot.Salt)
		if err != nil {
			continue
		}

		kek, err := device.Derive(credID, salt)
		if err != nil {
			continue
		}
		if len(kek) < dekSize {
			continue
		}
		kek = kek[:dekSize]

		wrappedDEK, err := base64.StdEncoding.DecodeString(slot.WrappedDEK)
		if err != nil {
			continue
		}

		dek, err := unwrapDEK(wrappedDEK, kek)
		if err != nil {
			continue
		}

		g.dek = dek
		return true
	}

	return false
}

// defaultFIDO2Label returns "<hostname>" as the default FIDO2 slot label.
func defaultFIDO2Label() string {
	host, err := os.Hostname()
	if err != nil {
		return "fido2/device"
	}
	// Use short hostname (before first dot).
	if idx := strings.IndexByte(host, '.'); idx >= 0 {
		host = host[:idx]
	}
	return "fido2/" + host
}
