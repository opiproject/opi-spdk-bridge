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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
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

func createTestEnvironment(startSpdkServer bool, spdkResponses []string) *testEnv {
	return createTestEnvironmentWithVirtioBlkQosProvider(
		startSpdkServer, spdkResponses, stubQosProvider{})
}

func createTestEnvironmentWithVirtioBlkQosProvider(
	startSpdkServer bool, spdkResponses []string, qosProvider VirtioBlkQosProvider,
) *testEnv {
	env := &testEnv{}
	env.testSocket = server.GenerateSocketName("frontend")
	env.ln, env.jsonRPC = server.CreateTestSpdkServer(env.testSocket, startSpdkServer, spdkResponses)
	env.opiSpdkServer = NewServer(env.jsonRPC, qosProvider)

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
