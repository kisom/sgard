package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var forceMirror bool

var mirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "Sync directory contents between filesystem and manifest",
}

var mirrorUpCmd = &cobra.Command{
	Use:   "up <path>...",
	Short: "Sync filesystem state into manifest (add new, remove deleted, rehash changed)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if err := g.MirrorUp(args); err != nil {
			return err
		}

		fmt.Println("Mirror up complete.")
		return nil
	},
}

var mirrorDownCmd = &cobra.Command{
	Use:   "down <path>...",
	Short: "Sync manifest state to filesystem (restore tracked, delete untracked)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		confirm := func(path string) bool {
			fmt.Printf("Delete untracked file %s? [y/N] ", path)
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				return answer == "y" || answer == "yes"
			}
			return false
		}

		if err := g.MirrorDown(args, forceMirror, confirm); err != nil {
			return err
		}

		fmt.Println("Mirror down complete.")
		return nil
	},
}

func init() {
	mirrorDownCmd.Flags().BoolVarP(&forceMirror, "force", "f", false, "delete untracked files without prompting")
	mirrorCmd.AddCommand(mirrorUpCmd, mirrorDownCmd)
	rootCmd.AddCommand(mirrorCmd)
}
