// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

type testEnv struct {
	*server.TestEnv
	opiSpdkServer *Server
	client        *server.TestOpiClient
	ctx           context.Context
}

func createTestEnvironment(startSpdkServer bool, spdkResponses []string) *testEnv {
	var opiSpdkServer *Server
	env := server.CreateTestEnvironment(startSpdkServer, spdkResponses,
		func(jsonRPC server.JSONRPC) server.TestOpiServer {
			opiSpdkServer = NewServer(jsonRPC)
			return server.TestOpiServer{
				FrontendNvmeServiceServer:         &pb.UnimplementedFrontendNvmeServiceServer{},
				FrontendVirtioBlkServiceServer:    &pb.UnimplementedFrontendVirtioBlkServiceServer{},
				FrontendVirtioScsiServiceServer:   &pb.UnimplementedFrontendVirtioScsiServiceServer{},
				MiddleendServiceServer:            &pb.UnimplementedMiddleendServiceServer{},
				AioControllerServiceServer:        opiSpdkServer,
				NullDebugServiceServer:            opiSpdkServer,
				NVMfRemoteControllerServiceServer: opiSpdkServer,
			}
		})
	return &testEnv{env, opiSpdkServer, env.Client, env.Ctx}
}
