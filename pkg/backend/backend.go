// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"log"
	"os"

	"github.com/philippgille/gokv"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// Server contains backend related OPI services
type Server struct {
	pb.UnimplementedNvmeRemoteControllerServiceServer
	pb.UnimplementedNullVolumeServiceServer
	pb.UnimplementedAioVolumeServiceServer

	rpc        spdk.JSONRPC
	store      gokv.Store
	Pagination map[string]int
	psk        psk
}

type psk struct {
	createTempFile func(dir, pattern string) (*os.File, error)
	writeKey       func(keyFile string, key []byte, perm os.FileMode) error
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
		rpc:        jsonRPC,
		store:      store,
		Pagination: make(map[string]int),
		psk: psk{
			createTempFile: os.CreateTemp,
			writeKey:       os.WriteFile,
		},
	}
}
