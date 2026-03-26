package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type remoteConfig struct {
	Addr  string `yaml:"addr"`
	TLS   bool   `yaml:"tls"`
	TLSCA string `yaml:"tls_ca,omitempty"`
}

func remoteConfigPath() string {
	return filepath.Join(repoFlag, "remote.yaml")
}

func loadRemoteConfig() (*remoteConfig, error) {
	data, err := os.ReadFile(remoteConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg remoteConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing remote config: %w", err)
	}
	return &cfg, nil
}

func saveRemoteConfig(cfg *remoteConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding remote config: %w", err)
	}
	return os.WriteFile(remoteConfigPath(), data, 0o644)
}

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage default remote server",
}

var remoteSetCmd = &cobra.Command{
	Use:   "set <addr>",
	Short: "Set the default remote address",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := &remoteConfig{
			Addr:  args[0],
			TLS:   tlsFlag,
			TLSCA: tlsCAFlag,
		}
		if err := saveRemoteConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Remote set: %s", cfg.Addr)
		if cfg.TLS {
			fmt.Print(" (TLS")
			if cfg.TLSCA != "" {
				fmt.Printf(", CA: %s", cfg.TLSCA)
			}
			fmt.Print(")")
		}
		fmt.Println()
		return nil
	},
}

var remoteShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the configured remote",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadRemoteConfig()
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No remote configured.")
				return nil
			}
			return err
		}
		fmt.Printf("addr:   %s\n", cfg.Addr)
		fmt.Printf("tls:    %v\n", cfg.TLS)
		if cfg.TLSCA != "" {
			fmt.Printf("tls-ca: %s\n", cfg.TLSCA)
		}
		return nil
	},
}

func init() {
	remoteCmd.AddCommand(remoteSetCmd, remoteShowCmd)
	rootCmd.AddCommand(remoteCmd)
}
