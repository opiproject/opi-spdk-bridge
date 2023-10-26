// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package utils contails useful helper functions
package utils

import (
	"strings"

	"go.einride.tech/aip/resourcename"
)

// ResourceIDToVolumeName creates name of volume resource based on ID
func ResourceIDToVolumeName(resourceID string) string {
	return resourcename.Join(
		"//storage.opiproject.org/",
		"volumes", resourceID,
	)
}

// ResourceIDToSubsystemName transforms subsystem resource ID to subsystem name
func ResourceIDToSubsystemName(resourceID string) string {
	return resourcename.Join(
		"//storage.opiproject.org/",
		"subsystems", resourceID,
	)
}

// ResourceIDToNamespaceName transforms subsystem resource ID and namespace
// resource ID to namespace name
func ResourceIDToNamespaceName(subsysResourceID, ctrlrResourceID string) string {
	return resourcename.Join(
		"//storage.opiproject.org/",
		"subsystems", subsysResourceID,
		"namespaces", ctrlrResourceID,
	)
}

// ResourceIDToControllerName transforms subsystem resource ID and controller
// resource ID to controller name
func ResourceIDToControllerName(subsysResourceID, ctrlrResourceID string) string {
	return resourcename.Join(
		"//storage.opiproject.org/",
		"subsystems", subsysResourceID,
		"controllers", ctrlrResourceID,
	)
}

// GetSubsystemIDFromNvmeName get parent ID (subsystem ID) from nvme related names
func GetSubsystemIDFromNvmeName(name string) string {
	segments := strings.Split(name, "/")
	for i := range segments {
		if (i + 1) == len(segments) {
			return ""
		}

		if segments[i] == "subsystems" {
			return segments[i+1]
		}
	}

	return ""
}
