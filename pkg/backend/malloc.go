// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2024 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation
// Copyright (c) 2024 Xsight Labs Inc

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"

	"github.com/opiproject/gospdk/spdk"
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

func sortMallocVolumes(volumes []*pb.MallocVolume) {
	sort.Slice(volumes, func(i int, j int) bool {
		return volumes[i].Name < volumes[j].Name
	})
}

// CreateMallocVolume creates a Malloc volume instance
func (s *Server) CreateMallocVolume(ctx context.Context, in *pb.CreateMallocVolumeRequest) (*pb.MallocVolume, error) {
	// check input correctness
	if err := s.validateCreateMallocVolumeRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.MallocVolumeId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.MallocVolumeId, in.MallocVolume.Name)
		resourceID = in.MallocVolumeId
	}
	in.MallocVolume.Name = utils.ResourceIDToVolumeName(resourceID)
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.MallocVolumes[in.MallocVolume.Name]
	if ok {
		log.Printf("Already existing MallocVolume with id %v", in.MallocVolume.Name)
		return volume, nil
	}
	// not found, so create a new one
	params := spdk.BdevMallocCreateParams{
		Name:         resourceID,
		BlockSize:    int(in.GetMallocVolume().GetBlockSize()),
		NumBlocks:    int(in.GetMallocVolume().GetBlocksCount()),
		MdSize:       int(in.GetMallocVolume().GetMetadataSize()),
		MdInterleave: true,
	}
	var result spdk.BdevMallocCreateResult
	err := s.rpc.Call(ctx, "bdev_malloc_create", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result == "" {
		msg := fmt.Sprintf("Could not create Malloc Dev: %s", params.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := utils.ProtoClone(in.MallocVolume)
	s.Volumes.MallocVolumes[in.MallocVolume.Name] = response
	return response, nil
}

// DeleteMallocVolume deletes a Malloc volume instance
func (s *Server) DeleteMallocVolume(ctx context.Context, in *pb.DeleteMallocVolumeRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteMallocVolumeRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.MallocVolumes[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.BdevMallocDeleteParams{
		Name: resourceID,
	}
	var result spdk.BdevMallocDeleteResult
	err := s.rpc.Call(ctx, "bdev_malloc_delete", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete Malloc Dev: %s", params.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Volumes.MallocVolumes, volume.Name)
	return &emptypb.Empty{}, nil
}

// UpdateMallocVolume updates a Malloc volume instance
func (s *Server) UpdateMallocVolume(ctx context.Context, in *pb.UpdateMallocVolumeRequest) (*pb.MallocVolume, error) {
	// check input correctness
	if err := s.validateUpdateMallocVolumeRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.MallocVolumes[in.MallocVolume.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("Got AllowMissing, create a new resource, don't return error when resource not found")
			params := spdk.BdevMallocCreateParams{
				Name:      path.Base(in.MallocVolume.Name),
				BlockSize: int(in.GetMallocVolume().GetBlockSize()),
				NumBlocks: int(in.GetMallocVolume().GetBlocksCount()),
			}
			var result spdk.BdevMallocCreateResult
			err := s.rpc.Call(ctx, "bdev_malloc_create", &params, &result)
			if err != nil {
				return nil, err
			}
			log.Printf("Received from SPDK: %v", result)
			if result == "" {
				msg := fmt.Sprintf("Could not create Malloc Dev: %s", params.Name)
				return nil, status.Errorf(codes.InvalidArgument, msg)
			}
			response := utils.ProtoClone(in.MallocVolume)
			s.Volumes.MallocVolumes[in.MallocVolume.Name] = response
			return response, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.MallocVolume.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.MallocVolume); err != nil {
		return nil, err
	}
	params1 := spdk.BdevMallocDeleteParams{
		Name: resourceID,
	}
	var result1 spdk.BdevMallocDeleteResult
	err1 := s.rpc.Call(ctx, "bdev_malloc_delete", &params1, &result1)
	if err1 != nil {
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		msg := fmt.Sprintf("Could not delete Malloc Dev: %s", params1.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	params2 := spdk.BdevMallocCreateParams{
		Name:      resourceID,
		BlockSize: 512,
		NumBlocks: 64,
	}
	var result2 spdk.BdevMallocCreateResult
	err2 := s.rpc.Call(ctx, "bdev_malloc_create", &params2, &result2)
	if err2 != nil {
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	if result2 == "" {
		msg := fmt.Sprintf("Could not create Malloc Dev: %s", params2.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := utils.ProtoClone(in.MallocVolume)
	s.Volumes.MallocVolumes[in.MallocVolume.Name] = response
	return response, nil
}

// ListMallocVolumes lists Malloc volume instances
func (s *Server) ListMallocVolumes(ctx context.Context, in *pb.ListMallocVolumesRequest) (*pb.ListMallocVolumesResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}
	var result []spdk.BdevGetBdevsResult
	err := s.rpc.Call(ctx, "bdev_get_bdevs", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
	result, hasMoreElements := utils.LimitPagination(result, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.MallocVolume, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.MallocVolume{Name: r.Name, Uuid: r.UUID, BlockSize: r.BlockSize, BlocksCount: r.NumBlocks}
	}
	sortMallocVolumes(Blobarray)
	return &pb.ListMallocVolumesResponse{MallocVolumes: Blobarray, NextPageToken: token}, nil
}

// GetMallocVolume gets a a Malloc volume instance
func (s *Server) GetMallocVolume(ctx context.Context, in *pb.GetMallocVolumeRequest) (*pb.MallocVolume, error) {
	// check input correctness
	if err := s.validateGetMallocVolumeRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.MallocVolumes[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.BdevGetBdevsParams{
		Name: resourceID,
	}
	var result []spdk.BdevGetBdevsResult
	err := s.rpc.Call(ctx, "bdev_get_bdevs", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result))
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.MallocVolume{Name: result[0].Name, Uuid: result[0].UUID, BlockSize: result[0].BlockSize, BlocksCount: result[0].NumBlocks}, nil
}

// StatsMallocVolume gets a Malloc volume instance stats
func (s *Server) StatsMallocVolume(ctx context.Context, in *pb.StatsMallocVolumeRequest) (*pb.StatsMallocVolumeResponse, error) {
	// check input correctness
	if err := s.validateStatsMallocVolumeRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.MallocVolumes[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.BdevGetIostatParams{
		Name: resourceID,
	}
	// See https://mholt.github.io/json-to-go/
	var result spdk.BdevGetIostatResult
	err := s.rpc.Call(ctx, "bdev_get_iostat", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result.Bdevs) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result.Bdevs))
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.StatsMallocVolumeResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.Bdevs[0].BytesRead),
		ReadOpsCount:      int32(result.Bdevs[0].NumReadOps),
		WriteBytesCount:   int32(result.Bdevs[0].BytesWritten),
		WriteOpsCount:     int32(result.Bdevs[0].NumWriteOps),
		UnmapBytesCount:   int32(result.Bdevs[0].BytesUnmapped),
		UnmapOpsCount:     int32(result.Bdevs[0].NumUnmapOps),
		ReadLatencyTicks:  int32(result.Bdevs[0].ReadLatencyTicks),
		WriteLatencyTicks: int32(result.Bdevs[0].WriteLatencyTicks),
		UnmapLatencyTicks: int32(result.Bdevs[0].UnmapLatencyTicks),
	}}, nil
}
