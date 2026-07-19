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
	"k8s.io/utils/ptr"
)

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateShootNetworking", func() {
		It("should accept a fully specified networking section", func() {
			networking := core.Networking{
				Type:     ptr.To("calico"),
				Nodes:    ptr.To("10.250.0.0/16"),
				Pods:     ptr.To("100.96.0.0/11"),
				Services: ptr.To("100.64.0.0/13"),
			}

			Expect(ValidateShootNetworking(networking)).To(BeEmpty())
		})

		It("should accept an empty networking section", func() {
			Expect(ValidateShootNetworking(core.Networking{})).To(BeEmpty())
		})
	})
})
