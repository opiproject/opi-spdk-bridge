// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation
package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/opiproject/opi-spdk-bridge/pkg/backend"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/middleend"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 50051, "The Server port")
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	fe := frontend.NewServer()
	be := backend.NewServer()
	me := middleend.NewServer()

	pb.RegisterFrontendNvmeServiceServer(s, fe)
	pb.RegisterFrontendVirtioBlkServiceServer(s, fe)
	pb.RegisterFrontendVirtioScsiServiceServer(s, fe)
	pb.RegisterNVMfRemoteControllerServiceServer(s, be)
	pb.RegisterNullDebugServiceServer(s, be)
	pb.RegisterAioControllerServiceServer(s, be)
	pb.RegisterMiddleendServiceServer(s, me)

	reflection.Register(s)

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
