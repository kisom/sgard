//go:build fido2

package garden

import (
	"crypto/sha256"
	"fmt"

	libfido2 "github.com/keys-pub/go-libfido2"
)

const rpID = "sgard"

// HardwareFIDO2 implements FIDO2Device using a real hardware authenticator
// via libfido2.
type HardwareFIDO2 struct {
	pin string // device PIN (empty if no PIN set)
}

// NewHardwareFIDO2 creates a HardwareFIDO2 device. The PIN is needed for
// operations on PIN-protected authenticators.
func NewHardwareFIDO2(pin string) *HardwareFIDO2 {
	return &HardwareFIDO2{pin: pin}
}

// Available reports whether a FIDO2 device is connected.
func (h *HardwareFIDO2) Available() bool {
	locs, err := libfido2.DeviceLocations()
	if err != nil {
		return false
	}
	return len(locs) > 0
}

// Register creates a new credential with the hmac-secret extension.
// Returns the credential ID and the HMAC-secret output for the given salt.
func (h *HardwareFIDO2) Register(salt []byte) ([]byte, []byte, error) {
	dev, err := h.deviceForPath()
	if err != nil {
		return nil, nil, err
	}

	cdh := sha256.Sum256(salt)
	// CTAP2 hmac-secret extension requires a 32-byte salt.
	hmacSalt := fido2Salt(salt)

	userID := sha256.Sum256([]byte("sgard-user"))
	attest, err := dev.MakeCredential(
		cdh[:],
		libfido2.RelyingParty{ID: rpID, Name: "sgard"},
		libfido2.User{ID: userID[:], Name: "sgard"},
		libfido2.ES256,
		h.pin,
		&libfido2.MakeCredentialOpts{
			Extensions: []libfido2.Extension{libfido2.HMACSecretExtension},
			RK:         libfido2.False,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("fido2 make credential: %w", err)
	}

	// Do an assertion to get the HMAC-secret for this salt.
	assertion, err := dev.Assertion(
		rpID,
		cdh[:],
		[][]byte{attest.CredentialID},
		h.pin,
		&libfido2.AssertionOpts{
			Extensions: []libfido2.Extension{libfido2.HMACSecretExtension},
			HMACSalt:   hmacSalt,
			UP:         libfido2.True,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("fido2 assertion for hmac-secret: %w", err)
	}

	return attest.CredentialID, assertion.HMACSecret, nil
}

// Derive computes HMAC(device_secret, salt) for an existing credential.
// Requires user touch.
func (h *HardwareFIDO2) Derive(credentialID []byte, salt []byte) ([]byte, error) {
	dev, err := h.deviceForPath()
	if err != nil {
		return nil, err
	}

	cdh := sha256.Sum256(salt)
	hmacSalt := fido2Salt(salt)

	assertion, err := dev.Assertion(
		rpID,
		cdh[:],
		[][]byte{credentialID},
		h.pin,
		&libfido2.AssertionOpts{
			Extensions: []libfido2.Extension{libfido2.HMACSecretExtension},
			HMACSalt:   hmacSalt,
			UP:         libfido2.True,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("fido2 assertion: %w", err)
	}

	return assertion.HMACSecret, nil
}

// MatchesCredential reports whether the connected device might hold the
// given credential. Since probing without user presence is unreliable
// across devices, we optimistically return true and let Derive handle
// the actual verification (which requires a touch).
func (h *HardwareFIDO2) MatchesCredential(_ []byte) bool {
	return h.Available()
}

// fido2Salt returns a 32-byte salt suitable for the CTAP2 hmac-secret
// extension. If the input is already 32 bytes, it is returned as-is.
// Otherwise, SHA-256 is used to derive a 32-byte value deterministically.
func fido2Salt(salt []byte) []byte {
	if len(salt) == 32 {
		return salt
	}
	h := sha256.Sum256(salt)
	return h[:]
}

// deviceForPath returns a Device handle for the first connected FIDO2
// device. The library manages open/close internally per operation.
func (h *HardwareFIDO2) deviceForPath() (*libfido2.Device, error) {
	locs, err := libfido2.DeviceLocations()
	if err != nil {
		return nil, fmt.Errorf("listing fido2 devices: %w", err)
	}
	if len(locs) == 0 {
		return nil, fmt.Errorf("no fido2 device found")
	}

	dev, err := libfido2.NewDevice(locs[0].Path)
	if err != nil {
		return nil, fmt.Errorf("opening fido2 device %s: %w", locs[0].Path, err)
	}
	return dev, nil
}

// DetectHardwareFIDO2 returns a HardwareFIDO2 device if hardware is available,
// or nil if no device is connected.
func DetectHardwareFIDO2(pin string) FIDO2Device {
	d := NewHardwareFIDO2(pin)
	if d.Available() {
		return d
	}
	return nil
}
