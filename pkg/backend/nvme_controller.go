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
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNvmeRemoteControllers(controllers []*pb.NvmeRemoteController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Name < controllers[j].Name
	})
}

// CreateNvmeRemoteController creates an Nvme remote controller
func (s *Server) CreateNvmeRemoteController(_ context.Context, in *pb.CreateNvmeRemoteControllerRequest) (*pb.NvmeRemoteController, error) {
	// check input correctness
	if err := s.validateCreateNvmeRemoteControllerRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvmeRemoteControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeRemoteControllerId, in.NvmeRemoteController.Name)
		resourceID = in.NvmeRemoteControllerId
	}
	in.NvmeRemoteController.Name = utils.ResourceIDToVolumeName(resourceID)
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.NvmeControllers[in.NvmeRemoteController.Name]
	if ok {
		log.Printf("Already existing NvmeRemoteController with id %v", in.NvmeRemoteController.Name)
		return volume, nil
	}
	// not found, so create a new one
	response := utils.ProtoClone(in.NvmeRemoteController)
	s.Volumes.NvmeControllers[in.NvmeRemoteController.Name] = response
	return response, nil
}

// DeleteNvmeRemoteController deletes an Nvme remote controller
func (s *Server) DeleteNvmeRemoteController(_ context.Context, in *pb.DeleteNvmeRemoteControllerRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteNvmeRemoteControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	if s.numberOfPathsForController(in.Name) > 0 {
		return nil, status.Error(codes.FailedPrecondition, "NvmePaths exist for controller")
	}
	delete(s.Volumes.NvmeControllers, volume.Name)
	return &emptypb.Empty{}, nil
}

// ResetNvmeRemoteController resets an Nvme remote controller
func (s *Server) ResetNvmeRemoteController(_ context.Context, in *pb.ResetNvmeRemoteControllerRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateResetNvmeRemoteControllerRequest(in); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// UpdateNvmeRemoteController resets an Nvme remote controller
func (s *Server) UpdateNvmeRemoteController(_ context.Context, in *pb.UpdateNvmeRemoteControllerRequest) (*pb.NvmeRemoteController, error) {
	// check input correctness
	if err := s.validateUpdateNvmeRemoteControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.NvmeRemoteController.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeRemoteController.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmeRemoteController); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	response := utils.ProtoClone(in.NvmeRemoteController)
	// s.Volumes.NvmeControllers[in.NvmeRemoteController.Name] = response
	return response, nil
}

// ListNvmeRemoteControllers lists an Nvme remote controllers
func (s *Server) ListNvmeRemoteControllers(_ context.Context, in *pb.ListNvmeRemoteControllersRequest) (*pb.ListNvmeRemoteControllersResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}

	Blobarray := []*pb.NvmeRemoteController{}
	for _, controller := range s.Volumes.NvmeControllers {
		Blobarray = append(Blobarray, controller)
	}
	sortNvmeRemoteControllers(Blobarray)

	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(Blobarray), offset, size)
	Blobarray, hasMoreElements := utils.LimitPagination(Blobarray, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	return &pb.ListNvmeRemoteControllersResponse{NvmeRemoteControllers: Blobarray, NextPageToken: token}, nil
}

// GetNvmeRemoteController gets an Nvme remote controller
func (s *Server) GetNvmeRemoteController(_ context.Context, in *pb.GetNvmeRemoteControllerRequest) (*pb.NvmeRemoteController, error) {
	// check input correctness
	if err := s.validateGetNvmeRemoteControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}

	response := utils.ProtoClone(volume)
	return response, nil
}

// StatsNvmeRemoteController gets Nvme remote controller stats
func (s *Server) StatsNvmeRemoteController(_ context.Context, in *pb.StatsNvmeRemoteControllerRequest) (*pb.StatsNvmeRemoteControllerResponse, error) {
	// check input correctness
	if err := s.validateStatsNvmeRemoteControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	name := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", name)
	return &pb.StatsNvmeRemoteControllerResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}
