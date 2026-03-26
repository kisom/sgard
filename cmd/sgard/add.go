package main

import (
	"fmt"
	"os"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	encryptFlag bool
	lockFlag    bool
	dirOnlyFlag bool
	onlyFlag    []string
	neverFlag   []string
)

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
			if err := unlockDEK(g); err != nil {
				return err
			}
		}

		if len(onlyFlag) > 0 && len(neverFlag) > 0 {
			return fmt.Errorf("--only and --never are mutually exclusive")
		}

		opts := garden.AddOptions{
			Encrypt: encryptFlag,
			Lock:    lockFlag,
			DirOnly: dirOnlyFlag,
			Only:    onlyFlag,
			Never:   neverFlag,
		}

		if err := g.Add(args, opts); err != nil {
			return err
		}

		fmt.Printf("Added %d path(s)\n", len(args))
		return nil
	},
}

func promptPassphrase() (string, error) {
	fmt.Fprint(os.Stderr, "Passphrase: ")
	fd := int(os.Stdin.Fd())
	passphrase, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	if len(passphrase) == 0 {
		return "", fmt.Errorf("no passphrase provided")
	}
	return string(passphrase), nil
}

func init() {
	addCmd.Flags().BoolVar(&encryptFlag, "encrypt", false, "encrypt file contents before storing")
	addCmd.Flags().BoolVar(&lockFlag, "lock", false, "mark as locked (repo-authoritative, restore always overwrites)")
	addCmd.Flags().BoolVar(&dirOnlyFlag, "dir", false, "track directory itself without recursing into contents")
	addCmd.Flags().StringSliceVar(&onlyFlag, "only", nil, "only apply on machines matching these labels (comma-separated)")
	addCmd.Flags().StringSliceVar(&neverFlag, "never", nil, "never apply on machines matching these labels (comma-separated)")
	rootCmd.AddCommand(addCmd)
}
