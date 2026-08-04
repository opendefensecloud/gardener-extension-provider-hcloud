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

package validator

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Shoot validator", func() {
	const (
		cloudProfileName = "hcloud"
		region           = "fsn1"
	)

	var (
		ctx       = context.Background()
		validator *shoot

		newInfrastructureConfig = func(workersCIDR string) *runtime.RawExtension {
			return &runtime.RawExtension{
				Raw: []byte(`{"apiVersion":"hcloud.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureConfig","networks":{"workers":"` + workersCIDR + `"}}`),
			}
		}

		newShoot = func(networking *core.Networking, workersCIDR string) *core.Shoot {
			return &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "garden-dev"},
				Spec: core.ShootSpec{
					CloudProfile: &core.CloudProfileReference{Name: cloudProfileName},
					Region:       region,
					Networking:   networking,
					Provider: core.Provider{
						Type:                 "hcloud",
						InfrastructureConfig: newInfrastructureConfig(workersCIDR),
						Workers: []core.Worker{{
							Name:    "worker",
							Minimum: 1,
							Maximum: 2,
							Zones:   []string{"fsn1-dc14"},
						}},
					},
				},
			}
		}

		defaultNetworking = func() *core.Networking {
			return &core.Networking{
				Type:     ptr.To("calico"),
				Nodes:    ptr.To("10.250.0.0/16"),
				Pods:     ptr.To("100.96.0.0/11"),
				Services: ptr.To("100.64.0.0/13"),
			}
		}
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

		client := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&gardencorev1beta1.CloudProfile{ObjectMeta: metav1.ObjectMeta{Name: cloudProfileName}}).
			Build()

		validator = &shoot{}
		Expect(validator.InjectClient(client)).To(Succeed())
	})

	Describe("#Validate", func() {
		It("should accept a shoot whose worker network matches the nodes CIDR", func() {
			Expect(validator.Validate(ctx, newShoot(defaultNetworking(), "10.250.0.0/16"), nil)).To(Succeed())
		})

		It("should reject a shoot without a networking section instead of panicking", func() {
			err := validator.Validate(ctx, newShoot(nil, "10.250.0.0/16"), nil)

			Expect(err).To(MatchError(ContainSubstring("spec.networking.nodes")))
		})

		It("should reject a worker network outside of the nodes CIDR", func() {
			err := validator.Validate(ctx, newShoot(defaultNetworking(), "10.251.0.0/16"), nil)

			Expect(err).To(MatchError(ContainSubstring("must be a subset of")))
		})

		It("should reject a worker network outside the private IPv4 ranges", func() {
			networking := defaultNetworking()
			networking.Nodes = ptr.To("1.2.3.0/24")

			err := validator.Validate(ctx, newShoot(networking, "1.2.3.0/24"), nil)

			Expect(err).To(MatchError(ContainSubstring("private IPv4 range")))
		})

		It("should reject a wrong object type", func() {
			Expect(validator.Validate(ctx, &core.Seed{}, nil)).To(MatchError(ContainSubstring("wrong object type")))
		})
	})
})
