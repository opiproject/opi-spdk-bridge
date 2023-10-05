// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"log"

	"github.com/philippgille/gokv"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// TODO: can we combine all of volume types into a single list?
//		 maybe create a volume abstraction like bdev in SPDK?

// VolumeParameters contains all BackEnd volume related structures
type VolumeParameters struct {
	AioVolumes  map[string]*pb.AioVolume
	NullVolumes map[string]*pb.NullVolume

	NvmeControllers map[string]*pb.NvmeRemoteController
	NvmePaths       map[string]*pb.NvmePath
}

// Server contains backend related OPI services
type Server struct {
	pb.UnimplementedNvmeRemoteControllerServiceServer
	pb.UnimplementedNullVolumeServiceServer
	pb.UnimplementedAioVolumeServiceServer

	rpc                spdk.JSONRPC
	store              gokv.Store
	Volumes            VolumeParameters
	Pagination         map[string]int
	keyToTemporaryFile func(tmpDir string, pskKey []byte) (string, error)
	pskDir             string
}

// NewServer creates initialized instance of BackEnd server communicating
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
		Volumes: VolumeParameters{
			AioVolumes:      make(map[string]*pb.AioVolume),
			NullVolumes:     make(map[string]*pb.NullVolume),
			NvmeControllers: make(map[string]*pb.NvmeRemoteController),
			NvmePaths:       make(map[string]*pb.NvmePath),
		},
		Pagination:         make(map[string]int),
		keyToTemporaryFile: keyToTemporaryFile,
		pskDir:             "/var/tmp",
	}
}
