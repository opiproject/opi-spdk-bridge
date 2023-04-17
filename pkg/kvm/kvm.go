// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	errNoController           = status.Error(codes.NotFound, "no controller found")
	errInvalidSubsystem       = status.Error(codes.InvalidArgument, "invalid subsystem")
	errDevicePartiallyDeleted = status.Error(codes.Internal, "device is partially deleted")
	errFailedToCreateNvmeDir  = status.Error(codes.FailedPrecondition, "cannot create directory for Nvme controller")
	errDeviceEndpoint         = status.Error(codes.InvalidArgument, "values in endpoint cannot be used to calculate device location")
	errNoPcieEndpoint         = status.Error(codes.InvalidArgument, "no pcie endpoint provided")
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

	nvmeDeviceLocator      deviceLocator
	virtioBlkDeviceLocator deviceLocator
}

// NewServer creates instance of KvmServer
func NewServer(s *frontend.Server, qmpAddress string, ctrlrDir string, nvmeBuses []string, virtioBlkBuses []string) *Server {
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
	return &Server{s, qmpAddress, ctrlrDir, qmpProtocol, timeout, pollDevicePresenceStep,
		newDeviceLocator(nvmeBuses, nvmeDeviceType), newDeviceLocator(virtioBlkBuses, virtioBlkDeviceType)}
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

func toQemuID(name string) string {
	// qemu id cannot start with numbers. Add prefix
	return "opi-" + name
}
