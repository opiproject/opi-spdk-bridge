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
