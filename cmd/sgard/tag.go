package main

import (
	"fmt"
	"sort"

	"github.com/kisom/sgard/garden"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage machine tags for per-machine targeting",
}

var tagAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a tag to this machine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}
		if err := g.SaveTag(args[0]); err != nil {
			return err
		}
		fmt.Printf("Tag %q added.\n", args[0])
		return nil
	},
}

var tagRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a tag from this machine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}
		if err := g.RemoveTag(args[0]); err != nil {
			return err
		}
		fmt.Printf("Tag %q removed.\n", args[0])
		return nil
	},
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags on this machine",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := garden.Open(repoFlag)
		if err != nil {
			return err
		}
		tags := g.LoadTags()
		if len(tags) == 0 {
			fmt.Println("No tags set.")
			return nil
		}
		sort.Strings(tags)
		for _, tag := range tags {
			fmt.Println(tag)
		}
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRemoveCmd)
	tagCmd.AddCommand(tagListCmd)
	rootCmd.AddCommand(tagCmd)
}
