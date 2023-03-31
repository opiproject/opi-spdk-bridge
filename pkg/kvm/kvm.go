// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

const (
	tcpProtocol        = "tcp"
	unixSocketProtocol = "unix"
)

var (
	errAddChardevFailed       = status.Error(codes.FailedPrecondition, "couldn't add chardev")
	errMonitorCreation        = status.Error(codes.Internal, "failed to create QEMU monitor")
	errAddDeviceFailed        = status.Error(codes.FailedPrecondition, "couldn't add device")
	errDeviceNotDeleted       = status.Error(codes.FailedPrecondition, "device is not deleted")
	errDevicePartiallyDeleted = status.Error(codes.Internal, "device is partially deleted")
	errFailedToCreateNvmeDir  = status.Error(codes.FailedPrecondition, "cannot create directory for NVMe controller")
)

// Server is a wrapper for default opi-spdk-bridge frontend which automates
// interaction with QEMU instance to plug/unplug SPDK devices
type Server struct {
	*frontend.Server

	qmpAddress string
	ctrlrDir   string
	protocol   string

	timeout                time.Duration
	pollDevicePresenceStep time.Duration
}

// NewServer creates instance of KvmServer
func NewServer(s *frontend.Server, qmpAddress string, ctrlrDir string) *Server {
	if s == nil {
		log.Fatalf("Frontend Server cannot be nil")
	}

	if qmpAddress == "" {
		log.Fatalf("qmpAddress cannot be empty")
	}

	if ctrlrDir == "" {
		log.Fatalf("ctrlrDir cannot be empty")
	}

	qmpProtocol, err := getProtocol(qmpAddress)
	if err != nil {
		log.Fatalf(err.Error())
	}

	timeout := 2 * time.Second
	pollDevicePresenceStep := 5 * time.Millisecond
	return &Server{s, qmpAddress, ctrlrDir, qmpProtocol, timeout, pollDevicePresenceStep}
}

// CreateVirtioBlk creates a virtio-blk device and attaches it to QEMU instance
func (s *Server) CreateVirtioBlk(ctx context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	out, err := s.Server.CreateVirtioBlk(ctx, in)
	if err != nil {
		log.Println("Error running cmd on opi-spdk bridge:", err)
		return out, err
	}

	id := out.Id.Value
	mon, err := newMonitor(s.qmpAddress, s.protocol, s.timeout, s.pollDevicePresenceStep)
	if err != nil {
		log.Println("Couldn't create QEMU monitor")
		_, _ = s.Server.DeleteVirtioBlk(context.Background(), &pb.DeleteVirtioBlkRequest{Name: id})
		return nil, errMonitorCreation
	}
	defer mon.Disconnect()

	ctrlr := filepath.Join(s.ctrlrDir, id)
	chardevID := out.Id.Value
	if err := mon.AddChardev(chardevID, ctrlr); err != nil {
		log.Println("Couldn't add chardev:", err)
		_, _ = s.Server.DeleteVirtioBlk(context.Background(), &pb.DeleteVirtioBlkRequest{Name: id})
		return nil, errAddChardevFailed
	}

	if err = mon.AddVirtioBlkDevice(id, id); err != nil {
		log.Println("Couldn't add device:", err)
		_ = mon.DeleteChardev(id)
		_, _ = s.Server.DeleteVirtioBlk(context.Background(), &pb.DeleteVirtioBlkRequest{Name: id})
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

	id := in.Name
	delDevErr := mon.DeleteVirtioBlkDevice(id)
	if delDevErr != nil {
		log.Printf("Couldn't delete virtio-blk: %v", delDevErr)
	}

	delChardevErr := mon.DeleteChardev(id)
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

func getProtocol(qmpAddress string) (string, error) {
	if isUnixSocketPath(qmpAddress) {
		return unixSocketProtocol, nil
	} else if isTCPAddress(qmpAddress) {
		return tcpProtocol, nil
	}
	return "", fmt.Errorf("unknown protocol for %v", qmpAddress)
}

func isUnixSocketPath(qmpAddress string) bool {
	fileInfo, err := os.Stat(qmpAddress)
	if os.IsNotExist(err) {
		return false
	} else if fileInfo.IsDir() {
		return false
	}
	return true
}

func isTCPAddress(qmpAddress string) bool {
	_, _, err := net.SplitHostPort(qmpAddress)
	return err == nil
}
