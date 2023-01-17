package server

import "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"

type Server struct {
	_go.UnimplementedFrontendNvmeServiceServer
	_go.UnimplementedNVMfRemoteControllerServiceServer
	_go.UnimplementedFrontendVirtioBlkServiceServer
	_go.UnimplementedFrontendVirtioScsiServiceServer
	_go.UnimplementedNullDebugServiceServer
	_go.UnimplementedAioControllerServiceServer
	_go.UnimplementedMiddleendServiceServer
}
