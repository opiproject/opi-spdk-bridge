// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
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

	"github.com/philippgille/gokv/gomap"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var checkGlobalTestProtoObjectsNotChanged = utils.CheckTestProtoObjectsNotChanged(
	&testAioVolume,
	&testAioVolumeWithName,
	&testNullVolume,
	&testNullVolumeWithName,
	&testMallocVolume,
	&testMallocVolumeWithName,
	&testNvmeCtrl,
	&testNvmeCtrlWithName,
	&testNvmePath,
	&testNvmePathWithName,
)

// TODO: move test infrastructure code to a separate (test/server) package to avoid duplication

type backendClient struct {
	pb.NvmeRemoteControllerServiceClient
	pb.NullVolumeServiceClient
	pb.MallocVolumeServiceClient
	pb.AioVolumeServiceClient
}

type testEnv struct {
	opiSpdkServer *Server
	client        *backendClient
	ln            net.Listener
	testSocket    string
	ctx           context.Context
	conn          *grpc.ClientConn
	jsonRPC       spdk.JSONRPC
}

func (e *testEnv) Close() {
	utils.CloseListener(e.ln)
	if err := os.RemoveAll(e.testSocket); err != nil {
		log.Fatal(err)
	}
	utils.CloseGrpcConnection(e.conn)
}

func createTestEnvironment(spdkResponses []string) *testEnv {
	env := &testEnv{}
	env.testSocket = utils.GenerateSocketName("backend")
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

	env.client = &backendClient{
		pb.NewNvmeRemoteControllerServiceClient(env.conn),
		pb.NewNullVolumeServiceClient(env.conn),
		pb.NewMallocVolumeServiceClient(env.conn),
		pb.NewAioVolumeServiceClient(env.conn),
	}

	return env
}

func dialer(opiSpdkServer *Server) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterNvmeRemoteControllerServiceServer(server, opiSpdkServer)
	pb.RegisterNullVolumeServiceServer(server, opiSpdkServer)
	pb.RegisterMallocVolumeServiceServer(server, opiSpdkServer)
	pb.RegisterAioVolumeServiceServer(server, opiSpdkServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}
