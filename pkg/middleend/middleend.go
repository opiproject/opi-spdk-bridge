// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"log"

	"github.com/opiproject/opi-spdk-bridge/pkg/spdk"
	"github.com/philippgille/gokv"
	"github.com/spdk/spdk/go/rpc/client"

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

	rpc        *spdk.SpdkClientAdapter
	store      gokv.Store
	volumes    VolumeParameters
	tweakMode  string
	Pagination map[string]int
}

// NewServer creates initialized instance of MiddleEnd server communicating
// with provided jsonRPC
func NewServer(spdkClient client.IClient, store gokv.Store) *Server {
	return NewCustomizedServer(spdkClient, store, spdk.TweakModeSimpleLba)
}

// NewCustomizedServer creates initialized instance of MiddleEnd server communicating
// with provided jsonRPC, store and non standard tweak mode
func NewCustomizedServer(spdkClient client.IClient, store gokv.Store, tweakMode string) *Server {
	if spdkClient == nil {
		log.Panic("nil for spdkClient is not allowed")
	}
	if store == nil {
		log.Panic("nil for Store is not allowed")
	}
	return &Server{
		rpc:   spdk.NewSpdkClientAdapter(spdkClient),
		store: store,
		volumes: VolumeParameters{
			qosVolumes: make(map[string]*pb.QosVolume),
			encVolumes: make(map[string]*pb.EncryptedVolume),
		},
		tweakMode:  tweakMode,
		Pagination: make(map[string]int),
	}
}
