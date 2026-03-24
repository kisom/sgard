package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var encryptFlag bool

var addCmd = &cobra.Command{
	Use:   "add <path>...",
	Short: "Track files, directories, or symlinks",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if encryptFlag {
			if !g.HasEncryption() {
				return fmt.Errorf("encryption not initialized; run sgard encrypt init first")
			}
			if err := g.UnlockDEK(promptPassphrase); err != nil {
				return err
			}
		}

		if err := g.Add(args, encryptFlag); err != nil {
			return err
		}

		fmt.Printf("Added %d path(s)\n", len(args))
		return nil
	},
}

func promptPassphrase() (string, error) {
	fmt.Fprint(os.Stderr, "Passphrase: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	return "", fmt.Errorf("no passphrase provided")
}

func init() {
	addCmd.Flags().BoolVar(&encryptFlag, "encrypt", false, "encrypt file contents before storing")
	rootCmd.AddCommand(addCmd)
}
