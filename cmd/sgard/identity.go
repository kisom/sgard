package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Show this machine's identity labels",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}
		for _, label := range g.Identity() {
			fmt.Println(label)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(identityCmd)
}
