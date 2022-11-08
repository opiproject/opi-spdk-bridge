// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// The main package of the storage server
package main

import (
	"context"
	"fmt"
	"log"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

//////////////////////////////////////////////////////////

func (s *server) NVMfRemoteControllerConnect(ctx context.Context, in *pb.NVMfRemoteControllerConnectRequest) (*pb.NVMfRemoteControllerConnectResponse, error) {
	log.Printf("NVMfRemoteControllerConnect: Received from client: %v", in)
	params := BdevNvmeAttachControllerParams{
		Name:      fmt.Sprint("OpiNvme", in.GetCtrl().GetId()),
		Type:      "TCP",
		Address:   in.GetCtrl().GetTraddr(),
		Family:    "ipv4",
		Port:      fmt.Sprint(in.GetCtrl().GetTrsvcid()),
		Subsystem: in.GetCtrl().GetSubnqn(),
	}
	var result []BdevNvmeAttachControllerResult
	err := call("bdev_nvme_attach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		log.Printf("expecting exactly 1 result")
	}
	return &pb.NVMfRemoteControllerConnectResponse{}, nil
}

func (s *server) NVMfRemoteControllerDisconnect(ctx context.Context, in *pb.NVMfRemoteControllerDisconnectRequest) (*pb.NVMfRemoteControllerDisconnectResponse, error) {
	log.Printf("NVMfRemoteControllerDisconnect: Received from client: %v", in)
	params := BdevNvmeDetachControllerParams{
		Name: fmt.Sprint("OpiNvme", in.GetId()),
	}
	var result BdevNvmeDetachControllerResult
	err := call("bdev_nvme_detach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NVMfRemoteControllerDisconnectResponse{}, nil
}

func (s *server) NVMfRemoteControllerReset(ctx context.Context, in *pb.NVMfRemoteControllerResetRequest) (*pb.NVMfRemoteControllerResetResponse, error) {
	log.Printf("Received: %v", in.GetId())
	return &pb.NVMfRemoteControllerResetResponse{}, nil
}

func (s *server) NVMfRemoteControllerList(ctx context.Context, in *pb.NVMfRemoteControllerListRequest) (*pb.NVMfRemoteControllerListResponse, error) {
	log.Printf("NVMfRemoteControllerList: Received from client: %v", in)
	var result []BdevNvmeGetControllerResult
	err := call("bdev_nvme_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.NVMfRemoteController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NVMfRemoteController{Subnqn: r.Name}
	}
	return &pb.NVMfRemoteControllerListResponse{Ctrl: Blobarray}, nil
}

func (s *server) NVMfRemoteControllerGet(ctx context.Context, in *pb.NVMfRemoteControllerGetRequest) (*pb.NVMfRemoteControllerGetResponse, error) {
	log.Printf("NVMfRemoteControllerGet: Received from client: %v", in)
	params := BdevNvmeGetControllerParams{
		Name: fmt.Sprint("OpiNvme", in.GetId()),
	}
	var result []BdevNvmeGetControllerResult
	err := call("bdev_nvme_get_controllers", &params, &result)
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
	return &pb.NVMfRemoteControllerGetResponse{Ctrl: &pb.NVMfRemoteController{Subnqn: result[0].Name}}, nil
}

func (s *server) NVMfRemoteControllerStats(ctx context.Context, in *pb.NVMfRemoteControllerStatsRequest) (*pb.NVMfRemoteControllerStatsResponse, error) {
	log.Printf("Received: %v", in.GetId())
	return &pb.NVMfRemoteControllerStatsResponse{}, nil
}

//////////////////////////////////////////////////////////

func (s *server) NullDebugCreate(ctx context.Context, in *pb.NullDebugCreateRequest) (*pb.NullDebugCreateResponse, error) {
	log.Printf("NullDebugCreate: Received from client: %v", in)
	params := BdevNullCreateParams{
		Name:      in.GetDevice().GetName(),
		BlockSize: 512,
		NumBlocks: 64,
	}
	var result BdevNullCreateResult
	err := call("bdev_null_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NullDebugCreateResponse{}, nil
}

func (s *server) NullDebugDelete(ctx context.Context, in *pb.NullDebugDeleteRequest) (*pb.NullDebugDeleteResponse, error) {
	log.Printf("NullDebugDelete: Received from client: %v", in)
	params := BdevNullDeleteParams{
		Name: fmt.Sprint("OpiNull", in.GetId()),
	}
	var result BdevNullDeleteResult
	err := call("bdev_null_delete", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	return &pb.NullDebugDeleteResponse{}, nil
}

func (s *server) NullDebugUpdate(ctx context.Context, in *pb.NullDebugUpdateRequest) (*pb.NullDebugUpdateResponse, error) {
	log.Printf("NullDebugUpdate: Received from client: %v", in)
	params1 := BdevNullDeleteParams{
		Name: in.GetDevice().GetName(),
	}
	var result1 BdevNullDeleteResult
	err1 := call("bdev_null_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		log.Printf("Could not delete: %v", in)
	}
	params2 := BdevNullCreateParams{
		Name:      in.GetDevice().GetName(),
		BlockSize: 512,
		NumBlocks: 64,
	}
	var result2 BdevNullCreateResult
	err2 := call("bdev_null_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	return &pb.NullDebugUpdateResponse{}, nil
}

func (s *server) NullDebugList(ctx context.Context, in *pb.NullDebugListRequest) (*pb.NullDebugListResponse, error) {
	log.Printf("NullDebugList: Received from client: %v", in)
	var result []BdevGetBdevsResult
	err := call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.NullDebug, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NullDebug{Name: r.Name, Uuid: &pc.Uuid{Value: r.UUID}}
	}
	return &pb.NullDebugListResponse{Device: Blobarray}, nil
}

func (s *server) NullDebugGet(ctx context.Context, in *pb.NullDebugGetRequest) (*pb.NullDebugGetResponse, error) {
	log.Printf("NullDebugGet: Received from client: %v", in)
	params := BdevGetBdevsParams{
		Name: fmt.Sprint("OpiNull", in.GetId()),
	}
	var result []BdevGetBdevsResult
	err := call("bdev_get_bdevs", &params, &result)
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
	return &pb.NullDebugGetResponse{Device: &pb.NullDebug{Name: result[0].Name, Uuid: &pc.Uuid{Value: result[0].UUID}}}, nil
}

func (s *server) NullDebugStats(ctx context.Context, in *pb.NullDebugStatsRequest) (*pb.NullDebugStatsResponse, error) {
	log.Printf("NullDebugStats: Received from client: %v", in)
	params := BdevGetIostatParams{
		Name: fmt.Sprint("OpiNull", in.GetId()),
	}
	// See https://mholt.github.io/json-to-go/
	var result BdevGetIostatResult
	err := call("bdev_get_iostat", &params, &result)
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
	return &pb.NullDebugStatsResponse{Stats: fmt.Sprint(result.Bdevs[0])}, nil
}

//////////////////////////////////////////////////////////

func (s *server) AioControllerCreate(ctx context.Context, in *pb.AioControllerCreateRequest) (*pb.AioController, error) {
	log.Printf("AioControllerCreate: Received from client: %v", in)
	params := BdevAioCreateParams{
		Name:      in.GetDevice().GetName(),
		BlockSize: 512,
		Filename:  in.GetDevice().GetFilename(),
	}
	var result BdevAioCreateResult
	err := call("bdev_aio_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.AioController{}, nil
}

func (s *server) AioControllerDelete(ctx context.Context, in *pb.AioControllerDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("AioControllerDelete: Received from client: %v", in)
	params := BdevAioDeleteParams{
		Name: fmt.Sprint("OpiAio", in.GetHandle().GetValue()),
	}
	var result BdevAioDeleteResult
	err := call("bdev_aio_delete", &params, &result)
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

func (s *server) AioControllerUpdate(ctx context.Context, in *pb.AioControllerUpdateRequest) (*pb.AioController, error) {
	log.Printf("AioControllerUpdate: Received from client: %v", in)
	params1 := BdevAioDeleteParams{
		Name: in.GetDevice().GetName(),
	}
	var result1 BdevAioDeleteResult
	err1 := call("bdev_aio_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		log.Printf("Could not delete: %v", in)
	}
	params2 := BdevAioCreateParams{
		Name:      in.GetDevice().GetName(),
		BlockSize: 512,
		Filename:  in.GetDevice().GetFilename(),
	}
	var result2 BdevAioCreateResult
	err2 := call("bdev_aio_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	return &pb.AioController{}, nil
}

func (s *server) AioControllerGetList(ctx context.Context, in *pb.AioControllerGetListRequest) (*pb.AioControllerList, error) {
	log.Printf("AioControllerGetList: Received from client: %v", in)
	var result []BdevGetBdevsResult
	err := call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.AioController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.AioController{Name: r.Name}
	}
	return &pb.AioControllerList{Device: Blobarray}, nil
}

func (s *server) AioControllerGet(ctx context.Context, in *pb.AioControllerGetRequest) (*pb.AioController, error) {
	log.Printf("AioControllerGet: Received from client: %v", in)
	params := BdevGetBdevsParams{
		Name: fmt.Sprint("OpiAio", in.GetHandle().GetValue()),
	}
	var result []BdevGetBdevsResult
	err := call("bdev_get_bdevs", &params, &result)
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
	return &pb.AioController{Name: result[0].Name}, nil
}

func (s *server) AioControllerGetStats(ctx context.Context, in *pb.AioControllerGetStatsRequest) (*pb.AioControllerStats, error) {
	log.Printf("AioControllerGetStats: Received from client: %v", in)
	params := BdevGetIostatParams{
		Name: fmt.Sprint("OpiAio", in.GetHandle().GetValue()),
	}
	// See https://mholt.github.io/json-to-go/
	var result BdevGetIostatResult
	err := call("bdev_get_iostat", &params, &result)
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
	return &pb.AioControllerStats{Stats: fmt.Sprint(result.Bdevs[0])}, nil
}

//////////////////////////////////////////////////////////
