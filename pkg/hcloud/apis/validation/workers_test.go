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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// spreadProviderConfig returns a RawExtension carrying a WorkerConfig whose
// placement group type is "spread".
func spreadProviderConfig() *runtime.RawExtension {
	return &runtime.RawExtension{
		Raw: []byte(`{
			"apiVersion": "hcloud.provider.extensions.gardener.cloud/v1alpha1",
			"kind": "WorkerConfig",
			"placementGroupType": "spread"
		}`),
	}
}

var _ = Describe("Workers validation", func() {
	var fldPath *field.Path

	BeforeEach(func() {
		fldPath = field.NewPath("workers")
	})

	Describe("#ValidateWorkers", func() {
		It("should accept a valid worker", func() {
			workers := []core.Worker{
				{
					Name:    "pool-1",
					Minimum: 1,
					Maximum: 3,
					Zones:   []string{"fsn1", "nbg1"},
				},
			}

			Expect(ValidateWorkers(workers, fldPath)).To(BeEmpty())
		})

		It("should require at least one zone", func() {
			workers := []core.Worker{
				{
					Name:    "pool-1",
					Minimum: 1,
					Maximum: 3,
					Zones:   nil,
				},
			}

			errList := ValidateWorkers(workers, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("workers[0].zones"),
				})),
			))
		})

		It("should forbid minimum 0 when maximum > 0", func() {
			workers := []core.Worker{
				{
					Name:    "pool-1",
					Minimum: 0,
					Maximum: 3,
					Zones:   []string{"fsn1"},
				},
			}

			errList := ValidateWorkers(workers, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("workers[0].minimum"),
				})),
			))
		})

		It("should reject a zone specified more than once per worker group", func() {
			workers := []core.Worker{
				{
					Name:    "pool-1",
					Minimum: 1,
					Maximum: 3,
					Zones:   []string{"fsn1", "fsn1"},
				},
			}

			errList := ValidateWorkers(workers, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("workers[0].zones[1]"),
					"BadValue": Equal("fsn1"),
				})),
			))
		})

		It("should accept a spread placement group within the size limit", func() {
			maxSurge := intstr.FromInt(2)
			workers := []core.Worker{
				{
					Name:           "pool-1",
					Minimum:        1,
					Maximum:        8,
					MaxSurge:       &maxSurge,
					Zones:          []string{"fsn1"},
					ProviderConfig: spreadProviderConfig(),
				},
			}

			Expect(ValidateWorkers(workers, fldPath)).To(BeEmpty())
		})

		It("should forbid a spread placement group larger than 10 - MaxSurge", func() {
			maxSurge := intstr.FromInt(3)
			workers := []core.Worker{
				{
					Name:           "pool-1",
					Minimum:        1,
					Maximum:        10,
					MaxSurge:       &maxSurge,
					Zones:          []string{"fsn1"},
					ProviderConfig: spreadProviderConfig(),
				},
			}

			errList := ValidateWorkers(workers, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("workers[0].maximum"),
				})),
			))
		})
	})

	Describe("#ValidateWorkersUpdate", func() {
		It("should accept an unchanged worker set", func() {
			workers := []core.Worker{
				{
					Name:  "pool-1",
					Zones: []string{"fsn1", "nbg1"},
				},
			}

			Expect(ValidateWorkersUpdate(workers, workers, fldPath)).To(BeEmpty())
		})

		It("should accept a newly added worker pool", func() {
			oldWorkers := []core.Worker{
				{Name: "pool-1", Zones: []string{"fsn1"}},
			}
			newWorkers := []core.Worker{
				{Name: "pool-1", Zones: []string{"fsn1"}},
				{Name: "pool-2", Zones: []string{"nbg1"}},
			}

			Expect(ValidateWorkersUpdate(oldWorkers, newWorkers, fldPath)).To(BeEmpty())
		})

		It("should reject changing the zones of an existing worker pool", func() {
			oldWorkers := []core.Worker{
				{Name: "pool-1", Zones: []string{"fsn1"}},
			}
			newWorkers := []core.Worker{
				{Name: "pool-1", Zones: []string{"nbg1"}},
			}

			errList := ValidateWorkersUpdate(oldWorkers, newWorkers, fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("workers[0].zones"),
				})),
			))
		})
	})
})
