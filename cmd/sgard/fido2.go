package main

import "github.com/kisom/sgard/garden"

var fido2PinFlag string

// unlockDEK attempts to unlock the DEK, trying FIDO2 hardware first
// (if available) and falling back to passphrase.
func unlockDEK(g *garden.Garden) error {
	device := garden.DetectHardwareFIDO2(fido2PinFlag)
	return g.UnlockDEK(promptPassphrase, device)
}
