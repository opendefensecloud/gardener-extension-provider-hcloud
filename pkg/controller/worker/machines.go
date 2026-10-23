/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package worker contains functions used at the worker controller
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/charts"
	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud"
	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis/transcoder"
)

// MachineClassKind yields the name of the machine class.
func (w *workerDelegate) MachineClassKind() string {
	return "MachineClass"
}

// MachineClass yields a newly initialized MachineClass object.
func (w *workerDelegate) MachineClass() client.Object {
	return &machinev1alpha1.MachineClass{}
}

// MachineClassList yields a newly initialized MachineClassList object.
func (w *workerDelegate) MachineClassList() client.ObjectList {
	return &machinev1alpha1.MachineClassList{}
}

// DeployMachineClasses generates and creates the HCloud specific machine classes.
//
// PARAMETERS
// ctx context.Context Execution context
func (w *workerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}

	if err := w.seedChartApplier.ApplyFromEmbeddedFS(ctx, charts.InternalChart, filepath.Join(charts.InternalChartsPath, "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses})); err != nil {
		return err
	}

	return w.backfillNodeTemplates(ctx)
}

// GenerateMachineDeployments generates the configuration for the desired machine deployments.
//
// PARAMETERS
// ctx context.Context Execution context
func (w *workerDelegate) GenerateMachineDeployments(ctx context.Context) (worker.MachineDeployments, error) {
	if w.machineDeployments == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return nil, err
		}
	}
	return w.machineDeployments, nil
}

// getSecretData returns the secret referenced by the WorkerDelegate instance's spec.
//
// PARAMETERS
// ctx context.Context Execution context
func (w *workerDelegate) getSecretData(ctx context.Context) (*corev1.Secret, error) {
	return extensionscontroller.GetSecretByReference(ctx, w.client, &w.worker.Spec.SecretRef)
}

// generateMachineClassSecretData returns the machine class relevant secret values.
//
// PARAMETERS
// ctx context.Context Execution context
func (w *workerDelegate) generateMachineClassSecretData(ctx context.Context) (map[string][]byte, error) {
	secret, err := w.getSecretData(ctx)
	if err != nil {
		return nil, err
	}

	credentials, err := hcloud.ExtractCredentials(secret)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		hcloud.HcloudToken: []byte(credentials.MCM().Token),
	}, nil
}

// poolZones yields the availability zones the machines of a worker pool are distributed over.
//
// Zones are optional in the Shoot API, but this provider derives one machine class and one
// machine deployment per zone, so a pool that leaves them unset would yield no machines at all.
// Fall back to the zones the cloud profile declares for the worker's region, and fail loudly
// when there are none rather than reconciling a node-less cluster.
//
// PARAMETERS
// pool extensionsv1alpha1.WorkerPool Worker pool to resolve the zones for
func (w *workerDelegate) poolZones(pool extensionsv1alpha1.WorkerPool) ([]string, error) {
	if len(pool.Zones) > 0 {
		return pool.Zones, nil
	}

	region := w.worker.Spec.Region

	for _, cloudProfileRegion := range w.cluster.CloudProfile.Spec.Regions {
		if cloudProfileRegion.Name != region {
			continue
		}

		zones := make([]string, 0, len(cloudProfileRegion.Zones))
		for _, zone := range cloudProfileRegion.Zones {
			zones = append(zones, zone.Name)
		}

		if len(zones) > 0 {
			return zones, nil
		}

		break
	}

	return nil, fmt.Errorf("worker pool %q specifies no zones and the cloud profile declares none for region %q", pool.Name, region)
}

// generateMachineConfig generates the machine config of the WorkerDelegate instance's spec.
//
// PARAMETERS
// ctx context.Context Execution context
func (w *workerDelegate) generateMachineConfig(ctx context.Context) error {
	var (
		machineDeployments = worker.MachineDeployments{}
		machineClasses     []map[string]interface{}
		// machineImages      []apis.MachineImage
	)

	machineClassSecretData, err := w.generateMachineClassSecretData(ctx)
	if err != nil {
		return err
	}

	infraStatus, err := transcoder.DecodeInfrastructureStatusFromWorker(w.worker)
	if err != nil {
		return err
	}

	workerStatus, err := transcoder.DecodeWorkerStatusFromWorker(w.worker)
	if err != nil {
		return err
	}

	sshFingerprint := infraStatus.SSHFingerprint

	if sshFingerprint == "" {
		sshFingerprint, err = apis.GetSSHFingerprint(w.worker.Spec.SSHPublicKey)
		if err != nil {
			return err
		}
	}

	if len(w.worker.Spec.Pools) == 0 {
		return fmt.Errorf("missing pool")
	}

	for _, pool := range w.worker.Spec.Pools {
		// A pool without zones would produce no machine classes at all, silently yielding a
		// cluster with no nodes (and, on deletion, a Worker that hangs forever waiting for the
		// machine class credentials secret to be acquired). Zones are optional in the Shoot API,
		// so fall back to the ones the cloud profile declares for the worker's region.
		zones, err := w.poolZones(pool)
		if err != nil {
			return err
		}

		zoneLen := int32(len(zones)) // #nosec: G115 - We validate if num pool zones exceeds max_int32.

		workerPoolHash, err := worker.WorkerPoolHash(pool, w.cluster, nil, nil)
		if err != nil {
			return err
		}

		imageName, err := w.findMachineImageName(ctx, pool.MachineImage.Name, pool.MachineImage.Version)
		if err != nil {
			return err
		}

		values, err := w.extractMachineValues(pool.MachineType)
		if err != nil {
			return fmt.Errorf("extracting machine values failed: %w", err)
		}

		userData, err := worker.FetchUserData(ctx, w.client, w.worker.Namespace, pool)
		if err != nil {
			return err
		}

		for zoneIndex, zone := range zones {
			zoneIdx := int32(zoneIndex) // #nosec: G115 - We validate if num pool zones exceeds max_int32.
			secretMap := map[string]interface{}{
				"userData": string(userData),
			}

			for key, value := range machineClassSecretData {
				secretMap[key] = value
			}

			machineClassSpec := map[string]interface{}{
				"cluster":        w.worker.Namespace,
				"zone":           zone,
				"imageName":      string(imageName),
				"sshFingerprint": sshFingerprint,
				"machineType":    string(pool.MachineType),
				"networkName":    fmt.Sprintf("%s-workers", w.worker.Namespace),
				"tags": map[string]string{
					"mcm.gardener.cloud/cluster": w.worker.Namespace,
					"mcm.gardener.cloud/role":    "node",
				},
				"credentialsSecretRef": map[string]interface{}{
					"name":      w.worker.Spec.SecretRef.Name,
					"namespace": w.worker.Spec.SecretRef.Namespace,
				},
				"secret": secretMap,
			}

			placementGroupName := fmt.Sprintf("%s-%s", w.worker.Namespace, pool.Name)
			if placementGroupID, ok := workerStatus.PlacementGroupIDs[placementGroupName]; ok {
				machineClassSpec["placementGroupID"] = placementGroupID
			}

			if infraStatus.FloatingPoolName != "" {
				machineClassSpec["floatingPoolName"] = infraStatus.FloatingPoolName
			}

			if values.MachineTypeOptions != nil {
				if len(values.MachineTypeOptions.ExtraConfig) > 0 {
					machineClassSpec["extraConfig"] = values.MachineTypeOptions.ExtraConfig
				}
			}

			machineClassSpec["nodeTemplate"] = nodeTemplate(pool, values.MachineType, w.worker.Spec.Region, zone)

			deploymentName := fmt.Sprintf("%s-%s-%s", w.worker.Namespace, pool.Name, zone)
			className := fmt.Sprintf("%s-%s", deploymentName, workerPoolHash)

			updateConfiguration := machinev1alpha1.UpdateConfiguration{
				MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxUnavailable, zoneLen, pool.Minimum)),
				MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxSurge, zoneLen, pool.Maximum)),
			}

			machineDeploymentStrategy := machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
					UpdateConfiguration: updateConfiguration,
				},
			}

			if gardencorev1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
				machineDeploymentStrategy = machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: updateConfiguration,
						OrchestrationType:   machinev1alpha1.OrchestrationTypeAuto,
					},
				}

				if gardencorev1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
					machineDeploymentStrategy.InPlaceUpdate.OrchestrationType = machinev1alpha1.OrchestrationTypeManual
				}
			}

			machineDeployments = append(machineDeployments, worker.MachineDeployment{
				Name:                 deploymentName,
				ClassName:            className,
				SecretName:           className,
				PoolName:             pool.Name,
				Minimum:              pool.Minimum,
				Maximum:              pool.Maximum,
				Priority:             pool.Priority,
				Strategy:             machineDeploymentStrategy,
				Labels:               pool.Labels,
				Annotations:          pool.Annotations,
				Taints:               pool.Taints,
				MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
			})

			machineClassSpec["name"] = className

			machineClasses = append(machineClasses, machineClassSpec)
		}

	}
	w.machineDeployments = machineDeployments
	w.machineClasses = machineClasses

	return nil
}

type machineValues struct {
	MachineType        *corev1beta1.MachineType
	MachineTypeOptions *apis.MachineTypeOptions
}

// nodeTemplate describes the machines of a pool to MCM: instance type, placement and capacity.
// Since v0.63.0, machine-controller-manager labels its machine duration metrics with the instance
// type from the MachineClass's node template and panics on a class without one: the machine never
// leaves Pending although its node has joined, and deleting it fails the same way. MCM has always
// rejected a template whose capacity lacks cpu, gpu or memory, when creating and deleting machines
// alike, so the capacity gardenlet derived for the pool is laid over the CloudProfile's own values.
func nodeTemplate(pool extensionsv1alpha1.WorkerPool, machineType *corev1beta1.MachineType, region, zone string) map[string]interface{} {
	capacity := machineTypeCapacity(machineType)
	template := map[string]interface{}{
		"instanceType": pool.MachineType,
		"region":       region,
		"zone":         zone,
		"architecture": ptr.Deref(pool.Architecture, v1beta1constants.ArchitectureAMD64),
	}

	if pool.NodeTemplate != nil {
		maps.Copy(capacity, pool.NodeTemplate.Capacity)
		if len(pool.NodeTemplate.VirtualCapacity) > 0 {
			template["virtualCapacity"] = quantityStrings(pool.NodeTemplate.VirtualCapacity)
		}
	}
	template["capacity"] = quantityStrings(capacity)

	return template
}

// machineTypeCapacity yields the capacity MCM requires in a node template -- cpu, gpu and memory --
// as the CloudProfile declares it for the machine type. Without a machine type all three are zero,
// which still passes MCM's validation.
func machineTypeCapacity(machineType *corev1beta1.MachineType) corev1.ResourceList {
	capacity := corev1.ResourceList{
		corev1.ResourceCPU:    resource.Quantity{},
		"gpu":                 resource.Quantity{},
		corev1.ResourceMemory: resource.Quantity{},
	}

	if machineType != nil {
		capacity[corev1.ResourceCPU] = machineType.CPU
		capacity["gpu"] = machineType.GPU
		capacity[corev1.ResourceMemory] = machineType.Memory
	}

	return capacity
}

// quantityStrings renders a resource list for the machineclass chart.
func quantityStrings(resources corev1.ResourceList) map[string]string {
	rendered := make(map[string]string, len(resources))
	for name, quantity := range resources {
		rendered[string(name)] = quantity.String()
	}

	return rendered
}

// backfillNodeTemplates gives a node template to every MachineClass in the shoot namespace that
// lacks one, built from the class's server type and zone, the CloudProfile machine type and the
// Worker's region. MCM >= v0.63.0 panics on any machine of a class without one (see nodeTemplate),
// after every restart again. DeployMachineClasses renders only the wanted classes, so the classes of
// a previous pool hash keep what an older release wrote until their machines are gone -- unpatched,
// their deletion would wedge the rolling update. PreReconcileHook calls this before the machine
// classes are generated, so it skips none as wanted there; DeployMachineClasses renders those in full
// afterwards.
//
// PARAMETERS
// ctx context.Context Execution context
func (w *workerDelegate) backfillNodeTemplates(ctx context.Context) error {
	wanted := make(map[string]struct{}, len(w.machineClasses))
	for _, machineClass := range w.machineClasses {
		if name, ok := machineClass["name"].(string); ok {
			wanted[name] = struct{}{}
		}
	}

	machineClasses := &machinev1alpha1.MachineClassList{}
	if err := w.client.List(ctx, machineClasses, client.InNamespace(w.worker.Namespace)); err != nil {
		return fmt.Errorf("could not list machine classes: %w", err)
	}

	for i := range machineClasses.Items {
		machineClass := &machineClasses.Items[i]
		if _, ok := wanted[machineClass.Name]; ok || machineClass.NodeTemplate != nil || machineClass.ProviderSpec.Raw == nil {
			continue
		}

		var providerSpec struct {
			ServerType string `json:"serverType"`
			Zone       string `json:"zone"`
		}
		// A class this cannot be derived for is left alone rather than failing: the error would never
		// go away and would block every reconciliation and the deletion of the Worker.
		if err := json.Unmarshal(machineClass.ProviderSpec.Raw, &providerSpec); err != nil {
			log.FromContext(ctx).Info("Not adding a node template to machine class, its provider spec cannot be decoded", "machineClass", machineClass.Name, "error", err.Error())
			continue
		}
		if providerSpec.ServerType == "" || providerSpec.Zone == "" {
			log.FromContext(ctx).Info("Not adding a node template to machine class, its provider spec lacks the server type or zone", "machineClass", machineClass.Name)
			continue
		}

		machineType := w.cloudProfileMachineType(providerSpec.ServerType)
		architecture := v1beta1constants.ArchitectureAMD64
		if machineType != nil && machineType.Architecture != nil {
			architecture = *machineType.Architecture
		}

		patch := client.MergeFrom(machineClass.DeepCopy())
		machineClass.NodeTemplate = &machinev1alpha1.NodeTemplate{
			Capacity:     machineTypeCapacity(machineType),
			InstanceType: providerSpec.ServerType,
			Region:       w.worker.Spec.Region,
			Zone:         providerSpec.Zone,
			Architecture: ptr.To(architecture),
		}
		if err := w.client.Patch(ctx, machineClass, patch); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not add a node template to machine class %s: %w", machineClass.Name, err)
		}
	}

	return nil
}

// cloudProfileMachineType yields the CloudProfile's machine type of the given name, or nil.
func (w *workerDelegate) cloudProfileMachineType(name string) *corev1beta1.MachineType {
	for _, mt := range w.cluster.CloudProfile.Spec.MachineTypes {
		if mt.Name == name {
			return &mt
		}
	}

	return nil
}

// extractMachineValues extracts the relevant machine values from the cloud profile spec.
//
// PARAMETERS
// ctx context.Context Execution context
func (w *workerDelegate) extractMachineValues(machineTypeName string) (*machineValues, error) {
	machineType := w.cloudProfileMachineType(machineTypeName)
	if machineType == nil {
		err := fmt.Errorf("machine type %s not found in cloud profile spec", machineTypeName)
		return nil, err
	}

	values := &machineValues{MachineType: machineType}

	cloudProfileConfig, err := transcoder.DecodeConfigFromCloudProfile(w.cluster.CloudProfile)
	if err != nil {
		return nil, err
	}

	for _, mt := range cloudProfileConfig.MachineTypeOptions {
		if mt.Name == machineTypeName {
			values.MachineTypeOptions = &mt
			break
		}
	}

	return values, nil
}
