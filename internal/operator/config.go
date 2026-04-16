// SPDX-FileCopyrightText: 2026 Dalibo <contact@dalibo.com>
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"fmt"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/decoder"
	"github.com/dalibo/cnpg-i-logical-backup/internal/metadata"
)

type PluginConfiguration struct {
	Cluster          *cnpgv1.Cluster
	ServerName       string
	LogicalBackupRef string
}

type Plugin struct {
	Cluster *cnpgv1.Cluster
	// Parameters are the configuration parameters of this plugin
	Parameters  map[string]string
	PluginIndex int
}

func NewPlugin(cluster cnpgv1.Cluster, pluginName string) *Plugin {
	result := &Plugin{Cluster: &cluster}

	result.PluginIndex = -1
	for idx, cfg := range result.Cluster.Spec.Plugins {
		if cfg.Name == pluginName {
			result.PluginIndex = idx
			result.Parameters = cfg.Parameters
		}
	}

	return result
}

func NewFromClusterJSON(clusterJSON []byte) (*PluginConfiguration, error) {
	var res cnpgv1.Cluster
	if err := decoder.DecodeObjectLenient(clusterJSON, &res); err != nil {
		return nil, fmt.Errorf("cluster not found")
	}
	return NewFromCluster(&res)
}

func NewFromCluster(cluster *cnpgv1.Cluster) (*PluginConfiguration, error) {
	helper := NewPlugin(
		*cluster,
		metadata.PluginName,
	)
	serverName := cluster.Name
	lbr := ""
	if l, ok := helper.Parameters["LogicalBackupRef"]; ok {
		lbr = l
	}
	result := &PluginConfiguration{
		Cluster:          cluster,
		ServerName:       serverName,
		LogicalBackupRef: lbr,
	}
	return result, nil
}
