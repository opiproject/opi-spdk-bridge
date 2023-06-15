// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implements the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateNVMfPath creates a new NVMf path
func (s *Server) CreateNVMfPath(_ context.Context, in *pb.CreateNVMfPathRequest) (*pb.NVMfPath, error) {
	log.Printf("CreateNVMfPath: Received from client: %v", in)
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	resourceID := resourceid.NewSystemGenerated()
	if in.NvMfPathId != "" {
		err := resourceid.ValidateUserSettable(in.NvMfPathId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMfPathId, in.NvMfPath.Name)
		resourceID = in.NvMfPathId
	}
	in.NvMfPath.Name = server.ResourceIDToVolumeName(resourceID)

	nvmfPath, ok := s.Volumes.NvmePaths[in.NvMfPath.Name]
	if ok {
		log.Printf("Already existing NVMfPath with id %v", in.NvMfPath.Name)
		return nvmfPath, nil
	}

	controller, ok := s.Volumes.NvmeControllers[in.NvMfPath.ControllerId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find NVMfRemoteController by key %s", in.NvMfPath.ControllerId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	multipath := ""
	if numberOfPaths := s.numberOfPathsForController(controller.Name); numberOfPaths > 0 {
		// set multipath parameter only when at least one path already exists
		multipath = s.opiMultipathToSpdk(controller.Multipath)
	}
	params := spdk.BdevNvmeAttachControllerParams{
		Name:      path.Base(controller.Name),
		Trtype:    s.opiTransportToSpdk(in.NvMfPath.Trtype),
		Traddr:    in.NvMfPath.Traddr,
		Adrfam:    s.opiAdressFamilyToSpdk(in.NvMfPath.Adrfam),
		Trsvcid:   fmt.Sprint(in.NvMfPath.Trsvcid),
		Subnqn:    in.NvMfPath.Subnqn,
		Hostnqn:   in.NvMfPath.Hostnqn,
		Multipath: multipath,
		Hdgst:     controller.Hdgst,
		Ddgst:     controller.Ddgst,
	}
	var result []spdk.BdevNvmeAttachControllerResult
	err := s.rpc.Call("bdev_nvme_attach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	response := server.ProtoClone(in.NvMfPath)
	s.Volumes.NvmePaths[in.NvMfPath.Name] = response
	log.Printf("CreateNVMfPath: Sending to client: %v", response)
	return response, nil
}

// DeleteNVMfPath deletes a NVMf path
func (s *Server) DeleteNVMfPath(_ context.Context, in *pb.DeleteNVMfPathRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMfPath: Received from client: %v", in)

	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	nvmfPath, ok := s.Volumes.NvmePaths[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	controller, ok := s.Volumes.NvmeControllers[nvmfPath.ControllerId.Value]
	if !ok {
		err := status.Errorf(codes.Internal, "unable to find NVMfRemoteController by key %s", nvmfPath.ControllerId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.BdevNvmeDetachControllerParams{
		Name:    path.Base(controller.Name),
		Trtype:  s.opiTransportToSpdk(nvmfPath.Trtype),
		Traddr:  nvmfPath.Traddr,
		Adrfam:  s.opiAdressFamilyToSpdk(nvmfPath.Adrfam),
		Trsvcid: fmt.Sprint(nvmfPath.Trsvcid),
		Subnqn:  nvmfPath.Subnqn,
	}

	var result spdk.BdevNvmeDetachControllerResult
	err := s.rpc.Call("bdev_nvme_detach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, spdk.ErrFailedSpdkCall
	}
	log.Printf("Received from SPDK: %v", result)

	delete(s.Volumes.NvmePaths, in.Name)

	return &emptypb.Empty{}, nil
}

func (s *Server) opiTransportToSpdk(transport pb.NvmeTransportType) string {
	return strings.ReplaceAll(transport.String(), "NVME_TRANSPORT_", "")
}

func (s *Server) opiAdressFamilyToSpdk(adrfam pb.NvmeAddressFamily) string {
	return strings.ReplaceAll(adrfam.String(), "NVMF_ADRFAM_", "")
}

func (s *Server) opiMultipathToSpdk(multipath pb.NvmeMultipath) string {
	return strings.ToLower(
		strings.ReplaceAll(multipath.String(), "NVME_MULTIPATH_", ""),
	)
}

func (s *Server) numberOfPathsForController(controllerName string) int {
	numberOfPaths := 0
	for _, path := range s.Volumes.NvmePaths {
		if path.ControllerId.Value == controllerName {
			numberOfPaths++
		}
	}
	return numberOfPaths
}
