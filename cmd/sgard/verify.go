package main

import (
	"errors"
	"fmt"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check all blobs against manifest hashes",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}

		results, err := g.Verify()
		if err != nil {
			return err
		}

		allOK := true
		for _, r := range results {
			fmt.Printf("%-14s %s\n", r.Detail, r.Path)
			if !r.OK {
				allOK = false
			}
		}

		if !allOK {
			return errors.New("verification failed: one or more blobs are corrupt or missing")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
