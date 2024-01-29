// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2024 Dell Inc, or its subsidiaries.
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
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	pc "github.com/opiproject/opi-api/inventory/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/redis"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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

	var tlsFiles string
	flag.StringVar(&tlsFiles, "tls", "", "TLS files in server_cert:server_key:ca_cert format.")

	var redisAddress string
	flag.StringVar(&redisAddress, "redis_addr", "127.0.0.1:6379", "Redis address in ip_address:port format")

	flag.Parse()

	// Create KV store for persistence
	options := redis.DefaultOptions
	options.Address = redisAddress
	options.Codec = utils.ProtoCodec{}
	store, err := redis.NewClient(options)
	if err != nil {
		log.Panic(err)
	}
	defer func(store gokv.Store) {
		err := store.Close()
		if err != nil {
			log.Panic(err)
		}
	}(store)

	go runGatewayServer(grpcPort, httpPort)
	runGrpcServer(grpcPort, useKvm, store, spdkAddress, qmpAddress, ctrlrDir, busesStr, tlsFiles)
}

func runGrpcServer(grpcPort int, useKvm bool, store gokv.Store, spdkAddress, qmpAddress, ctrlrDir, busesStr, tlsFiles string) {
	tp := utils.InitTracerProvider("opi-spdk-bridge")
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Panicf("Tracer Provider Shutdown: %v", err)
		}
	}()

	buses := splitBusesBySeparator(busesStr)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Panicf("failed to listen: %v", err)
	}

	var serverOptions []grpc.ServerOption
	if tlsFiles == "" {
		log.Println("TLS files are not specified. Use insecure connection.")
	} else {
		log.Println("Use TLS certificate files:", tlsFiles)
		config, err := utils.ParseTLSFiles(tlsFiles)
		if err != nil {
			log.Panic("Failed to parse string with tls paths:", err)
		}
		log.Println("TLS config:", config)
		var option grpc.ServerOption
		if option, err = utils.SetupTLSCredentials(config); err != nil {
			log.Panic("Failed to setup TLS:", err)
		}
		serverOptions = append(serverOptions, option)
	}
	serverOptions = append(serverOptions,
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(
			logging.UnaryServerInterceptor(utils.InterceptorLogger(log.Default()),
				logging.WithLogOnEvents(
					logging.StartCall,
					logging.FinishCall,
					logging.PayloadReceived,
					logging.PayloadSent,
				),
			)),
	)
	s := grpc.NewServer(serverOptions...)

	jsonRPC := spdk.NewClient(spdkAddress)
	backendServer := backend.NewServer(jsonRPC, store)
	middleendServer := middleend.NewServer(jsonRPC, store)

	if useKvm {
		log.Println("Creating KVM server.")
		frontendServer := frontend.NewCustomizedServer(jsonRPC,
			store,
			map[pb.NvmeTransportType]frontend.NvmeTransport{
				pb.NvmeTransportType_NVME_TRANSPORT_TYPE_TCP:  frontend.NewNvmeTCPTransport(jsonRPC),
				pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE: kvm.NewNvmeVfiouserTransport(ctrlrDir, jsonRPC),
			},
			frontend.NewVhostUserBlkTransport(),
		)
		kvmServer := kvm.NewServer(frontendServer, qmpAddress, ctrlrDir, buses)

		pb.RegisterFrontendNvmeServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioBlkServiceServer(s, kvmServer)
		pb.RegisterFrontendVirtioScsiServiceServer(s, kvmServer)
	} else {
		frontendServer := frontend.NewCustomizedServer(jsonRPC,
			store,
			map[pb.NvmeTransportType]frontend.NvmeTransport{
				pb.NvmeTransportType_NVME_TRANSPORT_TYPE_TCP: frontend.NewNvmeTCPTransport(jsonRPC),
			},
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
		log.Panicf("failed to serve: %v", err)
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
	endpoint := fmt.Sprintf("localhost:%d", grpcPort)
	registerGatewayHandler(ctx, mux, endpoint, opts, pc.RegisterInventoryServiceHandlerFromEndpoint, "inventory")

	registerGatewayHandler(ctx, mux, endpoint, opts, pb.RegisterAioVolumeServiceHandlerFromEndpoint, "backend aio")
	registerGatewayHandler(ctx, mux, endpoint, opts, pb.RegisterNullVolumeServiceHandlerFromEndpoint, "backend null")
	registerGatewayHandler(ctx, mux, endpoint, opts, pb.RegisterNvmeRemoteControllerServiceHandlerFromEndpoint, "backend nvme")

	// Start HTTP server (and proxy calls to gRPC server endpoint)
	log.Printf("HTTP Server listening at %v", httpPort)
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Panic("cannot start HTTP gateway server")
	}
}

type registerHandlerFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

func registerGatewayHandler(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption, registerFunc registerHandlerFunc, serviceName string) {
	err := registerFunc(ctx, mux, endpoint, opts)
	if err != nil {
		log.Panicf("cannot register %s handler server: %v", serviceName, err)
	}
}
