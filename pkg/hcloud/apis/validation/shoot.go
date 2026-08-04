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
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateShootNetworking validates the networking section for a shoot
//
// PARAMETERS
// networking *core.Networking Networking section of the shoot
// fldPath    *field.Path      Field path of the networking section
func ValidateShootNetworking(networking *core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	nodesPath := fldPath.Child("nodes")

	// The worker nodes get their private addresses from the HCloud network created by
	// the infrastructure controller, so the nodes CIDR is what that network is carved
	// out of and cannot be omitted.
	if networking == nil || !isSet(networking.Nodes) {
		allErrs = append(allErrs, field.Required(nodesPath, "must provide a nodes CIDR: the HCloud worker network is created from it"))
		return allErrs
	}

	allErrs = append(allErrs, cidrvalidation.NewCIDR(*networking.Nodes, nodesPath).ValidateParse()...)

	return allErrs
}
