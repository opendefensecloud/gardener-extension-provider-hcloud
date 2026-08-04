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

// Package validation contains functions to validate controller specifications
package validation

import (
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

var _ = Describe("InfrastructureConfig cloud profile validation", func() {
	var (
		fldPath      = field.NewPath("providerConfig")
		cloudProfile = &gardencorev1beta1.CloudProfile{}

		shoot       *core.Shoot
		infraConfig *apis.InfrastructureConfig
	)

	newConfigWithZone := func(zone hcloud.NetworkZone) *apis.InfrastructureConfig {
		return &apis.InfrastructureConfig{
			Networks: &apis.InfrastructureConfigNetworks{
				WorkersConfiguration: &apis.InfrastructureConfigNetwork{
					Cidr: "10.250.0.0/16",
					Zone: zone,
				},
			},
		}
	}

	BeforeEach(func() {
		shoot = &core.Shoot{Spec: core.ShootSpec{Region: "fsn1"}}
		infraConfig = newConfigWithZone(hcloud.NetworkZoneEUCentral)
	})

	Describe("#ValidateInfrastructureConfigAgainstCloudProfile", func() {
		It("should accept a network zone matching the region", func() {
			Expect(ValidateInfrastructureConfigAgainstCloudProfile(nil, infraConfig, shoot, cloudProfile, fldPath)).To(BeEmpty())
		})

		It("should reject a network zone that does not host the region", func() {
			infraConfig = newConfigWithZone(hcloud.NetworkZoneUSEast)

			errList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infraConfig, shoot, cloudProfile, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("providerConfig.networks.workersConfiguration.zone"),
					"Detail": ContainSubstring(string(hcloud.NetworkZoneEUCentral)),
				})),
			))
		})

		It("should accept an unknown region so that new Hetzner locations are not blocked", func() {
			shoot.Spec.Region = "xyz1"
			infraConfig = newConfigWithZone(hcloud.NetworkZoneUSEast)

			Expect(ValidateInfrastructureConfigAgainstCloudProfile(nil, infraConfig, shoot, cloudProfile, fldPath)).To(BeEmpty())
		})

		It("should tolerate an already persisted mismatch as long as the zone does not change", func() {
			infraConfig = newConfigWithZone(hcloud.NetworkZoneUSEast)
			oldInfraConfig := newConfigWithZone(hcloud.NetworkZoneUSEast)

			Expect(ValidateInfrastructureConfigAgainstCloudProfile(oldInfraConfig, infraConfig, shoot, cloudProfile, fldPath)).To(BeEmpty())
		})

		It("should reject a changed zone that does not host the region", func() {
			oldInfraConfig := newConfigWithZone(hcloud.NetworkZoneEUCentral)
			infraConfig = newConfigWithZone(hcloud.NetworkZoneUSEast)

			Expect(ValidateInfrastructureConfigAgainstCloudProfile(oldInfraConfig, infraConfig, shoot, cloudProfile, fldPath)).To(HaveLen(1))
		})

		It("should accept an empty network zone, which is resolved from the region at reconcile time", func() {
			infraConfig = newConfigWithZone("")

			Expect(ValidateInfrastructureConfigAgainstCloudProfile(nil, infraConfig, shoot, cloudProfile, fldPath)).To(BeEmpty())
		})

		It("should accept a config without a workersConfiguration", func() {
			infraConfig = &apis.InfrastructureConfig{
				Networks: &apis.InfrastructureConfigNetworks{Workers: "10.250.0.0/16"},
			}

			Expect(ValidateInfrastructureConfigAgainstCloudProfile(nil, infraConfig, shoot, cloudProfile, fldPath)).To(BeEmpty())
		})

		It("should accept an empty config", func() {
			Expect(ValidateInfrastructureConfigAgainstCloudProfile(nil, nil, shoot, cloudProfile, fldPath)).To(BeEmpty())
		})
	})
})
