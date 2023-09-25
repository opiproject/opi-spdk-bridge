// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
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

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	server "github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var checkGlobalTestProtoObjectsNotChanged = server.CheckTestProtoObjectsNotChanged(
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
	server.CloseListener(e.ln)
	server.CloseGrpcConnection(e.conn)
	if err := os.RemoveAll(e.testSocket); err != nil {
		log.Fatal(err)
	}
}

func createTestEnvironment(spdkResponses []string) *testEnv {
	env := &testEnv{}
	env.testSocket = server.GenerateSocketName("frontend")
	env.ln, env.jsonRPC = server.CreateTestSpdkServer(env.testSocket, spdkResponses)
	env.opiSpdkServer = NewServer(env.jsonRPC)

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
	validJSONRPC := spdk.NewSpdkJSONRPC("/some/path")
	validNvmeTransport := NewNvmeTCPTransport("10.10.10.10:1234")
	validVirtioBLkTransport := NewVhostUserBlkTransport()

	tests := map[string]struct {
		jsonRPC            spdk.JSONRPC
		nvmeTransport      NvmeTransport
		virtioBlkTransport VirtioBlkTransport
		wantPanic          bool
	}{
		"nil json rpc": {
			jsonRPC:            nil,
			nvmeTransport:      validNvmeTransport,
			virtioBlkTransport: validVirtioBLkTransport,
			wantPanic:          true,
		},
		"nil nvme transport": {
			jsonRPC:            validJSONRPC,
			nvmeTransport:      nil,
			virtioBlkTransport: validVirtioBLkTransport,
			wantPanic:          true,
		},
		"nil virtio blk transport": {
			jsonRPC:            validJSONRPC,
			nvmeTransport:      validNvmeTransport,
			virtioBlkTransport: nil,
			wantPanic:          true,
		},
		"all valid arguments": {
			jsonRPC:            validJSONRPC,
			nvmeTransport:      validNvmeTransport,
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

			server := NewCustomizedServer(tt.jsonRPC, tt.nvmeTransport, tt.virtioBlkTransport)
			if server == nil && !tt.wantPanic {
				t.Error("expected non nil server or panic")
			}
		})
	}
}
