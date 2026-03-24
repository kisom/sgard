//go:build !fido2

package garden

// DetectHardwareFIDO2 is a stub that returns nil when built without the
// fido2 build tag. Build with -tags fido2 and link against libfido2 to
// enable real hardware support.
func DetectHardwareFIDO2(_ string) FIDO2Device {
	return nil
}
