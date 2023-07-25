// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

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

func sortAioVolumes(volumes []*pb.AioVolume) {
	sort.Slice(volumes, func(i int, j int) bool {
		return volumes[i].Name < volumes[j].Name
	})
}

// CreateAioVolume creates an Aio volume
func (s *Server) CreateAioVolume(_ context.Context, in *pb.CreateAioVolumeRequest) (*pb.AioVolume, error) {
	log.Printf("CreateAioVolume: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.AioVolumeId != "" {
		err := resourceid.ValidateUserSettable(in.AioVolumeId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.AioVolumeId, in.AioVolume.Name)
		resourceID = in.AioVolumeId
	}
	in.AioVolume.Name = server.ResourceIDToVolumeName(resourceID)
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.AioVolumes[in.AioVolume.Name]
	if ok {
		log.Printf("Already existing AioVolume with id %v", in.AioVolume.Name)
		return volume, nil
	}
	// not found, so create a new one
	params := spdk.BdevAioCreateParams{
		Name:      resourceID,
		BlockSize: 512,
		Filename:  in.AioVolume.Filename,
	}
	var result spdk.BdevAioCreateResult
	err := s.rpc.Call("bdev_aio_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result == "" {
		msg := fmt.Sprintf("Could not create Aio Dev: %s", params.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := server.ProtoClone(in.AioVolume)
	s.Volumes.AioVolumes[in.AioVolume.Name] = response
	log.Printf("CreateAioVolume: Sending to client: %v", response)
	return response, nil
}

// DeleteAioVolume deletes an Aio volume
func (s *Server) DeleteAioVolume(_ context.Context, in *pb.DeleteAioVolumeRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteAioVolume: Received from client: %v", in)
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
	volume, ok := s.Volumes.AioVolumes[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.BdevAioDeleteParams{
		Name: resourceID,
	}
	var result spdk.BdevAioDeleteResult
	err := s.rpc.Call("bdev_aio_delete", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete Aio Dev: %s", params.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Volumes.AioVolumes, volume.Name)
	return &emptypb.Empty{}, nil
}

// UpdateAioVolume updates an Aio volume
func (s *Server) UpdateAioVolume(_ context.Context, in *pb.UpdateAioVolumeRequest) (*pb.AioVolume, error) {
	log.Printf("UpdateAioVolume: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.AioVolume.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.AioVolumes[in.AioVolume.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("Got AllowMissing, create a new resource, don't return error when resource not found")
			params := spdk.BdevAioCreateParams{
				Name:      path.Base(in.AioVolume.Name),
				BlockSize: 512,
				Filename:  in.AioVolume.Filename,
			}
			var result spdk.BdevAioCreateResult
			err := s.rpc.Call("bdev_aio_create", &params, &result)
			if err != nil {
				log.Printf("error: %v", err)
				return nil, err
			}
			log.Printf("Received from SPDK: %v", result)
			if result == "" {
				msg := fmt.Sprintf("Could not create Aio Dev: %s", params.Name)
				log.Print(msg)
				return nil, status.Errorf(codes.InvalidArgument, msg)
			}
			response := server.ProtoClone(in.AioVolume)
			s.Volumes.AioVolumes[in.AioVolume.Name] = response
			log.Printf("CreateAioVolume: Sending to client: %v", response)
			return response, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.AioVolume.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.AioVolume); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	params1 := spdk.BdevAioDeleteParams{
		Name: resourceID,
	}
	var result1 spdk.BdevAioDeleteResult
	err1 := s.rpc.Call("bdev_aio_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		msg := fmt.Sprintf("Could not delete Aio Dev: %s", params1.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	params2 := spdk.BdevAioCreateParams{
		Name:      resourceID,
		BlockSize: 512,
		Filename:  in.AioVolume.Filename,
	}
	var result2 spdk.BdevAioCreateResult
	err2 := s.rpc.Call("bdev_aio_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	if result2 == "" {
		msg := fmt.Sprintf("Could not create Aio Dev: %s", params2.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := server.ProtoClone(in.AioVolume)
	s.Volumes.AioVolumes[in.AioVolume.Name] = response
	return response, nil
}

// ListAioVolumes lists Aio volumes
func (s *Server) ListAioVolumes(_ context.Context, in *pb.ListAioVolumesRequest) (*pb.ListAioVolumesResponse, error) {
	log.Printf("ListAioVolumes: Received from client: %v", in)
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
	var result []spdk.BdevGetBdevsResult
	err := s.rpc.Call("bdev_get_bdevs", nil, &result)
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
	Blobarray := make([]*pb.AioVolume, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.AioVolume{Name: r.Name, BlockSize: r.BlockSize, BlocksCount: r.NumBlocks}
	}
	sortAioVolumes(Blobarray)
	return &pb.ListAioVolumesResponse{AioVolumes: Blobarray, NextPageToken: token}, nil
}

// GetAioVolume gets an Aio volume
func (s *Server) GetAioVolume(_ context.Context, in *pb.GetAioVolumeRequest) (*pb.AioVolume, error) {
	log.Printf("GetAioVolume: Received from client: %v", in)
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
	volume, ok := s.Volumes.AioVolumes[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.BdevGetBdevsParams{
		Name: resourceID,
	}
	var result []spdk.BdevGetBdevsResult
	err := s.rpc.Call("bdev_get_bdevs", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result))
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.AioVolume{Name: result[0].Name, BlockSize: result[0].BlockSize, BlocksCount: result[0].NumBlocks}, nil
}

// AioVolumeStats gets an Aio volume stats
func (s *Server) AioVolumeStats(_ context.Context, in *pb.AioVolumeStatsRequest) (*pb.AioVolumeStatsResponse, error) {
	log.Printf("AioVolumeStats: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Handle.Value); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.AioVolumes[in.Handle.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Handle.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.BdevGetIostatParams{
		Name: resourceID,
	}
	// See https://mholt.github.io/json-to-go/
	var result spdk.BdevGetIostatResult
	err := s.rpc.Call("bdev_get_iostat", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result.Bdevs) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result.Bdevs))
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.AioVolumeStatsResponse{Stats: &pb.VolumeStats{
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
