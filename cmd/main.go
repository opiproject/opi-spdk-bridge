// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/opiproject/gospdk/spdk"

	"github.com/opiproject/opi-spdk-bridge/pkg/backend"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/kvm"
	"github.com/opiproject/opi-spdk-bridge/pkg/middleend"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func splitBusesBySeparator(str string) []string {
	if str != "" {
		return strings.Split(str, ":")
	}
	return []string{}
}

func main() {
	var port int
	flag.IntVar(&port, "port", 50051, "The Server port")

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

	buses := splitBusesBySeparator(busesStr)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	jsonRPC := spdk.NewSpdkJSONRPC(spdkAddress)
	backendServer := backend.NewServer(jsonRPC)
	middleendServer := middleend.NewServer(jsonRPC)

	if useKvm {
		log.Println("Creating KVM server.")
		frontendServer := frontend.NewServerWithSubsystemListener(jsonRPC,
			kvm.NewVfiouserSubsystemListener(ctrlrDir))
		kvmServer := kvm.NewServer(frontendServer, qmpAddress, ctrlrDir, buses)

		pb.RegisterFrontendNvmeServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioBlkServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioScsiServiceServer(s, kvmServer)
	} else {
		frontendServer := frontend.NewServerWithSubsystemListener(jsonRPC,
			frontend.NewTCPSubsystemListener(tcpTransportListenAddr))
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

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
