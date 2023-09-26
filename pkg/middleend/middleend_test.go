// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implememnts the MiddleEnd APIs (service) of the storage Server
package middleend

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
	server "github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var checkGlobalTestProtoObjectsNotChanged = server.CheckTestProtoObjectsNotChanged(
	testQosVolume,
	&encryptedVolume,
)

// TODO: move test infrastructure code to a separate (test/server) package to avoid duplication

type middleendClient struct {
	pb.MiddleendEncryptionServiceClient
	pb.MiddleendQosVolumeServiceClient
}

type testEnv struct {
	opiSpdkServer *Server
	client        *middleendClient
	ln            net.Listener
	testSocket    string
	ctx           context.Context
	conn          *grpc.ClientConn
	jsonRPC       spdk.JSONRPC
}

func (e *testEnv) Close() {
	server.CloseListener(e.ln)
	if err := os.RemoveAll(e.testSocket); err != nil {
		log.Fatal(err)
	}
	server.CloseGrpcConnection(e.conn)
}

func createTestEnvironment(spdkResponses []string) *testEnv {
	env := &testEnv{}
	env.testSocket = server.GenerateSocketName("middleend")
	env.ln, env.jsonRPC = server.CreateTestSpdkServer(env.testSocket, spdkResponses)
	store := gomap.NewStore(gomap.DefaultOptions)
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

	env.client = &middleendClient{
		pb.NewMiddleendEncryptionServiceClient(env.conn),
		pb.NewMiddleendQosVolumeServiceClient(env.conn),
	}

	return env
}

func dialer(opiSpdkServer *Server) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterMiddleendEncryptionServiceServer(server, opiSpdkServer)
	pb.RegisterMiddleendQosVolumeServiceServer(server, opiSpdkServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

var (
	encryptedVolumeID   = "crypto-test"
	encryptedVolumeName = server.ResourceIDToVolumeName(encryptedVolumeID)
	encryptedVolume     = pb.EncryptedVolume{
		VolumeNameRef: "volume-test",
		Key:           []byte("0123456789abcdef0123456789abcdef"),
		Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128,
	}
)
