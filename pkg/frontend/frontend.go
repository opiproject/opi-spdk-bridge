// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"log"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

// SubsystemListener interface is used to provide SPDK call params to create/delete
// NVMe controllers depending on used transport type.
type SubsystemListener interface {
	Params(ctrlr *pb.NVMeController, nqn string) models.NvmfSubsystemAddListenerParams
}

// NvmeParameters contains all NVMe related structures
type NvmeParameters struct {
	Subsystems     map[string]*pb.NVMeSubsystem
	Controllers    map[string]*pb.NVMeController
	Namespaces     map[string]*pb.NVMeNamespace
	subsysListener SubsystemListener
}

// VirtioParameters contains all VirtIO related structures
type VirtioParameters struct {
	BlkCtrls  map[string]*pb.VirtioBlk
	ScsiCtrls map[string]*pb.VirtioScsiController
	ScsiLuns  map[string]*pb.VirtioScsiLun
}

// Server contains frontend related OPI services
type Server struct {
	pb.UnimplementedFrontendNvmeServiceServer
	pb.UnimplementedFrontendVirtioBlkServiceServer
	pb.UnimplementedFrontendVirtioScsiServiceServer

	rpc  server.JSONRPC
	Nvme NvmeParameters
	Virt VirtioParameters
}

// NewServer creates initialized instance of FrontEnd server communicating
// with provided jsonRPC
func NewServer(jsonRPC server.JSONRPC) *Server {
	return &Server{
		rpc: jsonRPC,
		Nvme: NvmeParameters{
			Subsystems:     make(map[string]*pb.NVMeSubsystem),
			Controllers:    make(map[string]*pb.NVMeController),
			Namespaces:     make(map[string]*pb.NVMeNamespace),
			subsysListener: NewTCPSubsystemListener("127.0.0.1:4420"),
		},
		Virt: VirtioParameters{
			BlkCtrls:  make(map[string]*pb.VirtioBlk),
			ScsiCtrls: make(map[string]*pb.VirtioScsiController),
			ScsiLuns:  make(map[string]*pb.VirtioScsiLun),
		},
	}
}

// NewServerWithSubsystemListener creates initialized instance of FrontEnd server communicating
// with provided jsonRPC and externally created SubsystemListener instead default one.
func NewServerWithSubsystemListener(jsonRPC server.JSONRPC, sysListener SubsystemListener) *Server {
	if sysListener == nil {
		log.Panic("nil for SubsystemListener is not allowed")
	}
	server := NewServer(jsonRPC)
	server.Nvme.subsysListener = sysListener
	return server
}
