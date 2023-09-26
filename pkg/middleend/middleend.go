// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"log"

	"github.com/philippgille/gokv"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// VolumeParameters contains MiddleEnd volume related structures
type VolumeParameters struct {
	qosVolumes map[string]*pb.QosVolume
	encVolumes map[string]*pb.EncryptedVolume
}

// Server contains middleend related OPI services
type Server struct {
	pb.UnimplementedMiddleendEncryptionServiceServer
	pb.UnimplementedMiddleendQosVolumeServiceServer

	rpc        spdk.JSONRPC
	store      gokv.Store
	volumes    VolumeParameters
	Pagination map[string]int
}

// NewServer creates initialized instance of MiddleEnd server communicating
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
		volumes: VolumeParameters{
			qosVolumes: make(map[string]*pb.QosVolume),
			encVolumes: make(map[string]*pb.EncryptedVolume),
		},
		Pagination: make(map[string]int),
	}
}
