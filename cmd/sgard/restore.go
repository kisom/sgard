package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var forceRestore bool

var restoreCmd = &cobra.Command{
	Use:   "restore [path...]",
	Short: "Restore tracked files to their original locations",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		if g.HasEncryption() && g.NeedsDEK(g.List()) {
			if err := unlockDEK(g); err != nil {
				return err
			}
		}

		confirm := func(path string) bool {
			fmt.Printf("Overwrite %s? [y/N] ", path)
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				return answer == "y" || answer == "yes"
			}
			return false
		}

		if err := g.Restore(args, forceRestore, confirm); err != nil {
			return err
		}

		fmt.Println("Restore complete.")
		return nil
	},
}

func init() {
	restoreCmd.Flags().BoolVarP(&forceRestore, "force", "f", false, "overwrite without prompting")
	rootCmd.AddCommand(restoreCmd)
}
