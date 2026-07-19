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

		It("should accept an empty cloud profile config", func() {
			Expect(ValidateCloudProfileConfig(&gardencorev1beta1.CloudProfileSpec{}, &apis.CloudProfileConfig{})).To(BeEmpty())
		})
	})
})
