// SPDX-FileCopyrightText: 2025 Dalibo <contact@dalibo.com>
//
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	instance_logicalbackup "github.com/dalibo/cnpg-i-logical-backup/internal/instance"
)

// NewCmd creates a new instance command
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Starts the logical backup sidecar plugin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return instance_logicalbackup.Start(cmd.Context())
		},
	}

	_ = viper.BindEnv("namespace", "NAMESPACE")
	_ = viper.BindEnv("pod-name", "POD_NAME")
	_ = viper.BindEnv("pgdata", "PGDATA")

	return cmd
}
