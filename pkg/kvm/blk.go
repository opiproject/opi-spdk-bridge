// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"log"
	"path/filepath"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateVirtioBlk creates a virtio-blk device and attaches it to QEMU instance
func (s *Server) CreateVirtioBlk(ctx context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	if in.VirtioBlk.PcieId == nil {
		log.Println("Pci endpoint should be specified")
		return nil, errNoPcieEndpoint
	}

	location, err := s.locator.Calculate(in.VirtioBlk.PcieId)
	if err != nil {
		log.Println("Failed to calculate device location:", err)
		return nil, errDeviceEndpoint
	}

	out, err := s.Server.CreateVirtioBlk(ctx, in)
	if err != nil {
		log.Println("Error running cmd on opi-spdk bridge:", err)
		return out, err
	}

	mon, err := newMonitor(s.qmpAddress, s.protocol, s.timeout, s.pollDevicePresenceStep)
	if err != nil {
		log.Println("Couldn't create QEMU monitor")
		_, _ = s.Server.DeleteVirtioBlk(context.Background(), &pb.DeleteVirtioBlkRequest{Name: out.Name})
		return nil, errMonitorCreation
	}
	defer mon.Disconnect()

	ctrlr := filepath.Join(s.ctrlrDir, out.Name)
	qemuChardevID := toQemuID(out.Name)
	if err := mon.AddChardev(qemuChardevID, ctrlr); err != nil {
		log.Println("Couldn't add chardev:", err)
		_, _ = s.Server.DeleteVirtioBlk(context.Background(), &pb.DeleteVirtioBlkRequest{Name: out.Name})
		return nil, errAddChardevFailed
	}

	qemuDevID := toQemuID(out.Name)
	if err = mon.AddVirtioBlkDevice(qemuDevID, qemuChardevID, location); err != nil {
		log.Println("Couldn't add device:", err)
		_ = mon.DeleteChardev(qemuDevID)
		_, _ = s.Server.DeleteVirtioBlk(context.Background(), &pb.DeleteVirtioBlkRequest{Name: out.Name})
		return nil, errAddDeviceFailed
	}

	return out, nil
}

// DeleteVirtioBlk deletes a virtio-blk device and detaches it from QEMU instance
func (s *Server) DeleteVirtioBlk(ctx context.Context, in *pb.DeleteVirtioBlkRequest) (*emptypb.Empty, error) {
	mon, monErr := newMonitor(s.qmpAddress, s.protocol, s.timeout, s.pollDevicePresenceStep)
	if monErr != nil {
		log.Println("Couldn't create QEMU monitor")
		return nil, errMonitorCreation
	}
	defer mon.Disconnect()

	qemuDeviceID := toQemuID(in.Name)
	delDevErr := mon.DeleteVirtioBlkDevice(qemuDeviceID)
	if delDevErr != nil {
		log.Printf("Couldn't delete virtio-blk: %v", delDevErr)
	}

	qemuChardevID := toQemuID(in.Name)
	delChardevErr := mon.DeleteChardev(qemuChardevID)
	if delChardevErr != nil {
		log.Printf("Couldn't delete chardev for virtio-blk: %v. Device is partially deleted", delChardevErr)
	}

	response, spdkErr := s.Server.DeleteVirtioBlk(ctx, in)
	if spdkErr != nil {
		log.Println("Error running underlying cmd on opi-spdk bridge:", spdkErr)
	}

	var err error
	if delDevErr != nil && delChardevErr != nil && spdkErr != nil {
		err = errDeviceNotDeleted
	} else if delDevErr != nil || delChardevErr != nil || spdkErr != nil {
		err = errDevicePartiallyDeleted
	}

	return response, err
}
