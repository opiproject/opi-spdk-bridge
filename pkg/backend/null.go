// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"fmt"
	"log"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"

	"github.com/google/uuid"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateNullDebug creates a Null Debug instance
func (s *Server) CreateNullDebug(_ context.Context, in *pb.CreateNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("CreateNullDebug: Received from client: %v", in)
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.NullVolumes[in.NullDebug.Handle.Value]
	if ok {
		log.Printf("Already existing NullDebug with id %v", in.NullDebug.Handle.Value)
		return volume, nil
	}
	// not found, so create a new one
	params := models.BdevNullCreateParams{
		Name:      in.NullDebug.Handle.Value,
		BlockSize: 512,
		NumBlocks: 64,
	}
	var result models.BdevNullCreateResult
	err := s.rpc.Call("bdev_null_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result == "" {
		msg := fmt.Sprintf("Could not create Null Dev: %s", in.NullDebug.Handle.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := &pb.NullDebug{}
	err = deepcopier.Copy(in.NullDebug).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	s.Volumes.NullVolumes[in.NullDebug.Handle.Value] = response
	return response, nil
}

// DeleteNullDebug deletes a Null Debug instance
func (s *Server) DeleteNullDebug(_ context.Context, in *pb.DeleteNullDebugRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNullDebug: Received from client: %v", in)
	volume, ok := s.Volumes.NullVolumes[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.BdevNullDeleteParams{
		Name: in.Name,
	}
	var result models.BdevNullDeleteResult
	err := s.rpc.Call("bdev_null_delete", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete Null Dev: %s", volume.Handle.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Volumes.NullVolumes, volume.Handle.Value)
	return &emptypb.Empty{}, nil
}

// UpdateNullDebug updates a Null Debug instance
func (s *Server) UpdateNullDebug(_ context.Context, in *pb.UpdateNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("UpdateNullDebug: Received from client: %v", in)
	params1 := models.BdevNullDeleteParams{
		Name: in.NullDebug.Handle.Value,
	}
	var result1 models.BdevNullDeleteResult
	err1 := s.rpc.Call("bdev_null_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		msg := fmt.Sprintf("Could not delete Null Dev: %s", in.NullDebug.Handle.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	params2 := models.BdevNullCreateParams{
		Name:      in.NullDebug.Handle.Value,
		BlockSize: 512,
		NumBlocks: 64,
	}
	var result2 models.BdevNullCreateResult
	err2 := s.rpc.Call("bdev_null_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	if result2 == "" {
		msg := fmt.Sprintf("Could not create Null Dev: %s", in.NullDebug.Handle.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := &pb.NullDebug{}
	err3 := deepcopier.Copy(in.NullDebug).To(response)
	if err3 != nil {
		log.Printf("error: %v", err3)
		return nil, err3
	}
	s.Volumes.NullVolumes[in.NullDebug.Handle.Value] = response
	return response, nil
}

// ListNullDebugs lists Null Debug instances
func (s *Server) ListNullDebugs(_ context.Context, in *pb.ListNullDebugsRequest) (*pb.ListNullDebugsResponse, error) {
	log.Printf("ListNullDebugs: Received from client: %v", in)
	if in.PageSize < 0 {
		err := status.Error(codes.InvalidArgument, "negative PageSize is not allowed")
		log.Printf("error: %v", err)
		return nil, err
	}
	offset := 0
	if in.PageToken != "" {
		var ok bool
		offset, ok = s.Pagination[in.PageToken]
		if !ok {
			err := status.Errorf(codes.NotFound, "unable to find pagination token %s", in.PageToken)
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("Found offset %d from pagination token: %s", offset, in.PageToken)
	}
	var result []models.BdevGetBdevsResult
	err := s.rpc.Call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	var token string
	if in.PageSize > 0 && int(in.PageSize) < len(result) {
		log.Printf("Limiting result to %d:%d", offset, in.PageSize)
		result = result[offset:in.PageSize]
		token = uuid.New().String()
		s.Pagination[token] = offset + int(in.PageSize)
	}
	Blobarray := make([]*pb.NullDebug, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NullDebug{Handle: &pc.ObjectKey{Value: r.Name}, Uuid: &pc.Uuid{Value: r.UUID}, BlockSize: r.BlockSize, BlocksCount: r.NumBlocks}
	}
	return &pb.ListNullDebugsResponse{NullDebugs: Blobarray, NextPageToken: token}, nil
}

// GetNullDebug gets a a Null Debug instance
func (s *Server) GetNullDebug(_ context.Context, in *pb.GetNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("GetNullDebug: Received from client: %v", in)
	params := models.BdevGetBdevsParams{
		Name: in.Name,
	}
	var result []models.BdevGetBdevsResult
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
	return &pb.NullDebug{Handle: &pc.ObjectKey{Value: result[0].Name}, Uuid: &pc.Uuid{Value: result[0].UUID}, BlockSize: result[0].BlockSize, BlocksCount: result[0].NumBlocks}, nil
}

// NullDebugStats gets a Null Debug instance stats
func (s *Server) NullDebugStats(_ context.Context, in *pb.NullDebugStatsRequest) (*pb.NullDebugStatsResponse, error) {
	log.Printf("NullDebugStats: Received from client: %v", in)
	params := models.BdevGetIostatParams{
		Name: in.Handle.Value,
	}
	// See https://mholt.github.io/json-to-go/
	var result models.BdevGetIostatResult
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
	return &pb.NullDebugStatsResponse{Stats: &pb.VolumeStats{
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
