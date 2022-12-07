// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// The main package of the storage server
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

//////////////////////////////////////////////////////////

func (s *server) CreateNVMfRemoteController(ctx context.Context, in *pb.CreateNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("CreateNVMfRemoteController: Received from client: %v", in)
	params := BdevNvmeAttachControllerParams{
		Name:    in.Ctrl.Id.Value,
		Trtype:  strings.ReplaceAll(in.Ctrl.Trtype.String(), "NVME_TRANSPORT_", ""),
		Traddr:  in.Ctrl.Traddr,
		Adrfam:  strings.ReplaceAll(in.Ctrl.Adrfam.String(), "NVMF_ADRFAM_", ""),
		Trsvcid: fmt.Sprint(in.Ctrl.Trsvcid),
		Subnqn:  in.Ctrl.Subnqn,
		Hostnqn: in.Ctrl.Hostnqn,
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
	response := &pb.NVMfRemoteController{}
	err = deepcopier.Copy(in.Ctrl).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteNVMfRemoteController(ctx context.Context, in *pb.DeleteNVMfRemoteControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMfRemoteController: Received from client: %v", in)
	params := BdevNvmeDetachControllerParams{
		Name: in.Id.Value,
	}
	var result BdevNvmeDetachControllerResult
	err := call("bdev_nvme_detach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &emptypb.Empty{}, nil
}

func (s *server) NVMfRemoteControllerReset(ctx context.Context, in *pb.NVMfRemoteControllerResetRequest) (*emptypb.Empty, error) {
	log.Printf("Received: %v", in.GetId())
	return &emptypb.Empty{}, nil
}

func (s *server) ListNVMfRemoteController(ctx context.Context, in *pb.ListNVMfRemoteControllerRequest) (*pb.ListNVMfRemoteControllerResponse, error) {
	log.Printf("ListNVMfRemoteController: Received from client: %v", in)
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
	return &pb.ListNVMfRemoteControllerResponse{Ctrls: Blobarray}, nil
}

func (s *server) GetNVMfRemoteController(ctx context.Context, in *pb.GetNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("GetNVMfRemoteController: Received from client: %v", in)
	params := BdevNvmeGetControllerParams{
		Name: in.Id.Value,
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

func (s *server) NVMfRemoteControllerStats(ctx context.Context, in *pb.NVMfRemoteControllerStatsRequest) (*pb.NVMfRemoteControllerStatsResponse, error) {
	log.Printf("Received: %v", in.GetId())
	return &pb.NVMfRemoteControllerStatsResponse{}, nil
}

//////////////////////////////////////////////////////////

func (s *server) CreateNullDebug(ctx context.Context, in *pb.CreateNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("CreateNullDebug: Received from client: %v", in)
	params := BdevNullCreateParams{
		Name:      in.Device.Handle.Value,
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
	response := &pb.NullDebug{}
	err = deepcopier.Copy(in.Device).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteNullDebug(ctx context.Context, in *pb.DeleteNullDebugRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNullDebug: Received from client: %v", in)
	params := BdevNullDeleteParams{
		Name: in.Handle.Value,
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
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateNullDebug(ctx context.Context, in *pb.UpdateNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("UpdateNullDebug: Received from client: %v", in)
	params1 := BdevNullDeleteParams{
		Name: in.Device.Handle.Value,
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
		Name:      in.Device.Handle.Value,
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
	response := &pb.NullDebug{}
	err3 := deepcopier.Copy(in.Device).To(response)
	if err3 != nil {
		log.Printf("error: %v", err3)
		return nil, err3
	}
	return response, nil
}

func (s *server) ListNullDebug(ctx context.Context, in *pb.ListNullDebugRequest) (*pb.ListNullDebugResponse, error) {
	log.Printf("ListNullDebug: Received from client: %v", in)
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
		Blobarray[i] = &pb.NullDebug{Handle: &pc.ObjectKey{Value: r.Name}, Uuid: &pc.Uuid{Value: r.UUID}}
	}
	return &pb.ListNullDebugResponse{Devices: Blobarray}, nil
}

func (s *server) GetNullDebug(ctx context.Context, in *pb.GetNullDebugRequest) (*pb.NullDebug, error) {
	log.Printf("GetNullDebug: Received from client: %v", in)
	params := BdevGetBdevsParams{
		Name: in.Handle.Value,
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
	return &pb.NullDebug{Handle: &pc.ObjectKey{Value: result[0].Name}, Uuid: &pc.Uuid{Value: result[0].UUID}}, nil
}

func (s *server) NullDebugStats(ctx context.Context, in *pb.NullDebugStatsRequest) (*pb.NullDebugStatsResponse, error) {
	log.Printf("NullDebugStats: Received from client: %v", in)
	params := BdevGetIostatParams{
		Name: in.Handle.Value,
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

func (s *server) CreateAioController(ctx context.Context, in *pb.CreateAioControllerRequest) (*pb.AioController, error) {
	log.Printf("CreateAioController: Received from client: %v", in)
	params := BdevAioCreateParams{
		Name:      in.GetDevice().GetHandle().GetValue(),
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
	response := &pb.AioController{}
	err = deepcopier.Copy(in.Device).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteAioController(ctx context.Context, in *pb.DeleteAioControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteAioController: Received from client: %v", in)
	params := BdevAioDeleteParams{
		Name: in.GetHandle().GetValue(),
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

func (s *server) UpdateAioController(ctx context.Context, in *pb.UpdateAioControllerRequest) (*pb.AioController, error) {
	log.Printf("UpdateAioController: Received from client: %v", in)
	params1 := BdevAioDeleteParams{
		Name: in.GetDevice().GetHandle().GetValue(),
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
		Name:      in.GetDevice().GetHandle().GetValue(),
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

func (s *server) ListAioController(ctx context.Context, in *pb.ListAioControllerRequest) (*pb.ListAioControllerResponse, error) {
	log.Printf("ListAioController: Received from client: %v", in)
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
		Blobarray[i] = &pb.AioController{Handle: &pc.ObjectKey{Value: r.Name}}
	}
	return &pb.ListAioControllerResponse{Devices: Blobarray}, nil
}

func (s *server) GetAioController(ctx context.Context, in *pb.GetAioControllerRequest) (*pb.AioController, error) {
	log.Printf("GetAioController: Received from client: %v", in)
	params := BdevGetBdevsParams{
		Name: in.GetHandle().GetValue(),
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
	return &pb.AioController{Handle: &pc.ObjectKey{Value: result[0].Name}}, nil
}

func (s *server) AioControllerStats(ctx context.Context, in *pb.AioControllerStatsRequest) (*pb.AioControllerStatsResponse, error) {
	log.Printf("AioControllerStats: Received from client: %v", in)
	params := BdevGetIostatParams{
		Name: in.GetHandle().GetValue(),
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
	return &pb.AioControllerStatsResponse{Stats: fmt.Sprint(result.Bdevs[0])}, nil
}

//////////////////////////////////////////////////////////
