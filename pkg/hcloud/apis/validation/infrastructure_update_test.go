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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

var _ = Describe("InfrastructureConfig update validation", func() {
	var (
		oldConfig *apis.InfrastructureConfig
		newConfig *apis.InfrastructureConfig
	)

	BeforeEach(func() {
		oldConfig = &apis.InfrastructureConfig{
			FloatingPoolName: "pool-old",
			Networks: &apis.InfrastructureConfigNetworks{
				Workers: "10.250.0.0/16",
			},
		}
		newConfig = &apis.InfrastructureConfig{
			FloatingPoolName: "pool-new",
			Networks: &apis.InfrastructureConfigNetworks{
				Workers: "10.251.0.0/16",
			},
		}
	})

	Describe("#ValidateInfrastructureConfigUpdate", func() {
		It("should not return errors for an unchanged config", func() {
			Expect(ValidateInfrastructureConfigUpdate(oldConfig, oldConfig)).To(BeEmpty())
		})

		It("should reject a change to the immutable networks field", func() {
			// newConfig changes the worker subnet (10.250.0.0/16 -> 10.251.0.0/16).
			errList := ValidateInfrastructureConfigUpdate(oldConfig, newConfig)
			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("networks"),
				})),
			))
		})

		It("should accept a change to a non-immutable field such as floatingPoolName", func() {
			changed := oldConfig.DeepCopy()
			changed.FloatingPoolName = "pool-changed"
			Expect(ValidateInfrastructureConfigUpdate(oldConfig, changed)).To(BeEmpty())
		})
	})

	Describe("#ValidateInfrastructureConfigAgainstCloudProfile", func() {
		It("should not return errors for the given inputs", func() {
			shoot := &core.Shoot{}
			cloudProfile := &gardencorev1beta1.CloudProfile{}
			fldPath := field.NewPath("providerConfig")

			errList := ValidateInfrastructureConfigAgainstCloudProfile(oldConfig, newConfig, shoot, cloudProfile, fldPath)

			Expect(errList).To(BeEmpty())
		})
	})
})
