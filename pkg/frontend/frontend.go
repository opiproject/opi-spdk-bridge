// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"log"

	"github.com/philippgille/gokv"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// NvmeParameters contains all Nvme related structures
type NvmeParameters struct {
	Subsystems  map[string]*pb.NvmeSubsystem
	Controllers map[string]*pb.NvmeController
	Namespaces  map[string]*pb.NvmeNamespace
	transports  map[pb.NvmeTransportType]NvmeTransport
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
	store      gokv.Store
	Nvme       NvmeParameters
	Virt       VirtioParameters
	Pagination map[string]int
}

// NewServer creates initialized instance of FrontEnd server communicating
// with provided jsonRPC
func NewServer(jsonRPC spdk.JSONRPC, store gokv.Store) *Server {
	if jsonRPC == nil {
		log.Panic("nil for JSONRPC is not allowed")
	}
	if store == nil {
		log.Panic("nil for Store is not allowed")
	}
	return &Server{
		rpc:   jsonRPC,
		store: store,
		Nvme: NvmeParameters{
			Subsystems:  make(map[string]*pb.NvmeSubsystem),
			Controllers: make(map[string]*pb.NvmeController),
			Namespaces:  make(map[string]*pb.NvmeNamespace),
			transports: map[pb.NvmeTransportType]NvmeTransport{
				pb.NvmeTransportType_NVME_TRANSPORT_TCP: NewNvmeTCPTransport(),
			},
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
// with provided jsonRPC and externally created NvmeTransport and VirtioBlkTransport
func NewCustomizedServer(
	jsonRPC spdk.JSONRPC,
	store gokv.Store,
	nvmeTransports map[pb.NvmeTransportType]NvmeTransport,
	virtioBlkTransport VirtioBlkTransport,
) *Server {
	if len(nvmeTransports) == 0 {
		log.Panic("empty NvmeTransports are not allowed")
	}

	for k, v := range nvmeTransports {
		if v == nil {
			log.Panicf("nil transport is not allowed for %v", k)
		}
	}

	if virtioBlkTransport == nil {
		log.Panic("nil for VirtioBlkTransport is not allowed")
	}

	if jsonRPC == nil {
		log.Panic("nil for JSONRPC is not allowed")
	}
	if store == nil {
		log.Panic("nil for Store is not allowed")
	}

	server := NewServer(jsonRPC, store)
	server.Nvme.transports = nvmeTransports
	server.Virt.transport = virtioBlkTransport
	return server
}
