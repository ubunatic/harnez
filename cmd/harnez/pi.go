// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/claude"
)

func newPiCmd() *cobra.Command {
	pi := &cobra.Command{Use: "pi", Short: "Manage Pi integrations"}
	tasks := &cobra.Command{Use: "background-tasks", Short: "Manage the Pi background-task extension"}
	var configPath string
	update := &cobra.Command{
		Use: "update", Short: "Install or update only the managed Pi background-task extension",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return claude.UpdatePiBackgroundTasks(cfg)
		},
	}
	update.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	tasks.AddCommand(update)
	pi.AddCommand(tasks)
	return pi
}
