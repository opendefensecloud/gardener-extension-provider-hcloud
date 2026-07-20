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
	"regexp"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

var validLoadBalancerSizeValues = sets.NewString("SMALL", "MEDIUM", "LARGE")
var namePrefixPattern = regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(_ *gardencorev1beta1.CloudProfileSpec, profileConfig *apis.CloudProfileConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	providerConfigPath := field.NewPath("providerConfig")

	if len(profileConfig.Regions) == 0 {
		allErrs = append(allErrs, field.Required(providerConfigPath.Child("regions"), "must provide at least one region"))
	}

	machineImagesPath := providerConfigPath.Child("machineImages")
	if len(profileConfig.MachineImages) == 0 {
		allErrs = append(allErrs, field.Required(machineImagesPath, "must provide at least one machine image"))
	}
	for i, machineImage := range profileConfig.MachineImages {
		idxPath := machineImagesPath.Index(i)
		if len(machineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}
		if len(machineImage.Versions) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("versions"), "must provide at least one version"))
		}
		for j, version := range machineImage.Versions {
			if len(version.Version) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("versions").Index(j).Child("version"), "must provide a version"))
			}
		}
	}

	if len(profileConfig.DefaultStorageFsType) == 0 {
		allErrs = append(allErrs, field.Required(providerConfigPath.Child("defaultStorageFsType"), "must provide a default storage filesystem type"))
	}

	return allErrs
}

func isSet(s *string) bool {
	return s != nil && *s != ""
}
