// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

// TODO: move test infrastructure code to a separate (test/server) package to avoid duplication

type backendClient struct {
	pb.NVMfRemoteControllerServiceClient
	pb.NullDebugServiceClient
	pb.AioControllerServiceClient
}

type testEnv struct {
	opiSpdkServer *Server
	client        *backendClient
	ln            net.Listener
	testSocket    string
	ctx           context.Context
	conn          *grpc.ClientConn
	jsonRPC       server.JSONRPC
}

func (e *testEnv) Close() {
	server.CloseListener(e.ln)
	if err := os.RemoveAll(e.testSocket); err != nil {
		log.Fatal(err)
	}
	server.CloseGrpcConnection(e.conn)
}

func createTestEnvironment(startSpdkServer bool, spdkResponses []string) *testEnv {
	env := &testEnv{}
	env.testSocket = server.GenerateSocketName("backend")
	env.ln, env.jsonRPC = server.CreateTestSpdkServer(env.testSocket, startSpdkServer, spdkResponses)
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

	env.client = &backendClient{
		pb.NewNVMfRemoteControllerServiceClient(env.conn),
		pb.NewNullDebugServiceClient(env.conn),
		pb.NewAioControllerServiceClient(env.conn),
	}

	return env
}

func dialer(opiSpdkServer *Server) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterNVMfRemoteControllerServiceServer(server, opiSpdkServer)
	pb.RegisterNullDebugServiceServer(server, opiSpdkServer)
	pb.RegisterAioControllerServiceServer(server, opiSpdkServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}
