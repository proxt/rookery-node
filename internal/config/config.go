// Package config loads the node's YAML configuration. Every field has a
// sane default except the panel registration fields (panel_addr, node_id,
// api_key), which are required — a node is useless without them, since it
// has no user database of its own anymore and relies entirely on the panel
// for session token verification and traffic reporting.
package config

import (
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the node's runtime configuration.
type Config struct {
	ListenAddr           string `yaml:"listen_addr"`
	ICEUDPPort           int    `yaml:"ice_udp_port"`
	PanelAddr            string `yaml:"panel_addr"`
	NodeID               string `yaml:"node_id"`
	APIKey               string `yaml:"api_key"`
	MaxStreams           int    `yaml:"max_streams_per_session"`
	DialTimeoutSec       int    `yaml:"dial_timeout_seconds"`
	AllowPrivateNet      bool   `yaml:"allow_private_net"`
	BufferedAmountLowKB  int    `yaml:"buffered_amount_low_kb"`
	BufferedAmountHighKB int    `yaml:"buffered_amount_high_kb"`
	LogLevel             string `yaml:"log_level"`
}

// Defaults returns the configuration used for anything not overridden by a
// config file. panel_addr/node_id/api_key have no default — see Validate.
func Defaults() Config {
	return Config{
		ListenAddr:           "127.0.0.1:8080",
		ICEUDPPort:           51000,
		MaxStreams:           256,
		DialTimeoutSec:       10,
		AllowPrivateNet:      false,
		BufferedAmountLowKB:  256,
		BufferedAmountHighKB: 1024,
		LogLevel:             "info",
	}
}

// Load reads a Config from a YAML file at path, applying Defaults() for any
// field the file doesn't set.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks that all fields are sane.
func (c *Config) Validate() error {
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return fmt.Errorf("config: listen_addr %q: %w", c.ListenAddr, err)
	}
	if c.ICEUDPPort <= 0 || c.ICEUDPPort > 65535 {
		return fmt.Errorf("config: ice_udp_port must be between 1 and 65535, got %d", c.ICEUDPPort)
	}
	if c.PanelAddr == "" {
		return fmt.Errorf("config: panel_addr is required")
	}
	if c.NodeID == "" {
		return fmt.Errorf("config: node_id is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("config: api_key is required")
	}
	if c.MaxStreams <= 0 {
		return fmt.Errorf("config: max_streams_per_session must be positive, got %d", c.MaxStreams)
	}
	if c.DialTimeoutSec <= 0 {
		return fmt.Errorf("config: dial_timeout_seconds must be positive, got %d", c.DialTimeoutSec)
	}
	if c.BufferedAmountLowKB <= 0 {
		return fmt.Errorf("config: buffered_amount_low_kb must be positive, got %d", c.BufferedAmountLowKB)
	}
	if c.BufferedAmountHighKB <= c.BufferedAmountLowKB {
		return fmt.Errorf("config: buffered_amount_high_kb (%d) must be greater than buffered_amount_low_kb (%d)", c.BufferedAmountHighKB, c.BufferedAmountLowKB)
	}
	return nil
}
