// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/volume"
)

// VolumeParameters contains MiddleEnd volume related structures
type VolumeParameters struct {
	qosVolumes map[string]*pb.QosVolume
}

// Server contains middleend related OPI services
type Server struct {
	pb.UnimplementedMiddleendEncryptionServiceServer
	pb.UnimplementedMiddleendQosVolumeServiceServer

	rpc        spdk.JSONRPC
	volumes    VolumeParameters
	Pagination map[string]int
	registry   *volume.Registry
}

// NewServer creates initialized instance of MiddleEnd server communicating
// with provided jsonRPC
func NewServer(jsonRPC spdk.JSONRPC, registry *volume.Registry) *Server {
	return &Server{
		rpc: jsonRPC,
		volumes: VolumeParameters{
			qosVolumes: make(map[string]*pb.QosVolume),
		},
		Pagination: make(map[string]int),
		registry:   registry,
	}
}
