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
	"sort"
	"strings"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNVMfPaths(paths []*pb.NVMfPath) {
	sort.Slice(paths, func(i int, j int) bool {
		return paths[i].Subnqn < paths[j].Subnqn
	})
}

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
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	delete(s.Volumes.NvmePaths, in.Name)

	return &emptypb.Empty{}, nil
}

// UpdateNVMfPath updates an Nvme path
func (s *Server) UpdateNVMfPath(_ context.Context, in *pb.UpdateNVMfPathRequest) (*pb.NVMfPath, error) {
	log.Printf("UpdateNVMfPath: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.NvMfPath.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmePaths[in.NvMfPath.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvMfPath.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvMfPath); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	response := server.ProtoClone(in.NvMfPath)
	// s.Volumes.NvmePaths[in.NvMfPath.Name] = response
	return response, nil
}

// ListNVMfPaths lists Nvme path
func (s *Server) ListNVMfPaths(_ context.Context, in *pb.ListNVMfPathsRequest) (*pb.ListNVMfPathsResponse, error) {
	log.Printf("ListNVMfPaths: Received from client: %v", in)
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
	var result []spdk.BdevNvmeGetControllerResult
	err := s.rpc.Call("bdev_nvme_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
	result, hasMoreElements := server.LimitPagination(result, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.NVMfPath, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NVMfPath{Name: r.Name /* TODO: fill this */}
	}
	sortNVMfPaths(Blobarray)
	return &pb.ListNVMfPathsResponse{NvMfPaths: Blobarray, NextPageToken: token}, nil
}

// GetNVMfPath gets Nvme path
func (s *Server) GetNVMfPath(_ context.Context, in *pb.GetNVMfPathRequest) (*pb.NVMfPath, error) {
	log.Printf("GetNVMfPath: Received from client: %v", in)
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
	path, ok := s.Volumes.NvmePaths[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	var result []spdk.BdevNvmeGetControllerResult
	err := s.rpc.Call("bdev_nvme_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	for i := range result {
		r := &result[i]
		if r.Name != "" {
			return &pb.NVMfPath{ /* TODO: fill this */ }, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", path.Subnqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// NVMfPathStats gets Nvme path stats
func (s *Server) NVMfPathStats(_ context.Context, in *pb.NVMfPathStatsRequest) (*pb.NVMfPathStatsResponse, error) {
	log.Printf("NVMfPathStats: Received from client: %v", in)
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
	volume, ok := s.Volumes.NvmePaths[in.Id.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Id.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	var result spdk.NvmfGetSubsystemStatsResult
	err := s.rpc.Call("nvmf_get_stats", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NVMfPathStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
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
