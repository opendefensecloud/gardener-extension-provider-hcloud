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
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerclient "github.com/gardener/gardener/pkg/client/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/charts"
	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis/mock"
	mockclient "github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis/mock/client"
	hcloudv1alpha1 "github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis/v1alpha1"
)

// newWorkerDelegate creates a new context for a worker reconciliation.
func newWorkerDelegate(
	client *mockclient.MockClient,
	scheme *runtime.Scheme,
	seedChartApplier gardenerclient.ChartApplier,
	serverVersion string,
	worker *v1alpha1.Worker,
	cluster *v1alpha1.Cluster,
) (genericactuator.WorkerDelegate, error) {
	var decodedCluster *controller.Cluster

	if nil != cluster {
		newDecodedCluster, err := mock.DecodeCluster(cluster)
		if nil != err {
			return nil, err
		}

		decodedCluster = newDecodedCluster
	}

	workerDelegate, err := NewWorkerDelegate(client, scheme, seedChartApplier, serverVersion, worker, decodedCluster)
	if nil != err {
		return nil, err
	}

	return workerDelegate, nil
}

var (
	mockTestEnv mock.MockTestEnv
	scheme      *runtime.Scheme
)

var _ = BeforeSuite(func() {
	mockTestEnv = mock.NewMockTestEnv()

	apis.SetClientForToken("dummy-token", mockTestEnv.HcloudClient)
	mock.SetupImagesEndpointOnMux(mockTestEnv.Mux)

	scheme = runtime.NewScheme()
	_ = apis.AddToScheme(scheme)
	_ = hcloudv1alpha1.AddToScheme(scheme)
})

var _ = AfterSuite(func() {
	mockTestEnv.Teardown()
})

var _ = Describe("Machines", func() {
	Describe("#DeployMachineClasses", func() {
		type setup struct {
		}

		type action struct {
			cluster *v1alpha1.Cluster
			worker  *v1alpha1.Worker
		}

		type expect struct {
			errToHaveOccurred bool
			err               error
			machineClasses    []map[string]interface{}
		}

		type data struct {
			setup  setup
			action action
			expect expect
		}

		machineClassName := fmt.Sprintf("%s-%s-%s-%s", mock.TestNamespace, mock.TestWorkerPoolName, mock.TestZone, "48c9a")

		// The node template a pool without its own yields: capacity from the cloud profile machine type.
		cloudProfileNodeTemplate := map[string]interface{}{
			"capacity":     map[string]string{"cpu": "1", "gpu": "0", "memory": "2Gi"},
			"instanceType": mock.TestWorkerMachineType,
			"region":       mock.TestRegion,
			"zone":         mock.TestZone,
			"architecture": "amd64",
		}

		// The single machine class the mock worker/cloud profile pair yields, with the given node
		// template. Shared by the explicit-zones and defaulted-zones entries, which must produce
		// identical output.
		expectedMachineClasses := func(nodeTemplate map[string]interface{}) []map[string]interface{} {
			return []map[string]interface{}{{
				"name": machineClassName,
				"credentialsSecretRef": map[string]interface{}{
					"name":      "secret",
					"namespace": "test-namespace"},
				"cluster":          mock.TestNamespace,
				"zone":             mock.TestZone,
				"imageName":        fmt.Sprintf("%s-%s", mock.TestWorkerMachineImageName, mock.TestWorkerMachineImageVersion),
				"sshFingerprint":   mock.TestSSHFingerprint,
				"machineType":      mock.TestWorkerMachineType,
				"floatingPoolName": mock.TestFloatingPoolName,
				"networkName":      fmt.Sprintf("%s-workers", mock.TestNamespace),
				"tags": map[string]string{
					"mcm.gardener.cloud/cluster": mock.TestNamespace,
					"mcm.gardener.cloud/role":    "node",
				},
				"secret": map[string]interface{}{
					"hcloudToken": []byte("dummy-token"),
					"userData":    mock.TestWorkerUserData,
				},
				"nodeTemplate": nodeTemplate,
			}}
		}

		DescribeTable("##table",
			func(data *data) {
				chartApplier := mockkubernetes.NewMockChartApplier(mockTestEnv.MockController)
				ctx := context.TODO()

				mockTestEnv.Client.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
					func(_ context.Context, objectKey k8sclient.ObjectKey, secret *corev1.Secret, _ ...k8sclient.GetOption) error {
						Expect(objectKey.Namespace).To(Equal(mock.TestNamespace))

						switch objectKey.Name {
						case mock.TestWorkerSecretName:
							secret.Data = map[string][]byte{
								"hcloudToken": []byte("dummy-token"),
							}
						case mock.TestUserDataSecretName:
							secret.Data = map[string][]byte{mock.TestUserDataSecretDataKey: []byte(mock.TestWorkerUserData)}
						default:
							return fmt.Errorf("unexpected secret name %s", objectKey.Name)
						}

						return nil
					}).AnyTimes()

				chartApplier.EXPECT().ApplyFromEmbeddedFS(
					ctx,
					charts.InternalChart,
					filepath.Join(charts.InternalChartsPath, "machineclass"),
					mock.TestNamespace,
					"machineclass",
					gardenerclient.Values(
						map[string]interface{}{"machineClasses": data.expect.machineClasses},
					),
				).AnyTimes()

				// No machine classes of an earlier pool hash: nothing to backfill.
				mockTestEnv.Client.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&machinev1alpha1.MachineClassList{}), gomock.Any()).Return(nil).AnyTimes()

				workerDelegate, err := newWorkerDelegate(mockTestEnv.Client, scheme, chartApplier, "", data.action.worker, data.action.cluster)
				Expect(err).NotTo(HaveOccurred())

				err = workerDelegate.DeployMachineClasses(ctx)

				if data.expect.errToHaveOccurred {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(data.expect.err))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},

			Entry("should successfully deploy machine classes", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.NewWorker(),
				},
				expect: expect{
					errToHaveOccurred: false,
					machineClasses:    expectedMachineClasses(cloudProfileNodeTemplate),
				},
			}),

			// The pool's template (gardenlet's) takes precedence, laid over the cloud profile values so
			// that cpu, gpu and memory stay present: MCM >= v0.63.0 rejects a template without any of
			// them when creating or deleting machines. Virtual capacity is passed through.
			Entry("should take the node template capacity, virtual capacity and architecture from the pool", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.ManipulateWorker(mock.NewWorker(), map[string]interface{}{
						"Spec.Pools.0.NodeTemplate": &v1alpha1.NodeTemplate{
							Capacity: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
							VirtualCapacity: corev1.ResourceList{
								"example.com/virtual": resource.MustParse("10"),
							},
						},
						"Spec.Pools.0.Architecture": ptr.To("arm64"),
					}),
				},
				expect: expect{
					errToHaveOccurred: false,
					machineClasses: expectedMachineClasses(map[string]interface{}{
						"capacity":        map[string]string{"cpu": "4", "gpu": "0", "memory": "8Gi"},
						"virtualCapacity": map[string]string{"example.com/virtual": "10"},
						"instanceType":    mock.TestWorkerMachineType,
						"region":          mock.TestRegion,
						"zone":            mock.TestZone,
						"architecture":    "arm64",
					}),
				},
			}),

			// A pool may legitimately omit zones (they are optional in the Shoot API). Deriving
			// them from the cloud profile keeps the pool from silently producing no machines.
			Entry("should default the zones from the cloud profile region when the pool has none", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.ManipulateWorker(mock.NewWorker(), map[string]interface{}{"Spec.Pools.0.Zones": []string{}}),
				},
				expect: expect{
					errToHaveOccurred: false,
					machineClasses:    expectedMachineClasses(cloudProfileNodeTemplate),
				},
			}),

			Entry("should fail when neither the pool nor the cloud profile region declares zones", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.ManipulateWorker(mock.NewWorker(), map[string]interface{}{
						"Spec.Pools.0.Zones": []string{},
						"Spec.Region":        "nowhere",
					}),
				},
				expect: expect{
					err:               fmt.Errorf("worker pool %q specifies no zones and the cloud profile declares none for region %q", mock.TestWorkerPoolName, "nowhere"),
					errToHaveOccurred: true,
				},
			}),
			Entry("should fail because of invalid image name", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.ManipulateWorker(
						mock.NewWorker(),
						map[string]interface{}{
							"Spec.Pools.0.MachineImage": v1alpha1.MachineImage{
								Name:    "test",
								Version: "1.0",
							},
						},
					),
				},
				expect: expect{
					err:               errors.New("could not find machine image for test/1.0 neither in cloud profile nor in worker status"),
					errToHaveOccurred: true,
				},
			}),
		)

		// MCM >= v0.63.0 panics on any machine of a class without a node template. Classes of an earlier
		// pool hash are not rendered any more but keep their machines until the rolling update has
		// replaced them, so they get one backfilled -- and every class without one gets it before the
		// Hetzner checks of a reconcile, which can fail before the classes are rendered at all.
		Context("backfilling node templates", func() {
			var (
				ctx          context.Context
				client       *mockclient.MockClient
				chartApplier *mockkubernetes.MockChartApplier
				patched      map[string]*machinev1alpha1.NodeTemplate
			)

			machineClass := func(name, serverType string, nodeTemplate *machinev1alpha1.NodeTemplate) machinev1alpha1.MachineClass {
				return machinev1alpha1.MachineClass{
					ObjectMeta:   metav1.ObjectMeta{Name: name, Namespace: mock.TestNamespace},
					ProviderSpec: runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"cluster":%q,"zone":%q,"serverType":%q}`, mock.TestNamespace, mock.TestZone, serverType))},
					NodeTemplate: nodeTemplate,
				}
			}

			existingMachineClasses := func(machineClasses ...machinev1alpha1.MachineClass) {
				client.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&machinev1alpha1.MachineClassList{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, list *machinev1alpha1.MachineClassList, _ ...k8sclient.ListOption) error {
						list.Items = machineClasses

						return nil
					})
			}

			// Records the node template each patch actually sends, decoded from the patch data rather
			// than taken from the object, and fails a patch that would change anything else.
			expectPatches := func(times int, result func(name string) error) {
				client.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&machinev1alpha1.MachineClass{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, obj *machinev1alpha1.MachineClass, patch k8sclient.Patch, _ ...k8sclient.PatchOption) error {
						data, err := patch.Data(obj)
						Expect(err).NotTo(HaveOccurred())

						fields := map[string]json.RawMessage{}
						Expect(json.Unmarshal(data, &fields)).To(Succeed())
						Expect(fields).To(HaveLen(1))
						Expect(fields).To(HaveKey("nodeTemplate"))

						nodeTemplate := &machinev1alpha1.NodeTemplate{}
						Expect(json.Unmarshal(fields["nodeTemplate"], nodeTemplate)).To(Succeed())
						patched[obj.Name] = nodeTemplate

						return result(obj.Name)
					}).Times(times)
			}
			succeed := func(string) error { return nil }

			newDelegate := func() *workerDelegate {
				delegate, err := newWorkerDelegate(client, scheme, chartApplier, "", mock.NewWorker(), mock.NewCluster())
				Expect(err).NotTo(HaveOccurred())

				return delegate.(*workerDelegate)
			}

			expectNodeTemplate := func(name, instanceType, architecture string, capacity map[string]string) {
				Expect(patched).To(HaveKey(name))
				nodeTemplate := patched[name]
				Expect(nodeTemplate.InstanceType).To(Equal(instanceType), name)
				Expect(quantityStrings(nodeTemplate.Capacity)).To(Equal(capacity), name)
				Expect(nodeTemplate.Region).To(Equal(mock.TestRegion), name)
				Expect(nodeTemplate.Zone).To(Equal(mock.TestZone), name)
				Expect(nodeTemplate.Architecture).To(Equal(ptr.To(architecture)), name)
			}

			BeforeEach(func() {
				ctx = context.TODO()
				ctrl := gomock.NewController(GinkgoT())
				client = mockclient.NewMockClient(ctrl)
				chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
				patched = map[string]*machinev1alpha1.NodeTemplate{}

				client.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
					func(_ context.Context, objectKey k8sclient.ObjectKey, secret *corev1.Secret, _ ...k8sclient.GetOption) error {
						switch objectKey.Name {
						case mock.TestWorkerSecretName:
							secret.Data = map[string][]byte{"hcloudToken": []byte("dummy-token")}
						case mock.TestUserDataSecretName:
							secret.Data = map[string][]byte{mock.TestUserDataSecretDataKey: []byte(mock.TestWorkerUserData)}
						default:
							return fmt.Errorf("unexpected secret name %s", objectKey.Name)
						}

						return nil
					}).AnyTimes()
			})

			It("should add one to the classes of an earlier pool hash after rendering the wanted ones", func() {
				chartApplier.EXPECT().ApplyFromEmbeddedFS(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				undecodable := machineClass("old-undecodable", mock.TestWorkerMachineType, nil)
				undecodable.ProviderSpec.Raw = []byte(`{"zone":5}`)
				existingMachineClasses(
					// Wanted: the chart has just rendered it with its own template.
					machineClass(machineClassName, mock.TestWorkerMachineType, nil),
					machineClass("old", mock.TestWorkerMachineType, nil),
					machineClass("old-arm", "cax11", nil),
					// Its machine type has since left the cloud profile.
					machineClass("old-unknown-type", "cpx99", nil),
					machineClass("old-with-template", mock.TestWorkerMachineType, &machinev1alpha1.NodeTemplate{InstanceType: mock.TestWorkerMachineType}),
					machineClass("old-without-zone-or-type", "", nil),
					undecodable,
				)
				expectPatches(3, succeed)

				Expect(newDelegate().DeployMachineClasses(ctx)).To(Succeed())

				Expect(patched).To(HaveLen(3))
				expectNodeTemplate("old", mock.TestWorkerMachineType, "amd64", map[string]string{"cpu": "1", "gpu": "0", "memory": "2Gi"})
				expectNodeTemplate("old-arm", "cax11", "arm64", map[string]string{"cpu": "2", "gpu": "0", "memory": "4Gi"})
				expectNodeTemplate("old-unknown-type", "cpx99", "amd64", map[string]string{"cpu": "0", "gpu": "0", "memory": "0"})
			})

			It("should add one to every class without one before the Hetzner checks of a reconcile", func() {
				existingMachineClasses(
					machineClass(machineClassName, mock.TestWorkerMachineType, nil),
					machineClass("old", mock.TestWorkerMachineType, nil),
				)
				expectPatches(2, succeed)

				// The mock Hetzner API serves no server types, so the hook fails right after the backfill --
				// as it does on a real API error or a machine type out of stock.
				Expect(newDelegate().PreReconcileHook(ctx)).To(MatchError(ContainSubstring("unable to list server types")))

				Expect(patched).To(HaveLen(2))
				expectNodeTemplate(machineClassName, mock.TestWorkerMachineType, "amd64", map[string]string{"cpu": "1", "gpu": "0", "memory": "2Gi"})
				expectNodeTemplate("old", mock.TestWorkerMachineType, "amd64", map[string]string{"cpu": "1", "gpu": "0", "memory": "2Gi"})
			})

			It("should ignore a class deleted in the meantime", func() {
				chartApplier.EXPECT().ApplyFromEmbeddedFS(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				existingMachineClasses(machineClass("old", mock.TestWorkerMachineType, nil))
				expectPatches(1, func(name string) error {
					return apierrors.NewNotFound(schema.GroupResource{Group: "machine.sapcloud.io", Resource: "machineclasses"}, name)
				})

				Expect(newDelegate().DeployMachineClasses(ctx)).To(Succeed())
			})

			It("should fail when a class cannot be patched", func() {
				chartApplier.EXPECT().ApplyFromEmbeddedFS(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				existingMachineClasses(machineClass("old", mock.TestWorkerMachineType, nil))
				expectPatches(1, func(string) error { return errors.New("conflict") })

				Expect(newDelegate().DeployMachineClasses(ctx)).To(MatchError(ContainSubstring("could not add a node template to machine class old")))
			})

			It("should fail when the classes cannot be listed", func() {
				chartApplier.EXPECT().ApplyFromEmbeddedFS(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				client.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&machinev1alpha1.MachineClassList{}), gomock.Any()).Return(errors.New("unavailable"))

				Expect(newDelegate().DeployMachineClasses(ctx)).To(MatchError(ContainSubstring("could not list machine classes")))
			})
		})
	})

	Describe("#GenerateMachineDeployments", func() {
		type setup struct {
		}

		type action struct {
			cluster *v1alpha1.Cluster
			worker  *v1alpha1.Worker
		}

		type expect struct {
			errToHaveOccurred          bool
			err                        error
			numberOfMachineDeployments int
		}

		type data struct {
			setup  setup
			action action
			expect expect
		}

		DescribeTable("##table",
			func(data *data) {
				ctx := context.TODO()

				mockTestEnv.Client.EXPECT().Get(ctx, k8sclient.ObjectKey{Namespace: mock.TestNamespace, Name: mock.TestWorkerSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ k8sclient.ObjectKey, secret *corev1.Secret, _ ...k8sclient.GetOption) error {
					secret.Data = map[string][]byte{
						"hcloudToken": []byte("dummy-token"),
					}

					return nil
				}).AnyTimes()

				workerDelegate, err := newWorkerDelegate(mockTestEnv.Client, scheme, nil, "", data.action.worker, data.action.cluster)
				Expect(err).NotTo(HaveOccurred())

				result, err := workerDelegate.GenerateMachineDeployments(ctx)

				if data.expect.errToHaveOccurred {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(data.expect.err))
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(result).Should(HaveLen(data.expect.numberOfMachineDeployments))

					// The generic worker actuator rejects MachineDeployments whose PoolName is not a worker pool.
					poolNames := make([]string, 0, len(data.action.worker.Spec.Pools))
					for _, pool := range data.action.worker.Spec.Pools {
						poolNames = append(poolNames, pool.Name)
					}
					for _, machineDeployment := range result {
						Expect(machineDeployment.PoolName).To(BeElementOf(poolNames))
					}
				}
			},

			Entry("should successfully generate machine deployments", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.NewWorker(),
				},
				expect: expect{
					errToHaveOccurred:          false,
					numberOfMachineDeployments: 1,
				},
			}),

			Entry("should default the zones from the cloud profile region when the pool has none", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.ManipulateWorker(mock.NewWorker(), map[string]interface{}{"Spec.Pools.0.Zones": []string{}}),
				},
				expect: expect{
					errToHaveOccurred:          false,
					numberOfMachineDeployments: 1,
				},
			}),

			Entry("should fail when neither the pool nor the cloud profile region declares zones", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.ManipulateWorker(mock.NewWorker(), map[string]interface{}{
						"Spec.Pools.0.Zones": []string{},
						"Spec.Region":        "nowhere",
					}),
				},
				expect: expect{
					err:               fmt.Errorf("worker pool %q specifies no zones and the cloud profile declares none for region %q", mock.TestWorkerPoolName, "nowhere"),
					errToHaveOccurred: true,
				},
			}),
			Entry("should fail because of invalid image name", &data{
				setup: setup{},
				action: action{
					mock.NewCluster(),
					mock.ManipulateWorker(
						mock.NewWorker(),
						map[string]interface{}{
							"Spec.Pools.0.MachineImage": v1alpha1.MachineImage{
								Name:    "test",
								Version: "1.0",
							},
						},
					),
				},
				expect: expect{
					err:               errors.New("could not find machine image for test/1.0 neither in cloud profile nor in worker status"),
					errToHaveOccurred: true,
				},
			}),
		)
	})
})
