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
)

// Server is a wrapper for default opi-spdk-bridge frontend which automates
// interaction with QEMU instance to plug/unplug SPDK devices
type Server struct {
	*frontend.Server

	qmpAddress string
	ctrlrDir   string
	protocol   string
	timeout    time.Duration
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
	return &Server{s, qmpAddress, ctrlrDir, qmpProtocol, timeout}
}

// CreateVirtioBlk creates a virtio-blk device and attaches it to QEMU instance
func (s *Server) CreateVirtioBlk(ctx context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	out, err := s.Server.CreateVirtioBlk(ctx, in)
	if err != nil {
		log.Println("Error running cmd on opi-spdk bridge:", err)
		return out, err
	}

	id := out.Id.Value
	mon, err := newMonitor(s.qmpAddress, s.protocol, s.timeout)
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
	ctrlr := filepath.Join(s.ctrlrDir, in.Name)
	log.Printf("Unplugging virtio-blk %v from qemu over %v", ctrlr, s.qmpAddress)
	log.Println("virtio-blk is unplugged.")
	return s.Server.DeleteVirtioBlk(ctx, in)
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
