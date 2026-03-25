package main

import (
	"fmt"
	"strings"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Show detailed information about a tracked file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		fi, err := g.Info(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Path:       %s\n", fi.Path)
		fmt.Printf("Type:       %s\n", fi.Type)
		fmt.Printf("Status:     %s\n", fi.State)
		fmt.Printf("Mode:       %s\n", fi.Mode)

		if fi.Locked {
			fmt.Printf("Locked:     yes\n")
		}
		if fi.Encrypted {
			fmt.Printf("Encrypted:  yes\n")
		}

		if fi.Updated != "" {
			fmt.Printf("Updated:    %s\n", fi.Updated)
		}
		if fi.DiskModTime != "" {
			fmt.Printf("Disk mtime: %s\n", fi.DiskModTime)
		}

		switch fi.Type {
		case "file":
			fmt.Printf("Hash:       %s\n", fi.Hash)
			if fi.CurrentHash != "" && fi.CurrentHash != fi.Hash {
				fmt.Printf("Disk hash:  %s\n", fi.CurrentHash)
			}
			if fi.PlaintextHash != "" {
				fmt.Printf("PT hash:    %s\n", fi.PlaintextHash)
			}
			if fi.BlobStored {
				fmt.Printf("Blob:       stored\n")
			} else {
				fmt.Printf("Blob:       missing\n")
			}
		case "link":
			fmt.Printf("Target:     %s\n", fi.Target)
			if fi.CurrentTarget != "" && fi.CurrentTarget != fi.Target {
				fmt.Printf("Disk target: %s\n", fi.CurrentTarget)
			}
		}

		if len(fi.Only) > 0 {
			fmt.Printf("Only:       %s\n", strings.Join(fi.Only, ", "))
		}
		if len(fi.Never) > 0 {
			fmt.Printf("Never:      %s\n", strings.Join(fi.Never, ", "))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
