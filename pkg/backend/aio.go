// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortAioControllers(controllers []*pb.AioController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Handle.Value < controllers[j].Handle.Value
	})
}

// CreateAioController creates an Aio controller
func (s *Server) CreateAioController(_ context.Context, in *pb.CreateAioControllerRequest) (*pb.AioController, error) {
	log.Printf("CreateAioController: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	name := uuid.New().String()
	if in.AioControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.AioControllerId, in.AioController.Handle)
		name = in.AioControllerId
	}
	in.AioController.Handle = &pc.ObjectKey{Value: name}
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.AioVolumes[in.AioController.Handle.Value]
	if ok {
		log.Printf("Already existing AioController with id %v", in.AioController.Handle.Value)
		return volume, nil
	}
	// not found, so create a new one
	params := spdk.BdevAioCreateParams{
		Name:      name,
		BlockSize: 512,
		Filename:  in.AioController.Filename,
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
func (s *Server) DeleteAioController(_ context.Context, in *pb.DeleteAioControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteAioController: Received from client: %v", in)
	volume, ok := s.Volumes.AioVolumes[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.BdevAioDeleteParams{
		Name: in.Name,
	}
	var result spdk.BdevAioDeleteResult
	err := s.rpc.Call("bdev_aio_delete", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete Aio Dev: %s", volume.Handle.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Volumes.AioVolumes, volume.Handle.Value)
	return &emptypb.Empty{}, nil
}

// UpdateAioController updates an Aio controller
func (s *Server) UpdateAioController(_ context.Context, in *pb.UpdateAioControllerRequest) (*pb.AioController, error) {
	log.Printf("UpdateAioController: Received from client: %v", in)
	params1 := spdk.BdevAioDeleteParams{
		Name: in.AioController.Handle.Value,
	}
	var result1 spdk.BdevAioDeleteResult
	err1 := s.rpc.Call("bdev_aio_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		msg := fmt.Sprintf("Could not delete Aio Dev: %s", in.AioController.Handle.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	params2 := spdk.BdevAioCreateParams{
		Name:      in.AioController.Handle.Value,
		BlockSize: 512,
		Filename:  in.AioController.Filename,
	}
	var result2 spdk.BdevAioCreateResult
	err2 := s.rpc.Call("bdev_aio_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	if result2 == "" {
		msg := fmt.Sprintf("Could not create Aio Dev: %s", in.AioController.Handle.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := &pb.AioController{}
	err3 := deepcopier.Copy(in.AioController).To(response)
	if err3 != nil {
		log.Printf("error: %v", err3)
		return nil, err3
	}
	s.Volumes.AioVolumes[in.AioController.Handle.Value] = response
	return response, nil
}

// ListAioControllers lists Aio controllers
func (s *Server) ListAioControllers(_ context.Context, in *pb.ListAioControllersRequest) (*pb.ListAioControllersResponse, error) {
	log.Printf("ListAioControllers: Received from client: %v", in)
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
	Blobarray := make([]*pb.AioController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.AioController{Handle: &pc.ObjectKey{Value: r.Name}, BlockSize: r.BlockSize, BlocksCount: r.NumBlocks}
	}
	sortAioControllers(Blobarray)
	return &pb.ListAioControllersResponse{AioControllers: Blobarray, NextPageToken: token}, nil
}

// GetAioController gets an Aio controller
func (s *Server) GetAioController(_ context.Context, in *pb.GetAioControllerRequest) (*pb.AioController, error) {
	log.Printf("GetAioController: Received from client: %v", in)
	params := spdk.BdevGetBdevsParams{
		Name: in.Name,
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
	return &pb.AioController{Handle: &pc.ObjectKey{Value: result[0].Name}, BlockSize: result[0].BlockSize, BlocksCount: result[0].NumBlocks}, nil
}

// AioControllerStats gets an Aio controller stats
func (s *Server) AioControllerStats(_ context.Context, in *pb.AioControllerStatsRequest) (*pb.AioControllerStatsResponse, error) {
	log.Printf("AioControllerStats: Received from client: %v", in)
	params := spdk.BdevGetIostatParams{
		Name: in.GetHandle().GetValue(),
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
