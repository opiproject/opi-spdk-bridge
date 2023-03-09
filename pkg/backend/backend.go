// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

// Server contains backend related OPI services
type Server struct {
	pb.UnimplementedNVMfRemoteControllerServiceServer
	pb.UnimplementedNullDebugServiceServer
	pb.UnimplementedAioControllerServiceServer

	rpc server.JSONRPC
}

// NewServer creates initialized instance of BackEnd server communicating
// with provided jsonRPC
func NewServer(jsonRPC server.JSONRPC) *Server {
	return &Server{
		rpc: jsonRPC,
	}
}
