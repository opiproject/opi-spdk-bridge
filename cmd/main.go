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
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 50051, "The Server port")
	flag.Parse()
	spdkSocket := *server.RPCSock

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	jsonRPC := server.NewUnixSocketJSONRPC(spdkSocket)
	frontendServer := frontend.NewServerWithJSONRPC(jsonRPC)
	backendServer := backend.NewServerWithJSONRPC(jsonRPC)
	middleendServer := middleend.NewServerWithJSONRPC(jsonRPC)

	pb.RegisterFrontendNvmeServiceServer(s, frontendServer)
	pb.RegisterFrontendVirtioBlkServiceServer(s, frontendServer)
	pb.RegisterFrontendVirtioScsiServiceServer(s, frontendServer)
	pb.RegisterNVMfRemoteControllerServiceServer(s, backendServer)
	pb.RegisterNullDebugServiceServer(s, backendServer)
	pb.RegisterAioControllerServiceServer(s, backendServer)
	pb.RegisterMiddleendServiceServer(s, middleendServer)

	reflection.Register(s)

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
