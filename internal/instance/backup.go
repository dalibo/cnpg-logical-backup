// SPDX-FileCopyrightText: 2025 Dalibo <contact@dalibo.com>
//
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/cloudnative-pg/cnpg-i/pkg/backup"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type BackupServiceImplementation struct {
	Client       client.Client
	InstanceName string
	backup.UnimplementedBackupServer
}

func (b BackupServiceImplementation) GetCapabilities(
	_ context.Context, _ *backup.BackupCapabilitiesRequest,
) (*backup.BackupCapabilitiesResult, error) {
	return &backup.BackupCapabilitiesResult{
		Capabilities: []*backup.BackupCapability{
			{
				Type: &backup.BackupCapability_Rpc{
					Rpc: &backup.BackupCapability_RPC{
						Type: backup.BackupCapability_RPC_TYPE_BACKUP,
					},
				},
			},
		},
	}, nil
}

func (b BackupServiceImplementation) Backup(
	ctx context.Context,
	request *backup.BackupRequest,
) (*backup.BackupResult, error) {
	contextLogger := log.FromContext(ctx)
	// name := "logical-backup-" + b.InstanceName
	// pvc := &corev1.PersistentVolumeClaim{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Namespace: "default",
	// 		Name:      name,
	// 	},
	// 	Spec: corev1.PersistentVolumeClaimSpec{
	// 		StorageClassName: ptr.To("default"),
	// 		AccessModes: []corev1.PersistentVolumeAccessMode{
	// 			corev1.ReadWriteOnce,
	// 		},
	// 		Resources: corev1.VolumeResourceRequirements{
	// 			Requests: corev1.ResourceList{
	// 				corev1.ResourceStorage: resource.MustParse("5G"),
	// 			},
	// 		},
	// 	},
	// }
	// // Create PVC if it does not exist
	// if err := b.Client.Create(ctx, pvc); err != nil {
	// 	return nil, err
	// }

	contextLogger.Info("should backup instance!")
	if err := b.runBackupCmd(ctx); err != nil {
		return nil, err
	}
	return &backup.BackupResult{}, nil

}

func (b BackupServiceImplementation) runBackupCmd(ctx context.Context) error {
	logger := log.FromContext(ctx)
	cmd := exec.Command(
		"/usr/bin/pg_back",
		"--backup-directory",
		"/controller/tmp/backups",
		"-F",
		"plain",
	)
	cmd.Env = os.Environ()
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	cmd.Start()
	var wg sync.WaitGroup

	readP := func(pipe io.ReadCloser) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			l := scanner.Text()
			logger.Info(l)
		}
	}
	// read stdout
	wg.Add(2)
	go readP(stdoutPipe)
	go readP(stderrPipe)
	return cmd.Wait()
}
