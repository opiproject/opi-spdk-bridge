// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"log"
	"path"
	"sort"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNVMfRemoteControllers(controllers []*pb.NVMfRemoteController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Name < controllers[j].Name
	})
}

// CreateNVMfRemoteController creates an NVMf remote controller
func (s *Server) CreateNVMfRemoteController(_ context.Context, in *pb.CreateNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("CreateNVMfRemoteController: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	if in.NvMfRemoteController.Multipath == pb.NvmeMultipath_NVME_MULTIPATH_UNSPECIFIED {
		msg := "Multipath type should be specified"
		log.Printf("error: %v", msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvMfRemoteControllerId != "" {
		err := resourceid.ValidateUserSettable(in.NvMfRemoteControllerId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMfRemoteControllerId, in.NvMfRemoteController.Name)
		resourceID = in.NvMfRemoteControllerId
	}
	in.NvMfRemoteController.Name = server.ResourceIDToVolumeName(resourceID)
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.NvmeControllers[in.NvMfRemoteController.Name]
	if ok {
		log.Printf("Already existing NVMfRemoteController with id %v", in.NvMfRemoteController.Name)
		return volume, nil
	}
	// not found, so create a new one
	response := server.ProtoClone(in.NvMfRemoteController)
	s.Volumes.NvmeControllers[in.NvMfRemoteController.Name] = response
	log.Printf("CreateNVMfRemoteController: Sending to client: %v", response)
	return response, nil
}

// DeleteNVMfRemoteController deletes an NVMf remote controller
func (s *Server) DeleteNVMfRemoteController(_ context.Context, in *pb.DeleteNVMfRemoteControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMfRemoteController: Received from client: %v", in)
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
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v -> %v", err, volume)
		return nil, err
	}
	if s.numberOfPathsForController(in.Name) > 0 {
		return nil, status.Error(codes.FailedPrecondition, "NvmfPaths exist for controller")
	}
	delete(s.Volumes.NvmeControllers, volume.Name)
	return &emptypb.Empty{}, nil
}

// NVMfRemoteControllerReset resets an NVMf remote controller
func (s *Server) NVMfRemoteControllerReset(_ context.Context, in *pb.NVMfRemoteControllerResetRequest) (*emptypb.Empty, error) {
	log.Printf("Received: %v", in.GetId())
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Id.Value); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ListNVMfRemoteControllers lists an NVMf remote controllers
func (s *Server) ListNVMfRemoteControllers(_ context.Context, in *pb.ListNVMfRemoteControllersRequest) (*pb.ListNVMfRemoteControllersResponse, error) {
	log.Printf("ListNVMfRemoteControllers: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}

	Blobarray := []*pb.NVMfRemoteController{}
	for _, controller := range s.Volumes.NvmeControllers {
		Blobarray = append(Blobarray, controller)
	}
	sortNVMfRemoteControllers(Blobarray)

	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(Blobarray), offset, size)
	Blobarray, hasMoreElements := server.LimitPagination(Blobarray, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	return &pb.ListNVMfRemoteControllersResponse{NvMfRemoteControllers: Blobarray, NextPageToken: token}, nil
}

// GetNVMfRemoteController gets an NVMf remote controller
func (s *Server) GetNVMfRemoteController(_ context.Context, in *pb.GetNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("GetNVMfRemoteController: Received from client: %v", in)
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
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	response := server.ProtoClone(volume)
	return response, nil
}

// NVMfRemoteControllerStats gets NVMf remote controller stats
func (s *Server) NVMfRemoteControllerStats(_ context.Context, in *pb.NVMfRemoteControllerStatsRequest) (*pb.NVMfRemoteControllerStatsResponse, error) {
	log.Printf("Received: %v", in.GetId())
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Id.Value); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Id.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Id.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	name := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", name)
	return &pb.NVMfRemoteControllerStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}
