// SPDX-FileCopyrightText: 2025 Dalibo <contact@dalibo.com>
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"errors"
	"fmt"
	"os"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/decoder"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/object"
	"github.com/cloudnative-pg/cnpg-i/pkg/lifecycle"
	"github.com/cloudnative-pg/machinery/pkg/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SIDECAR_NAME string = "plugin-logical-backup"
)

// LifecycleImplementation is the implementation of the lifecycle handler
type LifecycleImplementation struct {
	lifecycle.UnimplementedOperatorLifecycleServer
	Client client.Client
}

// GetCapabilities exposes the lifecycle capabilities
func (impl LifecycleImplementation) GetCapabilities(
	_ context.Context,
	_ *lifecycle.OperatorLifecycleCapabilitiesRequest,
) (*lifecycle.OperatorLifecycleCapabilitiesResponse, error) {
	return &lifecycle.OperatorLifecycleCapabilitiesResponse{
		LifecycleCapabilities: []*lifecycle.OperatorLifecycleCapabilities{
			{
				Group: "",
				Kind:  "Pod",
				OperationTypes: []*lifecycle.OperatorOperationType{
					{
						Type: lifecycle.OperatorOperationType_TYPE_CREATE,
					},
					{
						Type: lifecycle.OperatorOperationType_TYPE_EVALUATE,
					},
				},
			},
			{
				Group: batchv1.GroupName,
				Kind:  "Job",
				OperationTypes: []*lifecycle.OperatorOperationType{
					{
						Type: lifecycle.OperatorOperationType_TYPE_CREATE,
					},
				},
			},
		},
	}, nil
}

func (impl LifecycleImplementation) LifecycleHook(
	ctx context.Context,
	request *lifecycle.OperatorLifecycleRequest,
) (*lifecycle.OperatorLifecycleResponse, error) {
	contextLogger := log.FromContext(ctx).WithName("lifecycle")
	contextLogger.Info("Lifecycle hook reconciliation start")

	// retrieve information about current object manipulated by the request
	operation := request.GetOperationType().GetType().Enum()
	if operation == nil {
		return nil, errors.New("no operation set")
	}

	kind, err := object.GetKind(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}

	var cluster cnpgv1.Cluster
	if err := decoder.DecodeObjectLenient(request.GetClusterDefinition(), &cluster); err != nil {
		return nil, err
	}
	// TODO: Reuse plugin config
	// pluginConfig, err := NewFromCluster(&cluster)
	if err != nil {
		return nil, fmt.Errorf("can't parse user parameters: %w", err)
	}
	switch kind {
	case "Pod":
		contextLogger.Info("pod pod")
		return impl.reconcilePod(ctx, &cluster, request)
	case "Job":
		contextLogger.Info("job job")
		// return impl.reconcileJob(ctx, &cluster, request)
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}
}

// reconcilePod handles lifecycle reconciliation and injects the sidecar
func (impl LifecycleImplementation) reconcilePod(
	ctx context.Context,
	cluster *cnpgv1.Cluster,
	request *lifecycle.OperatorLifecycleRequest,
) (*lifecycle.OperatorLifecycleResponse, error) {
	logger := log.FromContext(ctx).WithName("lifecycle")

	// Decode pod
	pod, err := decoder.DecodePodJSON(request.GetObjectDefinition())
	logger.Info("reconciling pod", "pod name", pod.Name)
	if err != nil {
		return nil, err
	}
	sidecar := corev1.Container{Args: []string{"instance"}}
	sidecar.Name = "plugin-logical-backup"
	mutatedPod := pod.DeepCopy()

	reconcilePodSpec(cluster, &mutatedPod.Spec, "postgres", &sidecar)
	if err := object.InjectPluginInitContainerSidecarSpec(&mutatedPod.Spec, &sidecar, true); err != nil {
		return nil, err
	}
	injectPluginVolumeMount(&mutatedPod.Spec, "postgres")

	// Create JSON patch
	patch, err := object.CreatePatch(mutatedPod, pod)
	if err != nil {
		return nil, err
	}

	logger.Info("patched object", "patch", string(patch))
	return &lifecycle.OperatorLifecycleResponse{JsonPatch: patch}, nil
}

func reconcilePodSpec(
	cluster *cnpgv1.Cluster,
	spec *corev1.PodSpec,
	mainContainerName string,
	containerConfig *corev1.Container,
) {
	// TODO: add baseprobe here
	// ...

	// Set required fields
	img := "sidecar-logical-backup"
	if i, ok := os.LookupEnv("SIDECAR_IMAGE"); ok {
		img = i
	}
	containerConfig.Image = img
	containerConfig.ImagePullPolicy = cluster.Spec.ImagePullPolicy
	containerConfig.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		RunAsNonRoot:             ptr.To(true),
		Privileged:               ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
	containerConfig.Env = envFromContainer(mainContainerName, spec, containerConfig.Env)
	// containerConfig.StartupProbe = baseProbe.DeepCopy()
	containerConfig.RestartPolicy = ptr.To(corev1.ContainerRestartPolicyAlways)
	object.InjectPluginVolumeSpec(spec)
}

func envFromContainer(
	srcContainerName string,
	srcPod *corev1.PodSpec,
	destEnvVar []corev1.EnvVar,
) []corev1.EnvVar {
	var env []corev1.EnvVar
	existing := make(map[string]struct{}, len(destEnvVar))
	for _, d := range destEnvVar {
		existing[d.Name] = struct{}{}
	}
	var oriContainer *corev1.Container
	for i := range srcPod.Containers {
		if srcPod.Containers[i].Name == srcContainerName {
			oriContainer = &srcPod.Containers[i]
			break
		}
	}
	if oriContainer != nil {
		for _, srcEnv := range oriContainer.Env {
			if _, ok := existing[srcEnv.Name]; !ok {
				env = append(env, srcEnv)
			}
		}
	}
	return env
}

// injects the plugin volume (/plugin) into a CNPG Pod spec.
func injectPluginVolumeMount(spec *corev1.PodSpec, mainContainerName string) {
	const (
		pluginVolumeName = "plugins"
		pluginMountPath  = "/plugins"
	)
	spec.Volumes = ensureVolume(spec.Volumes, corev1.Volume{
		Name: pluginVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	for i := range spec.Containers {
		if spec.Containers[i].Name == mainContainerName {
			spec.Containers[i].VolumeMounts = ensureVolumeMount(
				spec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      pluginVolumeName,
					MountPath: pluginMountPath,
				},
			)
		}
	}
}

// ensureVolume makes sure the passed volume is present in the list of volumes.
// If the volume is already present, it is updated.
func ensureVolume(volumes []corev1.Volume, volume corev1.Volume) []corev1.Volume {
	volumeFound := false
	for i := range volumes {
		if volumes[i].Name == volume.Name {
			volumeFound = true
			volumes[i] = volume
		}
	}

	if !volumeFound {
		volumes = append(volumes, volume)
	}

	return volumes
}

// ensureVolumeMount makes sure the passed volume mounts are present in the list of volume mounts.
// If a volume mount is already present, it is updated.
func ensureVolumeMount(
	mounts []corev1.VolumeMount,
	volumeMounts ...corev1.VolumeMount,
) []corev1.VolumeMount {
	for _, mount := range volumeMounts {
		mountFound := false
		for i := range mounts {
			if mounts[i].Name == mount.Name {
				mountFound = true
				mounts[i] = mount
				break
			}
		}

		if !mountFound {
			mounts = append(mounts, mount)
		}
	}

	return mounts
}
