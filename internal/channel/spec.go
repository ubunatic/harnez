// Package channel implements the opt-in Claude MCP channel canary (issue 740).
package channel

import (
	"bytes"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Config contains defaults from the embedded channel spec, without Go fallbacks.
type Config struct {
	RegistryDir           string `yaml:"registry_dir"`
	ListenAddress         string `yaml:"listen_address"`
	ServerName            string `yaml:"server_name"`
	PluginName            string `yaml:"plugin_name"`
	MarketplaceName       string `yaml:"marketplace_name"`
	MarketplacePath       string `yaml:"marketplace_path"`
	ServerVersion         string `yaml:"server_version"`
	ProtocolVersion       string `yaml:"protocol_version"`
	IOTimeoutMS           int    `yaml:"io_timeout_ms"`
	MaxMessageBytes       int    `yaml:"max_message_bytes"`
	MaxRPCBytes           int    `yaml:"max_rpc_bytes"`
	CanaryModel           string `yaml:"canary_model"`
	CanaryMessage         string `yaml:"canary_message"`
	WaitTimeoutMS         int    `yaml:"wait_timeout_ms"`
	ReceiptPollMS         int    `yaml:"receipt_poll_ms"`
	ReceiptRetentionMS    int    `yaml:"receipt_retention_ms"`
	MaxReceipts           int    `yaml:"max_receipts"`
	ClaudeMetadataDir     string `yaml:"claude_metadata_dir"`
	ClaudeMetadataPattern string `yaml:"claude_metadata_pattern"`
	MaxMetadataBytes      int    `yaml:"max_metadata_bytes"`
	ReplyToolName         string `yaml:"reply_tool_name"`
	ReplyToolDescription  string `yaml:"reply_tool_description"`
	SenderFormat          string `yaml:"sender_format"`
	TerminalSender        string `yaml:"terminal_sender"`
	Instructions          string `yaml:"instructions"`
}

// LoadConfig reads the embedded spec and expands its user-owned registry path.
func LoadConfig() (Config, error) {
	data, err := harnez.DefaultFS.ReadFile("spec/channel.yaml")
	if err != nil {
		return Config{}, fmt.Errorf("channel spec: %w", err)
	}
	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("channel spec: %w", err)
	}
	for _, path := range []*string{&cfg.RegistryDir, &cfg.ClaudeMetadataDir} {
		if strings.HasPrefix(*path, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return Config{}, err
			}
			*path = filepath.Join(homeDir, strings.TrimPrefix(*path, "~/"))
		}
	}
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if c.MarketplacePath == "" {
		return fmt.Errorf("channel spec: empty marketplace path")
	}
	if c.RegistryDir == "" || c.ServerVersion == "" || c.ProtocolVersion == "" || c.CanaryMessage == "" || c.Instructions == "" || c.ClaudeMetadataDir == "" || c.ClaudeMetadataPattern == "" || c.ReplyToolDescription == "" || c.SenderFormat == "" || c.TerminalSender == "" {
		return fmt.Errorf("channel spec: empty required string")
	}
	identifier := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !identifier.MatchString(c.ServerName) || !identifier.MatchString(c.CanaryModel) || !identifier.MatchString(c.ReplyToolName) || !identifier.MatchString(c.PluginName) || !identifier.MatchString(c.MarketplaceName) {
		return fmt.Errorf("channel spec: invalid server name or canary model")
	}
	if c.IOTimeoutMS <= 0 || c.MaxMessageBytes <= 0 || c.MaxRPCBytes <= 0 || c.WaitTimeoutMS <= 0 || c.ReceiptPollMS <= 0 || c.ReceiptRetentionMS <= 0 || c.MaxReceipts <= 0 || c.MaxMetadataBytes <= 0 {
		return fmt.Errorf("channel spec: limits must be positive")
	}
	return validateLoopback(c.ListenAddress)
}

func (c Config) timeout() time.Duration { return time.Duration(c.IOTimeoutMS) * time.Millisecond }

// validateLoopback refuses wildcard, external and DNS addresses before binding.
func validateLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("channel listen address: %w", err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() {
		return fmt.Errorf("channel listener requires a numeric loopback address: %q", address)
	}
	return nil
}
