// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

// TODO: can we combine all of volume types into a single list?
//		 maybe create a volume abstraction like bdev in SPDK?

// VolumeParameters contains all BackEnd volume related structures
type VolumeParameters struct {
	AioVolumes  map[string]*pb.AioController
	NullVolumes map[string]*pb.NullDebug
	NvmeVolumes map[string]*pb.NVMfRemoteController
}

// Server contains backend related OPI services
type Server struct {
	pb.UnimplementedNVMfRemoteControllerServiceServer
	pb.UnimplementedNullDebugServiceServer
	pb.UnimplementedAioControllerServiceServer

	rpc     server.JSONRPC
	Volumes VolumeParameters
}

// NewServer creates initialized instance of BackEnd server communicating
// with provided jsonRPC
func NewServer(jsonRPC server.JSONRPC) *Server {
	return &Server{
		rpc: jsonRPC,
		Volumes: VolumeParameters{
			AioVolumes:  make(map[string]*pb.AioController),
			NullVolumes: make(map[string]*pb.NullDebug),
			NvmeVolumes: make(map[string]*pb.NVMfRemoteController),
		},
	}
}
