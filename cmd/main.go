// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/opiproject/gospdk/spdk"

	"github.com/opiproject/opi-spdk-bridge/pkg/backend"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/kvm"
	"github.com/opiproject/opi-spdk-bridge/pkg/middleend"
	server "github.com/opiproject/opi-spdk-bridge/pkg/utils"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func splitBusesBySeparator(str string) []string {
	if str != "" {
		return strings.Split(str, ":")
	}
	return []string{}
}

func main() {
	var grpcPort int
	flag.IntVar(&grpcPort, "grpc_port", 50051, "The gRPC server port")

	var httpPort int
	flag.IntVar(&httpPort, "http_port", 8082, "The HTTP server port")

	var spdkAddress string
	flag.StringVar(&spdkAddress, "spdk_addr", "/var/tmp/spdk.sock", "Points to SPDK unix socket/tcp socket to interact with")

	var useKvm bool
	flag.BoolVar(&useKvm, "kvm", false, "Automates interaction with QEMU to plug/unplug SPDK devices")

	var qmpAddress string
	flag.StringVar(&qmpAddress, "qmp_addr", "127.0.0.1:5555", "Points to QMP unix socket/tcp socket to interact with. Valid only with -kvm option")

	var ctrlrDir string
	flag.StringVar(&ctrlrDir, "ctrlr_dir", "", "Directory with created SPDK device unix sockets (-S option in SPDK). Valid only with -kvm option")

	var busesStr string
	flag.StringVar(&busesStr, "buses", "", "QEMU PCI buses IDs separated by `:` to attach Nvme/virtio-blk devices on. e.g. \"pci.opi.0:pci.opi.1\". Valid only with -kvm option")

	var tcpTransportListenAddr string
	flag.StringVar(&tcpTransportListenAddr, "tcp_trid", "127.0.0.1:4420", "ipv4 address:port (aka traddr:trsvcid) or ipv6 [address]:port tuple (aka [traddr]:trsvcid) to listen on for Nvme/TCP transport")
	flag.Parse()

	var tlsFiles string
	flag.StringVar(&tlsFiles, "tls", "", "TLS files in server_cert:server_key:ca_cert format.")

	go runGatewayServer(grpcPort, httpPort)
	runGrpcServer(grpcPort, useKvm, spdkAddress, qmpAddress, ctrlrDir, busesStr, tcpTransportListenAddr, tlsFiles)
}

func runGrpcServer(grpcPort int, useKvm bool, spdkAddress, qmpAddress, ctrlrDir, busesStr, tcpTransportListenAddr, tlsFiles string) {
	buses := splitBusesBySeparator(busesStr)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var serverOptions []grpc.ServerOption
	if tlsFiles == "" {
		log.Println("TLS files are not specified. Use insecure connection.")
	} else {
		log.Println("Use TLS certificate files:", tlsFiles)
		config, err := server.ParseTLSFiles(tlsFiles)
		if err != nil {
			log.Fatal("Failed to parse string with tls paths:", err)
		}
		log.Println("TLS config:", config)
		var option grpc.ServerOption
		if option, err = server.SetupTLSCredentials(config); err != nil {
			log.Fatal("Failed to setup TLS:", err)
		}
		serverOptions = append(serverOptions, option)
	}
	s := grpc.NewServer(serverOptions...)

	jsonRPC := spdk.NewSpdkJSONRPC(spdkAddress)
	backendServer := backend.NewServer(jsonRPC)
	middleendServer := middleend.NewServer(jsonRPC)

	if useKvm {
		log.Println("Creating KVM server.")
		frontendServer := frontend.NewCustomizedServer(jsonRPC,
			kvm.NewNvmeVfiouserTransport(ctrlrDir),
			frontend.NewVhostUserBlkTransport(),
		)
		kvmServer := kvm.NewServer(frontendServer, qmpAddress, ctrlrDir, buses)

		pb.RegisterFrontendNvmeServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioBlkServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioScsiServiceServer(s, kvmServer)
	} else {
		frontendServer := frontend.NewCustomizedServer(jsonRPC,
			frontend.NewNvmeTCPTransport(tcpTransportListenAddr),
			frontend.NewVhostUserBlkTransport(),
		)
		pb.RegisterFrontendNvmeServiceServer(s, frontendServer)
		pb.RegisterFrontendVirtioBlkServiceServer(s, frontendServer)
		pb.RegisterFrontendVirtioScsiServiceServer(s, frontendServer)
	}

	pb.RegisterNvmeRemoteControllerServiceServer(s, backendServer)
	pb.RegisterNullVolumeServiceServer(s, backendServer)
	pb.RegisterAioVolumeServiceServer(s, backendServer)
	pb.RegisterMiddleendEncryptionServiceServer(s, middleendServer)
	pb.RegisterMiddleendQosVolumeServiceServer(s, middleendServer)

	reflection.Register(s)

	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func runGatewayServer(grpcPort int, httpPort int) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register gRPC server endpoint
	// Note: Make sure the gRPC server is running properly and accessible
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err := pc.RegisterInventorySvcHandlerFromEndpoint(ctx, mux, fmt.Sprintf(":%d", grpcPort), opts)
	if err != nil {
		log.Panic("cannot register handler server")
	}

	// Start HTTP server (and proxy calls to gRPC server endpoint)
	log.Printf("HTTP Server listening at %v", httpPort)
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	err = server.ListenAndServe()
	if err != nil {
		log.Panic("cannot start HTTP gateway server")
	}
}
