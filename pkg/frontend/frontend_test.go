// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2024 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/philippgille/gokv/gomap"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var checkGlobalTestProtoObjectsNotChanged = utils.CheckTestProtoObjectsNotChanged(
	&testVirtioCtrl,
	&testController,
	&testSubsystem,
	&testNamespace,
)

// TODO: move test infrastructure code to a separate (test/server) package to avoid duplication

type frontendClient struct {
	pb.FrontendNvmeServiceClient
	pb.FrontendVirtioBlkServiceClient
	pb.FrontendVirtioScsiServiceClient
}

type testEnv struct {
	opiSpdkServer *Server
	client        *frontendClient
	ln            net.Listener
	testSocket    string
	ctx           context.Context
	conn          *grpc.ClientConn
	jsonRPC       spdk.JSONRPC
}

func (e *testEnv) Close() {
	utils.CloseListener(e.ln)
	utils.CloseGrpcConnection(e.conn)
	if err := os.RemoveAll(e.testSocket); err != nil {
		log.Fatal(err)
	}
}

func createTestEnvironment(spdkResponses []string) *testEnv {
	env := &testEnv{}
	env.testSocket = utils.GenerateSocketName("frontend")
	env.ln, env.jsonRPC = utils.CreateTestSpdkServer(env.testSocket, spdkResponses)
	options := gomap.DefaultOptions
	options.Codec = utils.ProtoCodec{}
	store := gomap.NewStore(options)
	env.opiSpdkServer = NewServer(env.jsonRPC, store)

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx,
		"",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer(env.opiSpdkServer)))
	if err != nil {
		log.Fatal(err)
	}
	env.ctx = ctx
	env.conn = conn

	env.client = &frontendClient{
		pb.NewFrontendNvmeServiceClient(env.conn),
		pb.NewFrontendVirtioBlkServiceClient(env.conn),
		pb.NewFrontendVirtioScsiServiceClient(env.conn),
	}

	return env
}

func dialer(opiSpdkServer *Server) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterFrontendNvmeServiceServer(server, opiSpdkServer)
	pb.RegisterFrontendVirtioBlkServiceServer(server, opiSpdkServer)
	pb.RegisterFrontendVirtioScsiServiceServer(server, opiSpdkServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func TestFrontEnd_NewCustomizedServer(t *testing.T) {
	validJSONRPC := spdk.NewClient("/some/path")
	validNvmeTransports := map[pb.NvmeTransportType]NvmeTransport{
		pb.NvmeTransportType_NVME_TRANSPORT_TYPE_TCP: NewNvmeTCPTransport(validJSONRPC),
	}
	validVirtioBLkTransport := NewVhostUserBlkTransport()
	validStore := gomap.NewStore(gomap.DefaultOptions)

	tests := map[string]struct {
		jsonRPC            spdk.JSONRPC
		store              gomap.Store
		nvmeTransports     map[pb.NvmeTransportType]NvmeTransport
		virtioBlkTransport VirtioBlkTransport
		wantPanic          bool
	}{
		"nil json rpc": {
			jsonRPC:            nil,
			store:              validStore,
			nvmeTransports:     validNvmeTransports,
			virtioBlkTransport: validVirtioBLkTransport,
			wantPanic:          true,
		},
		"nil nvme transports": {
			jsonRPC:            validJSONRPC,
			store:              validStore,
			nvmeTransports:     nil,
			virtioBlkTransport: validVirtioBLkTransport,
			wantPanic:          true,
		},
		"nil one of nvme transports": {
			jsonRPC: validJSONRPC,
			store:   validStore,
			nvmeTransports: map[pb.NvmeTransportType]NvmeTransport{
				pb.NvmeTransportType_NVME_TRANSPORT_TYPE_TCP: nil,
			},
			virtioBlkTransport: validVirtioBLkTransport,
			wantPanic:          true,
		},
		"nil virtio blk transport": {
			jsonRPC:            validJSONRPC,
			store:              validStore,
			nvmeTransports:     validNvmeTransports,
			virtioBlkTransport: nil,
			wantPanic:          true,
		},
		"all valid arguments": {
			jsonRPC:            validJSONRPC,
			store:              validStore,
			nvmeTransports:     validNvmeTransports,
			virtioBlkTransport: validVirtioBLkTransport,
			wantPanic:          false,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("NewCustomizedServer() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			server := NewCustomizedServer(tt.jsonRPC, tt.store, tt.nvmeTransports, tt.virtioBlkTransport)
			if server == nil && !tt.wantPanic {
				t.Error("expected non nil server or panic")
			}
		})
	}
}
