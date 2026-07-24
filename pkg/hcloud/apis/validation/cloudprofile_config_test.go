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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

var _ = Describe("CloudProfileConfig validation", func() {
	Describe("#ValidateCloudProfileConfig", func() {
		It("should accept a populated cloud profile config", func() {
			profileSpec := &gardencorev1beta1.CloudProfileSpec{
				Regions: []gardencorev1beta1.Region{
					{
						Name: "hetzner",
						Zones: []gardencorev1beta1.AvailabilityZone{
							{Name: "fsn1"},
							{Name: "nbg1"},
						},
					},
				},
			}
			profileConfig := &apis.CloudProfileConfig{
				Regions: []apis.RegionSpec{
					{Name: "hetzner"},
				},
				MachineImages: []apis.MachineImages{
					{
						Name: "ubuntu",
						Versions: []apis.MachineImageVersion{
							{Version: "20.04", ImageName: "ubuntu-20.04"},
						},
					},
				},
				DefaultStorageFsType: "ext4",
			}

			Expect(ValidateCloudProfileConfig(profileSpec, profileConfig)).To(BeEmpty())
		})

		It("should require regions, machine images and default storage fs type", func() {
			errList := ValidateCloudProfileConfig(&gardencorev1beta1.CloudProfileSpec{}, &apis.CloudProfileConfig{})
			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.regions"),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.machineImages"),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.defaultStorageFsType"),
				})),
			))
		})

		It("should require a name and versions on each machine image", func() {
			profileConfig := &apis.CloudProfileConfig{
				Regions:              []apis.RegionSpec{{Name: "hetzner"}},
				DefaultStorageFsType: "ext4",
				MachineImages: []apis.MachineImages{
					{Name: "", Versions: nil},
				},
			}
			errList := ValidateCloudProfileConfig(&gardencorev1beta1.CloudProfileSpec{}, profileConfig)
			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.machineImages[0].name"),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.machineImages[0].versions"),
				})),
			))
		})
	})
})
