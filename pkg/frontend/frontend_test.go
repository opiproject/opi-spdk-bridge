// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

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
				FrontendNvmeServiceServer:         opiSpdkServer,
				FrontendVirtioBlkServiceServer:    opiSpdkServer,
				FrontendVirtioScsiServiceServer:   opiSpdkServer,
				MiddleendServiceServer:            &pb.UnimplementedMiddleendServiceServer{},
				AioControllerServiceServer:        &pb.UnimplementedAioControllerServiceServer{},
				NullDebugServiceServer:            &pb.UnimplementedNullDebugServiceServer{},
				NVMfRemoteControllerServiceServer: &pb.UnimplementedNVMfRemoteControllerServiceServer{},
			}
		})
	return &testEnv{env, opiSpdkServer, env.Client, env.Ctx}
}
