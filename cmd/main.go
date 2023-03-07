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
	"github.com/opiproject/opi-spdk-bridge/pkg/kvm"
	"github.com/opiproject/opi-spdk-bridge/pkg/middleend"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 50051, "The Server port")

	var spdkSocket string
	flag.StringVar(&spdkSocket, "rpc_sock", "/var/tmp/spdk.sock", "Path to SPDK JSON RPC socket")

	var useKvm bool
	flag.BoolVar(&useKvm, "kvm", false, "Automates interaction with QEMU to plug/unplug SPDK devices")

	var qmpAddress string
	flag.StringVar(&qmpAddress, "qmp_addr", "127.0.0.1:5555", "Points to QMP unix socket/tcp socket to interact with. Valid only with -kvm option")

	var ctrlrDir string
	flag.StringVar(&ctrlrDir, "ctrlr_dir", "/var/tmp", "Directory with created SPDK device unix sockets. Valid only with -kvm option")
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	jsonRPC := server.NewUnixSocketJSONRPC(spdkSocket)
	frontendServer := frontend.NewServerWithJSONRPC(jsonRPC)
	backendServer := backend.NewServerWithJSONRPC(jsonRPC)
	middleendServer := middleend.NewServerWithJSONRPC(jsonRPC)

	if useKvm {
		log.Println("Creating KVM server.")
		kvmServer := kvm.NewServer(frontendServer, qmpAddress, ctrlrDir)

		pb.RegisterFrontendNvmeServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioBlkServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioScsiServiceServer(s, kvmServer)
	} else {
		pb.RegisterFrontendNvmeServiceServer(s, frontendServer)
		pb.RegisterFrontendVirtioBlkServiceServer(s, frontendServer)
		pb.RegisterFrontendVirtioScsiServiceServer(s, frontendServer)
	}

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
