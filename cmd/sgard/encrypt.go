package main

import (
	"fmt"
	"sort"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Manage encryption keys and slots",
}

var fido2InitFlag bool

var encryptInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize encryption (creates DEK and passphrase slot)",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		passphrase, err := promptPassphrase()
		if err != nil {
			return err
		}

		if err := g.EncryptInit(passphrase); err != nil {
			return err
		}

		fmt.Println("Encryption initialized with passphrase slot.")

		if fido2InitFlag {
			fmt.Println("FIDO2 support requires a hardware device implementation.")
			fmt.Println("Run 'sgard encrypt add-fido2' when a FIDO2 device is available.")
		}

		return nil
	},
}

var fido2LabelFlag string

var addFido2Cmd = &cobra.Command{
	Use:   "add-fido2",
	Short: "Add a FIDO2 KEK slot",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if !g.HasEncryption() {
			return fmt.Errorf("encryption not initialized; run sgard encrypt init first")
		}

		if err := g.UnlockDEK(promptPassphrase); err != nil {
			return err
		}

		// Real FIDO2 device implementation would go here.
		// For now, this is a placeholder that explains the requirement.
		return fmt.Errorf("FIDO2 hardware support not yet implemented; requires libfido2 binding")
	},
}

var removeSlotCmd = &cobra.Command{
	Use:   "remove-slot <name>",
	Short: "Remove a KEK slot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.RemoveSlot(args[0]); err != nil {
			return err
		}

		fmt.Printf("Removed slot %q.\n", args[0])
		return nil
	},
}

var listSlotsCmd = &cobra.Command{
	Use:   "list-slots",
	Short: "List all KEK slots",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		slots := g.ListSlots()
		if len(slots) == 0 {
			fmt.Println("No encryption configured.")
			return nil
		}

		// Sort for consistent output.
		names := make([]string, 0, len(slots))
		for name := range slots {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			fmt.Printf("%-30s %s\n", name, slots[name])
		}
		return nil
	},
}

var changePassphraseCmd = &cobra.Command{
	Use:   "change-passphrase",
	Short: "Change the passphrase for the passphrase KEK slot",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if !g.HasEncryption() {
			return fmt.Errorf("encryption not initialized")
		}

		// Unlock with current passphrase.
		fmt.Println("Enter current passphrase:")
		if err := g.UnlockDEK(promptPassphrase); err != nil {
			return err
		}

		// Get new passphrase.
		fmt.Println("Enter new passphrase:")
		newPassphrase, err := promptPassphrase()
		if err != nil {
			return err
		}

		if err := g.ChangePassphrase(newPassphrase); err != nil {
			return err
		}

		fmt.Println("Passphrase changed.")
		return nil
	},
}

var rotateDEKCmd = &cobra.Command{
	Use:   "rotate-dek",
	Short: "Generate a new DEK and re-encrypt all encrypted blobs",
	Long:  "Generates a new data encryption key, re-encrypts all encrypted blobs, and re-wraps the DEK with all KEK slots. Required when the DEK is suspected compromised.",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if !g.HasEncryption() {
			return fmt.Errorf("encryption not initialized")
		}

		// Unlock with current passphrase.
		fmt.Println("Enter passphrase to unlock:")
		if err := g.UnlockDEK(promptPassphrase); err != nil {
			return err
		}

		// Rotate — re-prompts for passphrase to re-wrap slot.
		fmt.Println("Enter passphrase to re-wrap DEK:")
		if err := g.RotateDEK(promptPassphrase); err != nil {
			return err
		}

		fmt.Println("DEK rotated. All encrypted blobs re-encrypted.")
		return nil
	},
}

func init() {
	encryptInitCmd.Flags().BoolVar(&fido2InitFlag, "fido2", false, "also set up FIDO2 (placeholder)")
	addFido2Cmd.Flags().StringVar(&fido2LabelFlag, "label", "", "slot label (default: fido2/<hostname>)")

	encryptCmd.AddCommand(encryptInitCmd)
	encryptCmd.AddCommand(addFido2Cmd)
	encryptCmd.AddCommand(removeSlotCmd)
	encryptCmd.AddCommand(listSlotsCmd)
	encryptCmd.AddCommand(changePassphraseCmd)
	encryptCmd.AddCommand(rotateDEKCmd)

	rootCmd.AddCommand(encryptCmd)
}
