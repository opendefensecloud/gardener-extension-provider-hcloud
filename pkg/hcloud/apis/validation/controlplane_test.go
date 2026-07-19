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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

var _ = Describe("ControlPlaneConfig validation", func() {
	Describe("#ValidateControlPlaneConfig", func() {
		var (
			allowedZones sets.Set[string]
			workerZones  sets.Set[string]
			fldPath      *field.Path
		)

		BeforeEach(func() {
			allowedZones = sets.New[string]("fsn1", "nbg1", "hel1")
			workerZones = sets.New[string]("fsn1", "nbg1")
			fldPath = field.NewPath("controlPlaneConfig")
		})

		It("should accept a zone that is part of a worker zone", func() {
			cfg := &apis.ControlPlaneConfig{Zone: "fsn1"}

			errList := ValidateControlPlaneConfig(cfg, allowedZones, workerZones, "1.28.0", fldPath)

			Expect(errList).To(BeEmpty())
		})

		It("should require a zone when it is empty", func() {
			cfg := &apis.ControlPlaneConfig{Zone: ""}

			errList := ValidateControlPlaneConfig(cfg, allowedZones, workerZones, "1.28.0", fldPath)

			// An empty zone triggers both the required check and the worker-zone
			// membership check (the empty string is not part of any worker zone).
			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controlPlaneConfig.zone"),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controlPlaneConfig.zone"),
				})),
			))
		})

		It("should reject a zone that is not part of any worker zone", func() {
			cfg := &apis.ControlPlaneConfig{Zone: "hel1"}

			errList := ValidateControlPlaneConfig(cfg, allowedZones, workerZones, "1.28.0", fldPath)

			Expect(errList).To(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("controlPlaneConfig.zone"),
					"BadValue": Equal("hel1"),
				})),
			))
		})
	})
})
