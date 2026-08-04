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
	"net"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis"
)

var (
	// privateIPv4Ranges are the ranges HCloud accepts as the IP range of a network.
	privateIPv4Ranges = []*net.IPNet{
		mustParseCIDR("10.0.0.0/8"),
		mustParseCIDR("172.16.0.0/12"),
		mustParseCIDR("192.168.0.0/16"),
	}

	// validNetworkZones are the network zones known to the HCloud client in use.
	validNetworkZones = sets.New(
		string(hcloud.NetworkZoneEUCentral),
		string(hcloud.NetworkZoneUSEast),
		string(hcloud.NetworkZoneUSWest),
		string(hcloud.NetworkZoneAPSouthEast),
	)

	// regionNetworkZones maps the HCloud locations to the network zone hosting them.
	// Regions missing from this map are not validated, so that locations added by
	// HCloud after this release are never rejected.
	regionNetworkZones = map[string]hcloud.NetworkZone{
		"fsn1": hcloud.NetworkZoneEUCentral,
		"nbg1": hcloud.NetworkZoneEUCentral,
		"hel1": hcloud.NetworkZoneEUCentral,
		"ash":  hcloud.NetworkZoneUSEast,
		"hil":  hcloud.NetworkZoneUSWest,
		"sin":  hcloud.NetworkZoneAPSouthEast,
	}
)

// ValidateInfrastructureConfig validates infrastructure config
//
// PARAMETERS
// infraConfig *apis.InfrastructureConfig Provider specification to validate
// nodes       *string                    Nodes CIDR of the shoot
// pods        *string                    Pods CIDR of the shoot
// services    *string                    Services CIDR of the shoot
// fldPath     *field.Path                Field path of the infrastructure config
func ValidateInfrastructureConfig(infraConfig *apis.InfrastructureConfig, nodes *string, pods *string, services *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if infraConfig == nil {
		return append(allErrs, field.Required(fldPath, "must provide an infrastructure configuration"))
	}

	networksPath := fldPath.Child("networks")
	if infraConfig.Networks == nil {
		return append(allErrs, field.Required(networksPath, "must provide a worker network configuration"))
	}

	workersCIDR := infraConfig.Networks.Workers
	workersPath := networksPath.Child("workers")

	if workersConfiguration := infraConfig.Networks.WorkersConfiguration; workersConfiguration != nil {
		workersCIDR = workersConfiguration.Cidr
		workersPath = networksPath.Child("workersConfiguration", "cidr")

		// An empty zone is resolved from the region by the infrastructure controller.
		if zone := workersConfiguration.Zone; zone != "" && !validNetworkZones.Has(string(zone)) {
			allErrs = append(allErrs, field.NotSupported(networksPath.Child("workersConfiguration", "zone"), zone, sets.List(validNetworkZones)))
		}
	}

	if workersCIDR == "" {
		return append(allErrs, field.Required(workersPath, "must provide the CIDR of the worker network"))
	}

	workerCIDR := cidrvalidation.NewCIDR(workersCIDR, workersPath)
	if !workerCIDR.Parse() {
		// The infrastructure controller passes the CIDR to HCloud unchecked, so anything
		// unparseable has to be rejected here.
		return append(allErrs, workerCIDR.ValidateParse()...)
	}

	allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(workersPath, workersCIDR)...)
	allErrs = append(allErrs, workerCIDR.ValidateIPFamily(cidrvalidation.IPFamilyIPv4)...)
	allErrs = append(allErrs, validatePrivateIPv4Range(workerCIDR)...)

	// The shoot networks themselves are validated by Gardener, only their relation to
	// the worker network is checked here.
	networkingPath := field.NewPath("spec", "networking")

	if isSet(nodes) {
		if nodeCIDR := cidrvalidation.NewCIDR(*nodes, networkingPath.Child("nodes")); nodeCIDR.Parse() {
			allErrs = append(allErrs, nodeCIDR.ValidateSubset(workerCIDR)...)
		}
	}

	for name, shootCIDR := range map[string]*string{"pods": pods, "services": services} {
		if !isSet(shootCIDR) {
			continue
		}

		if otherCIDR := cidrvalidation.NewCIDR(*shootCIDR, networkingPath.Child(name)); otherCIDR.Parse() {
			allErrs = append(allErrs, workerCIDR.ValidateNotOverlap(otherCIDR)...)
		}
	}

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
//
// A network subnet lives in exactly one network zone and only servers located in that
// zone can attach to it, so an explicitly configured zone has to match the region the
// shoot is scheduled in.
func ValidateInfrastructureConfigAgainstCloudProfile(
	oldInfraConfig *apis.InfrastructureConfig,
	infraConfig *apis.InfrastructureConfig,
	shoot *core.Shoot,
	_ *gardencorev1beta1.CloudProfile,
	fldPath *field.Path) field.ErrorList {

	allErrs := field.ErrorList{}

	zone := networkZoneOf(infraConfig)
	if zone == "" || shoot == nil {
		return allErrs
	}

	// Only enforce this on a change of the zone, so that shoots created before this
	// validation existed can still be updated.
	if networkZoneOf(oldInfraConfig) == zone {
		return allErrs
	}

	expectedZone, known := regionNetworkZones[shoot.Spec.Region]
	if !known {
		return allErrs
	}

	if zone != expectedZone {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networks", "workersConfiguration", "zone"), zone,
			fmt.Sprintf("region %q is hosted in network zone %q, servers cannot attach to a network of another zone", shoot.Spec.Region, expectedZone)))
	}

	return allErrs
}

// networkZoneOf returns the network zone configured for the worker network, if any.
//
// PARAMETERS
// infraConfig *apis.InfrastructureConfig Provider specification to read
func networkZoneOf(infraConfig *apis.InfrastructureConfig) hcloud.NetworkZone {
	if infraConfig == nil || infraConfig.Networks == nil || infraConfig.Networks.WorkersConfiguration == nil {
		return ""
	}

	return infraConfig.Networks.WorkersConfiguration.Zone
}

// validatePrivateIPv4Range validates that the given CIDR is one HCloud accepts for a network.
//
// PARAMETERS
// workerCIDR cidrvalidation.CIDR Worker network CIDR to validate
func validatePrivateIPv4Range(workerCIDR cidrvalidation.CIDR) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, privateRange := range privateIPv4Ranges {
		if privateRange.Contains(workerCIDR.GetIPNet().IP) {
			return allErrs
		}
	}

	return append(allErrs, field.Invalid(workerCIDR.GetFieldPath(), workerCIDR.GetCIDR(),
		"must be within one of the private IPv4 ranges 10.0.0.0/8, 172.16.0.0/12 or 192.168.0.0/16"))
}

// mustParseCIDR parses a CIDR known to be valid at build time.
//
// PARAMETERS
// cidr string CIDR to parse
func mustParseCIDR(cidr string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}

	return ipNet
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
