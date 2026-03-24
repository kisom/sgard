package main

import (
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new sgard repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := garden.Init(repoFlag)
		if err != nil {
			return err
		}
		fmt.Printf("Initialized sgard repository at %s\n", repoFlag)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
