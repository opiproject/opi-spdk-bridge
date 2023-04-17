// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"log"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

const (
	virtioBlkDeviceType = "virtio-blk"
	nvmeDeviceType      = "NVMe"
)

type deviceLocation struct {
	Bus  *string
	Addr *string
}

type deviceLocator interface {
	Calculate(endpoint *pb.PciEndpoint) (deviceLocation, error)
}

func newDeviceLocator(buses []string, deviceType string) deviceLocator {
	if len(buses) == 0 {
		log.Println(deviceType, "location will be assigned by QEMU")
		return defaultDeviceLocator{}
	}
	return nil
}

type defaultDeviceLocator struct{}

func (defaultDeviceLocator) Calculate(_ *pb.PciEndpoint) (deviceLocation, error) {
	return deviceLocation{
		Bus:  nil,
		Addr: nil,
	}, nil
}
