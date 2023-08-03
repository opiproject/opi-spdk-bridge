// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"log"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// SubsystemListener interface is used to provide SPDK call params to create/delete
// Nvme controllers depending on used transport type.
type SubsystemListener interface {
	Params(ctrlr *pb.NvmeController, nqn string) spdk.NvmfSubsystemAddListenerParams
}

// NvmeParameters contains all Nvme related structures
type NvmeParameters struct {
	Subsystems     map[string]*pb.NvmeSubsystem
	Controllers    map[string]*pb.NvmeController
	Namespaces     map[string]*pb.NvmeNamespace
	subsysListener SubsystemListener
}

// VirtioBlkTransport interface is used to provide SPDK call params to create/delete
// virtio-blk controllers depending on used transport type.
type VirtioBlkTransport interface {
	CreateParams(virtioBlk *pb.VirtioBlk) (any, error)
	DeleteParams(virtioBlk *pb.VirtioBlk) (any, error)
}

// VirtioParameters contains all VirtIO related structures
type VirtioParameters struct {
	BlkCtrls  map[string]*pb.VirtioBlk
	ScsiCtrls map[string]*pb.VirtioScsiController
	ScsiLuns  map[string]*pb.VirtioScsiLun
	transport VirtioBlkTransport
}

// Server contains frontend related OPI services
type Server struct {
	pb.UnimplementedFrontendNvmeServiceServer
	pb.UnimplementedFrontendVirtioBlkServiceServer
	pb.UnimplementedFrontendVirtioScsiServiceServer

	rpc        spdk.JSONRPC
	Nvme       NvmeParameters
	Virt       VirtioParameters
	Pagination map[string]int
}

// NewServer creates initialized instance of FrontEnd server communicating
// with provided jsonRPC
func NewServer(jsonRPC spdk.JSONRPC) *Server {
	return &Server{
		rpc: jsonRPC,
		Nvme: NvmeParameters{
			Subsystems:     make(map[string]*pb.NvmeSubsystem),
			Controllers:    make(map[string]*pb.NvmeController),
			Namespaces:     make(map[string]*pb.NvmeNamespace),
			subsysListener: NewTCPSubsystemListener("127.0.0.1:4420"),
		},
		Virt: VirtioParameters{
			BlkCtrls:  make(map[string]*pb.VirtioBlk),
			ScsiCtrls: make(map[string]*pb.VirtioScsiController),
			ScsiLuns:  make(map[string]*pb.VirtioScsiLun),
			transport: NewVhostUserBlkTransport(),
		},
		Pagination: make(map[string]int),
	}
}

// NewCustomizedServer creates initialized instance of FrontEnd server communicating
// with provided jsonRPC and externally created SubsystemListener and VirtioBlkTransport
func NewCustomizedServer(
	jsonRPC spdk.JSONRPC,
	sysListener SubsystemListener,
	virtioBlkTransport VirtioBlkTransport,
) *Server {
	if sysListener == nil {
		log.Panic("nil for SubsystemListener is not allowed")
	}

	if virtioBlkTransport == nil {
		log.Panic("nil for VirtioBlkTransport is not allowed")
	}

	server := NewServer(jsonRPC)
	server.Nvme.subsysListener = sysListener
	server.Virt.transport = virtioBlkTransport
	return server
}
