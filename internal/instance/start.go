// SPDX-FileCopyrightText: 2025 Dalibo <contact@dalibo.com>
//
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"

	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/http"
	"github.com/cloudnative-pg/cnpg-i/pkg/backup"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LogicalBackupPlugin is the implementation of the PostgreSQL sidecar
type LogicalBackupPluginServer struct {
	Client     client.Client
	PGDataPath string
	// mutually exclusive with serverAddress
	PluginPath   string
	InstanceName string
}

// Start starts the GRPC service
func (c *LogicalBackupPluginServer) Start(ctx context.Context) error {
	enrich := func(server *grpc.Server) error {
		backup.RegisterBackupServer(server, BackupServiceImplementation{
			Client:       c.Client,
			InstanceName: c.InstanceName,
		})
		return nil
	}

	srv := http.Server{
		IdentityImpl: IdentityImplementation{
			Client: c.Client,
		},
		Enrichers:  []http.ServerEnricher{enrich},
		PluginPath: c.PluginPath,
	}

	return srv.Start(ctx)
}
