// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
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

	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateAioController creates an Aio controller
func (s *Server) CreateAioController(ctx context.Context, in *pb.CreateAioControllerRequest) (*pb.AioController, error) {
	log.Printf("CreateAioController: Received from client: %v", in)
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.AioVolumes[in.AioController.Handle.Value]
	if ok {
		log.Printf("Already existing AioController with id %v", in.AioController.Handle.Value)
		return volume, nil
	}
	// not found, so create a new one
	params := models.BdevAioCreateParams{
		Name:      in.AioController.Handle.Value,
		BlockSize: 512,
		Filename:  in.AioController.Filename,
	}
	var result models.BdevAioCreateResult
	err := s.rpc.Call("bdev_aio_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	response := &pb.AioController{}
	err = deepcopier.Copy(in.AioController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	s.Volumes.AioVolumes[in.AioController.Handle.Value] = response
	return response, nil
}

// DeleteAioController deletes an Aio controller
func (s *Server) DeleteAioController(ctx context.Context, in *pb.DeleteAioControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteAioController: Received from client: %v", in)
	params := models.BdevAioDeleteParams{
		Name: in.Name,
	}
	var result models.BdevAioDeleteResult
	err := s.rpc.Call("bdev_aio_delete", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	return &emptypb.Empty{}, nil
}

// UpdateAioController updates an Aio controller
func (s *Server) UpdateAioController(ctx context.Context, in *pb.UpdateAioControllerRequest) (*pb.AioController, error) {
	log.Printf("UpdateAioController: Received from client: %v", in)
	params1 := models.BdevAioDeleteParams{
		Name: in.AioController.Handle.Value,
	}
	var result1 models.BdevAioDeleteResult
	err1 := s.rpc.Call("bdev_aio_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		log.Printf("Could not delete: %v", in)
	}
	params2 := models.BdevAioCreateParams{
		Name:      in.AioController.Handle.Value,
		BlockSize: 512,
		Filename:  in.AioController.Filename,
	}
	var result2 models.BdevAioCreateResult
	err2 := s.rpc.Call("bdev_aio_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	return &pb.AioController{}, nil
}

// ListAioControllers lists Aio controllers
func (s *Server) ListAioControllers(ctx context.Context, in *pb.ListAioControllersRequest) (*pb.ListAioControllersResponse, error) {
	log.Printf("ListAioControllers: Received from client: %v", in)
	var result []models.BdevGetBdevsResult
	err := s.rpc.Call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.AioController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.AioController{Handle: &pc.ObjectKey{Value: r.Name}}
	}
	return &pb.ListAioControllersResponse{AioControllers: Blobarray}, nil
}

// GetAioController gets an Aio controller
func (s *Server) GetAioController(ctx context.Context, in *pb.GetAioControllerRequest) (*pb.AioController, error) {
	log.Printf("GetAioController: Received from client: %v", in)
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
	return &pb.AioController{Handle: &pc.ObjectKey{Value: result[0].Name}}, nil
}

// AioControllerStats gets an Aio controller stats
func (s *Server) AioControllerStats(ctx context.Context, in *pb.AioControllerStatsRequest) (*pb.AioControllerStatsResponse, error) {
	log.Printf("AioControllerStats: Received from client: %v", in)
	params := models.BdevGetIostatParams{
		Name: in.GetHandle().GetValue(),
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
	return &pb.AioControllerStatsResponse{Stats: &pb.VolumeStats{
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
