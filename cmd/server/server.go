// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port = flag.Int("port", 50051, "The Server port")
)

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	pb.RegisterFrontendNvmeServiceServer(s, &server.Server{})
	pb.RegisterNVMfRemoteControllerServiceServer(s, &server.Server{})
	pb.RegisterFrontendVirtioBlkServiceServer(s, &server.Server{})
	pb.RegisterFrontendVirtioScsiServiceServer(s, &server.Server{})
	pb.RegisterNullDebugServiceServer(s, &server.Server{})
	pb.RegisterAioControllerServiceServer(s, &server.Server{})
	pb.RegisterMiddleendServiceServer(s, &server.Server{})

	reflection.Register(s)

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
