// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server contains backend related OPI services
type Server struct {
	pb.UnimplementedNVMfRemoteControllerServiceServer
	pb.UnimplementedNullDebugServiceServer
	pb.UnimplementedAioControllerServiceServer
}

// CreateNVMfRemoteController creates an NVMf remote controller
func (s *Server) CreateNVMfRemoteController(ctx context.Context, in *pb.CreateNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("CreateNVMfRemoteController: Received from client: %v", in)
	params := models.BdevNvmeAttachControllerParams{
		Name:    in.NvMfRemoteController.Id.Value,
		Trtype:  strings.ReplaceAll(in.NvMfRemoteController.Trtype.String(), "NVME_TRANSPORT_", ""),
		Traddr:  in.NvMfRemoteController.Traddr,
		Adrfam:  strings.ReplaceAll(in.NvMfRemoteController.Adrfam.String(), "NVMF_ADRFAM_", ""),
		Trsvcid: fmt.Sprint(in.NvMfRemoteController.Trsvcid),
		Subnqn:  in.NvMfRemoteController.Subnqn,
		Hostnqn: in.NvMfRemoteController.Hostnqn,
	}
	var result []models.BdevNvmeAttachControllerResult
	err := server.Call("bdev_nvme_attach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		log.Printf("expecting exactly 1 result")
	}
	response := &pb.NVMfRemoteController{}
	err = deepcopier.Copy(in.NvMfRemoteController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// DeleteNVMfRemoteController deletes an NVMf remote controller
func (s *Server) DeleteNVMfRemoteController(ctx context.Context, in *pb.DeleteNVMfRemoteControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMfRemoteController: Received from client: %v", in)
	params := models.BdevNvmeDetachControllerParams{
		Name: in.Name,
	}
	var result models.BdevNvmeDetachControllerResult
	err := server.Call("bdev_nvme_detach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &emptypb.Empty{}, nil
}

// NVMfRemoteControllerReset resets an NVMf remote controller
func (s *Server) NVMfRemoteControllerReset(ctx context.Context, in *pb.NVMfRemoteControllerResetRequest) (*emptypb.Empty, error) {
	log.Printf("Received: %v", in.GetId())
	return &emptypb.Empty{}, nil
}

// ListNVMfRemoteControllers lists an NVMf remote controllers
func (s *Server) ListNVMfRemoteControllers(ctx context.Context, in *pb.ListNVMfRemoteControllersRequest) (*pb.ListNVMfRemoteControllersResponse, error) {
	log.Printf("ListNVMfRemoteControllers: Received from client: %v", in)
	var result []models.BdevNvmeGetControllerResult
	err := server.Call("bdev_nvme_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.NVMfRemoteController, len(result))
	for i := range result {
		r := &result[i]
		port, _ := strconv.ParseInt(r.Ctrlrs[0].Trid.Trsvcid, 10, 64)
		Blobarray[i] = &pb.NVMfRemoteController{
			Id:      &pc.ObjectKey{Value: r.Name},
			Hostnqn: r.Ctrlrs[0].Host.Nqn,
			Trtype:  pb.NvmeTransportType(pb.NvmeTransportType_value["NVME_TRANSPORT_"+strings.ToUpper(r.Ctrlrs[0].Trid.Trtype)]),
			Adrfam:  pb.NvmeAddressFamily(pb.NvmeAddressFamily_value["NVMF_ADRFAM_"+strings.ToUpper(r.Ctrlrs[0].Trid.Adrfam)]),
			Traddr:  r.Ctrlrs[0].Trid.Traddr,
			Subnqn:  r.Ctrlrs[0].Trid.Subnqn,
			Trsvcid: port,
		}
	}
	return &pb.ListNVMfRemoteControllersResponse{NvMfRemoteControllers: Blobarray}, nil
}

// GetNVMfRemoteController gets an NVMf remote controller
func (s *Server) GetNVMfRemoteController(ctx context.Context, in *pb.GetNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("GetNVMfRemoteController: Received from client: %v", in)
	params := models.BdevNvmeGetControllerParams{
		Name: in.Name,
	}
	var result []models.BdevNvmeGetControllerResult
	err := server.Call("bdev_nvme_get_controllers", &params, &result)
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
	port, _ := strconv.ParseInt(result[0].Ctrlrs[0].Trid.Trsvcid, 10, 64)
	return &pb.NVMfRemoteController{
		Id:      &pc.ObjectKey{Value: result[0].Name},
		Hostnqn: result[0].Ctrlrs[0].Host.Nqn,
		Trtype:  pb.NvmeTransportType(pb.NvmeTransportType_value["NVME_TRANSPORT_"+strings.ToUpper(result[0].Ctrlrs[0].Trid.Trtype)]),
		Adrfam:  pb.NvmeAddressFamily(pb.NvmeAddressFamily_value["NVMF_ADRFAM_"+strings.ToUpper(result[0].Ctrlrs[0].Trid.Adrfam)]),
		Traddr:  result[0].Ctrlrs[0].Trid.Traddr,
		Subnqn:  result[0].Ctrlrs[0].Trid.Subnqn,
		Trsvcid: port,
	}, nil
}

// NVMfRemoteControllerStats gets NVMf remote controller stats
func (s *Server) NVMfRemoteControllerStats(ctx context.Context, in *pb.NVMfRemoteControllerStatsRequest) (*pb.NVMfRemoteControllerStatsResponse, error) {
	log.Printf("Received: %v", in.GetId())
	return &pb.NVMfRemoteControllerStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNullDebug creates a Null Debug instance
func (s *Server) CreateNullDebug(ctx context.Context, in *pb.CreateNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("CreateNullDebug: Received from client: %v", in)
	params := models.BdevNullCreateParams{
		Name:      in.NullDebug.Handle.Value,
		BlockSize: 512,
		NumBlocks: 64,
	}
	var result models.BdevNullCreateResult
	err := server.Call("bdev_null_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	response := &pb.NullDebug{}
	err = deepcopier.Copy(in.NullDebug).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// DeleteNullDebug deletes a Null Debug instance
func (s *Server) DeleteNullDebug(ctx context.Context, in *pb.DeleteNullDebugRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNullDebug: Received from client: %v", in)
	params := models.BdevNullDeleteParams{
		Name: in.Name,
	}
	var result models.BdevNullDeleteResult
	err := server.Call("bdev_null_delete", &params, &result)
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

// UpdateNullDebug updates a Null Debug instance
func (s *Server) UpdateNullDebug(ctx context.Context, in *pb.UpdateNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("UpdateNullDebug: Received from client: %v", in)
	params1 := models.BdevNullDeleteParams{
		Name: in.NullDebug.Handle.Value,
	}
	var result1 models.BdevNullDeleteResult
	err1 := server.Call("bdev_null_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		log.Printf("Could not delete: %v", in)
	}
	params2 := models.BdevNullCreateParams{
		Name:      in.NullDebug.Handle.Value,
		BlockSize: 512,
		NumBlocks: 64,
	}
	var result2 models.BdevNullCreateResult
	err2 := server.Call("bdev_null_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	response := &pb.NullDebug{}
	err3 := deepcopier.Copy(in.NullDebug).To(response)
	if err3 != nil {
		log.Printf("error: %v", err3)
		return nil, err3
	}
	return response, nil
}

// ListNullDebugs lists Null Debug instances
func (s *Server) ListNullDebugs(ctx context.Context, in *pb.ListNullDebugsRequest) (*pb.ListNullDebugsResponse, error) {
	log.Printf("ListNullDebugs: Received from client: %v", in)
	var result []models.BdevGetBdevsResult
	err := server.Call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.NullDebug, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NullDebug{Handle: &pc.ObjectKey{Value: r.Name}, Uuid: &pc.Uuid{Value: r.UUID}}
	}
	return &pb.ListNullDebugsResponse{NullDebugs: Blobarray}, nil
}

// GetNullDebug gets a a Null Debug instance
func (s *Server) GetNullDebug(ctx context.Context, in *pb.GetNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("GetNullDebug: Received from client: %v", in)
	params := models.BdevGetBdevsParams{
		Name: in.Name,
	}
	var result []models.BdevGetBdevsResult
	err := server.Call("bdev_get_bdevs", &params, &result)
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
	return &pb.NullDebug{Handle: &pc.ObjectKey{Value: result[0].Name}, Uuid: &pc.Uuid{Value: result[0].UUID}}, nil
}

// NullDebugStats gets a Null Debug instance stats
func (s *Server) NullDebugStats(ctx context.Context, in *pb.NullDebugStatsRequest) (*pb.NullDebugStatsResponse, error) {
	log.Printf("NullDebugStats: Received from client: %v", in)
	params := models.BdevGetIostatParams{
		Name: in.Handle.Value,
	}
	// See https://mholt.github.io/json-to-go/
	var result models.BdevGetIostatResult
	err := server.Call("bdev_get_iostat", &params, &result)
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

// CreateAioController creates an Aio controller
func (s *Server) CreateAioController(ctx context.Context, in *pb.CreateAioControllerRequest) (*pb.AioController, error) {
	log.Printf("CreateAioController: Received from client: %v", in)
	params := models.BdevAioCreateParams{
		Name:      in.AioController.Handle.Value,
		BlockSize: 512,
		Filename:  in.AioController.Filename,
	}
	var result models.BdevAioCreateResult
	err := server.Call("bdev_aio_create", &params, &result)
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
	return response, nil
}

// DeleteAioController deletes an Aio controller
func (s *Server) DeleteAioController(ctx context.Context, in *pb.DeleteAioControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteAioController: Received from client: %v", in)
	params := models.BdevAioDeleteParams{
		Name: in.Name,
	}
	var result models.BdevAioDeleteResult
	err := server.Call("bdev_aio_delete", &params, &result)
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
	err1 := server.Call("bdev_aio_delete", &params1, &result1)
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
	err2 := server.Call("bdev_aio_create", &params2, &result2)
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
	err := server.Call("bdev_get_bdevs", nil, &result)
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
	err := server.Call("bdev_get_bdevs", &params, &result)
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
	err := server.Call("bdev_get_iostat", &params, &result)
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
