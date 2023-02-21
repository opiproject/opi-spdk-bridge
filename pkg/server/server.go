// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package server implements the server
package server

import _go "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"

// Server represents the Server object
type Server struct {
	_go.UnimplementedFrontendNvmeServiceServer
	_go.UnimplementedNVMfRemoteControllerServiceServer
	_go.UnimplementedFrontendVirtioBlkServiceServer
	_go.UnimplementedFrontendVirtioScsiServiceServer
	_go.UnimplementedNullDebugServiceServer
	_go.UnimplementedAioControllerServiceServer
	_go.UnimplementedMiddleendServiceServer
}
