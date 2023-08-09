// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"fmt"
	"log"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

type deviceLocation struct {
	Bus  *string
	Addr *string
}

type deviceLocator interface {
	Calculate(endpoint *pb.PciEndpoint) (deviceLocation, error)
}

func newDeviceLocator(buses []string) deviceLocator {
	if len(buses) == 0 {
		log.Println("Device location for virtio-blk and Nvme devices will be assigned by QEMU")
		return defaultDeviceLocator{}
	}
	elementSet := make(map[string]struct{})
	for _, bus := range buses {
		if bus == "" {
			log.Panicln("Empty bus name cannot be used in", buses)
		}
		if _, ok := elementSet[bus]; ok {
			log.Panicln("Duplicated bus", bus)
		}
		elementSet[bus] = struct{}{}
	}
	log.Println("Device location will be calculated based on requested PcieEndpoint on", buses)
	return busDeviceLocator{buses}
}

type defaultDeviceLocator struct{}

func (defaultDeviceLocator) Calculate(_ *pb.PciEndpoint) (deviceLocation, error) {
	return deviceLocation{
		Bus:  nil,
		Addr: nil,
	}, nil
}

type busDeviceLocator struct {
	buses []string
}

func (l busDeviceLocator) Calculate(endpoint *pb.PciEndpoint) (deviceLocation, error) {
	if endpoint == nil {
		return deviceLocation{}, fmt.Errorf("pci endpoint is required to calculate device location")
	}
	bus, addr, err := l.calculateBusAddr(endpoint.PhysicalFunction.GetValue())
	if err != nil {
		return deviceLocation{}, err
	}

	addrInHex := fmt.Sprintf("%#x", addr)
	return deviceLocation{
		Bus:  &bus,
		Addr: &addrInHex,
	}, nil
}

func (l busDeviceLocator) calculateBusAddr(physicalFunction int32) (bus string, addr uint32, err error) {
	if physicalFunction < 0 {
		err = fmt.Errorf("physical function cannot be negative")
		return
	}

	var maxDevicesOnBus int32 = 32
	for _, qemuBus := range l.buses {
		if physicalFunction >= maxDevicesOnBus {
			physicalFunction -= maxDevicesOnBus
		} else {
			addr = uint32(physicalFunction)
			bus = qemuBus
			return
		}
	}
	err = fmt.Errorf("no corresponding bus found for physical function: %v", physicalFunction)
	return
}
