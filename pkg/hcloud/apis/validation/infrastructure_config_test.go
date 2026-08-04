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
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

var _ = Describe("InfrastructureConfig validation", func() {
	var (
		fldPath = field.NewPath("providerConfig")

		nodes    *string
		pods     *string
		services *string

		infraConfig *apis.InfrastructureConfig
	)

	BeforeEach(func() {
		nodes = ptr.To("10.250.0.0/16")
		pods = ptr.To("100.96.0.0/11")
		services = ptr.To("100.64.0.0/13")

		infraConfig = &apis.InfrastructureConfig{
			FloatingPoolName: "pool",
			Networks: &apis.InfrastructureConfigNetworks{
				Workers: "10.250.0.0/16",
			},
		}
	})

	Describe("#ValidateInfrastructureConfig", func() {
		It("should accept a worker network matching the nodes CIDR", func() {
			Expect(ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)).To(BeEmpty())
		})

		It("should accept a worker network that is a subset of the nodes CIDR", func() {
			infraConfig.Networks.Workers = "10.250.16.0/20"

			Expect(ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)).To(BeEmpty())
		})

		It("should accept a workersConfiguration with a known network zone", func() {
			infraConfig.Networks = &apis.InfrastructureConfigNetworks{
				WorkersConfiguration: &apis.InfrastructureConfigNetwork{
					Cidr: "10.250.0.0/16",
					Zone: hcloud.NetworkZoneEUCentral,
				},
			}

			Expect(ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)).To(BeEmpty())
		})

		It("should accept a workersConfiguration without a network zone, which is resolved from the region", func() {
			infraConfig.Networks = &apis.InfrastructureConfigNetworks{
				WorkersConfiguration: &apis.InfrastructureConfigNetwork{
					Cidr: "10.250.0.0/16",
				},
			}

			Expect(ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)).To(BeEmpty())
		})

		It("should not require the nodes, pods and services CIDRs to be set", func() {
			Expect(ValidateInfrastructureConfig(infraConfig, nil, nil, nil, fldPath)).To(BeEmpty())
		})

		It("should require an infrastructure config", func() {
			errList := ValidateInfrastructureConfig(nil, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig"),
				})),
			))
		})

		It("should require a networks section", func() {
			infraConfig.Networks = nil

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.networks"),
				})),
			))
		})

		It("should require a worker CIDR", func() {
			infraConfig.Networks = &apis.InfrastructureConfigNetworks{}

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.networks.workers"),
				})),
			))
		})

		It("should require a worker CIDR in the workersConfiguration", func() {
			infraConfig.Networks = &apis.InfrastructureConfigNetworks{
				WorkersConfiguration: &apis.InfrastructureConfigNetwork{
					Zone: hcloud.NetworkZoneEUCentral,
				},
			}

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.networks.workersConfiguration.cidr"),
				})),
			))
		})

		It("should reject a worker CIDR that cannot be parsed", func() {
			infraConfig.Networks.Workers = "10.250.0.0"

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("providerConfig.networks.workers"),
				})),
			))
		})

		It("should reject a worker CIDR that is not in canonical form", func() {
			infraConfig.Networks.Workers = "10.250.0.5/16"

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ContainElement(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("providerConfig.networks.workers"),
				})),
			))
		})

		It("should reject a worker CIDR outside the private IPv4 ranges", func() {
			infraConfig.Networks.Workers = "1.2.3.0/24"
			nodes = ptr.To("1.2.3.0/24")

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("providerConfig.networks.workers"),
					"Detail": ContainSubstring("private IPv4 range"),
				})),
			))
		})

		It("should reject an IPv6 worker CIDR", func() {
			infraConfig.Networks.Workers = "fd00::/64"

			errList := ValidateInfrastructureConfig(infraConfig, nil, nil, nil, fldPath)

			Expect(errList).To(ContainElement(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("providerConfig.networks.workers"),
				})),
			))
		})

		It("should reject a worker CIDR that is not a subset of the nodes CIDR", func() {
			infraConfig.Networks.Workers = "10.251.0.0/16"

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("providerConfig.networks.workers"),
					"Detail": ContainSubstring("must be a subset of"),
				})),
			))
		})

		It("should reject a worker CIDR overlapping the pods CIDR", func() {
			infraConfig.Networks.Workers = "10.250.0.0/16"
			nodes = ptr.To("10.250.0.0/16")
			pods = ptr.To("10.250.128.0/17")

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ContainElement(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Detail": ContainSubstring("must not overlap"),
				})),
			))
		})

		It("should reject a worker CIDR overlapping the services CIDR", func() {
			infraConfig.Networks.Workers = "10.250.0.0/16"
			nodes = ptr.To("10.250.0.0/16")
			services = ptr.To("10.250.128.0/17")

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ContainElement(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Detail": ContainSubstring("must not overlap"),
				})),
			))
		})

		It("should reject an unknown network zone", func() {
			infraConfig.Networks = &apis.InfrastructureConfigNetworks{
				WorkersConfiguration: &apis.InfrastructureConfigNetwork{
					Cidr: "10.250.0.0/16",
					Zone: "eu-west",
				},
			}

			errList := ValidateInfrastructureConfig(infraConfig, nodes, pods, services, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("providerConfig.networks.workersConfiguration.zone"),
				})),
			))
		})

		It("should not validate the nodes CIDR itself", func() {
			// The nodes CIDR is validated by ValidateShootNetworking; a broken one must
			// not produce a second error here.
			infraConfig.Networks.Workers = "10.250.0.0/16"

			Expect(ValidateInfrastructureConfig(infraConfig, ptr.To("not-a-cidr"), nil, nil, fldPath)).To(BeEmpty())
		})
	})
})
