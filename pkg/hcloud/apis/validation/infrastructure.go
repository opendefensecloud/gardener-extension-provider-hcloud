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
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

// ValidateInfrastructureConfig validates infrastructure config
func ValidateInfrastructureConfig(infraConfig *apis.InfrastructureConfig, nodes *string, pods *string, services *string) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func ValidateInfrastructureConfigUpdate(oldInfraConfig *apis.InfrastructureConfig, infraConfig *apis.InfrastructureConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	// The worker network configuration is immutable: changing the worker subnet
	// after the infrastructure has been created would orphan the existing network
	// and the nodes attached to it.
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(infraConfig.Networks, oldInfraConfig.Networks, field.NewPath("networks"))...)

	return allErrs
}

// ValidateInfrastructureConfigAgainstCloudProfile validates InfrastructureConfig against CloudProfile
func ValidateInfrastructureConfigAgainstCloudProfile(
	oldInfraConfig *apis.InfrastructureConfig,
	infraConfig *apis.InfrastructureConfig,
	shoot *core.Shoot,
	cloudProfile *gardencorev1beta1.CloudProfile,
	fldPath *field.Path) field.ErrorList {

	allErrs := field.ErrorList{}
	return allErrs
}

// ValidateInfrastructureConfigSpec validates provider specification to check if all fields are present and valid
//
// PARAMETERS
// spec *apis.InfrastructureConfig Provider specification to validate
func ValidateInfrastructureConfigSpec(spec *apis.InfrastructureConfig) []error {
	var allErrs []error

	if spec.Networks != nil && spec.Networks.WorkersConfiguration == nil && spec.Networks.Workers == "" {
		allErrs = append(allErrs, fmt.Errorf("networks.workersConfiguration or networks.workers is a required field"))
	}

	return allErrs
}
