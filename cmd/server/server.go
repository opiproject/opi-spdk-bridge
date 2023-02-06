// Package server contains the code which can be used to start opi-spdk-bridge
// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation
package server

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"github.com/opiproject/opi-spdk-bridge/pkg/server/extension"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port = flag.Int("port", 50051, "The Server port")
)

// Run is used to start opi-spdk-bridge instance
func Run() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	baseSpdkServer := &server.Server{}
	spdkServer, err := extension.Extend(baseSpdkServer)
	if err != nil {
		log.Fatalf("failed to apply extension: %v", err)
	}
	pb.RegisterFrontendNvmeServiceServer(s, spdkServer)
	pb.RegisterNVMfRemoteControllerServiceServer(s, spdkServer)
	pb.RegisterFrontendVirtioBlkServiceServer(s, spdkServer)
	pb.RegisterFrontendVirtioScsiServiceServer(s, spdkServer)
	pb.RegisterNullDebugServiceServer(s, spdkServer)
	pb.RegisterAioControllerServiceServer(s, spdkServer)
	pb.RegisterMiddleendServiceServer(s, spdkServer)

	reflection.Register(s)

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
