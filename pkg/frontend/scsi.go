// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

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

// CreateVirtioScsiController creates a Virtio SCSI controller
func (s *Server) CreateVirtioScsiController(_ context.Context, in *pb.CreateVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("CreateVirtioScsiController: Received from client: %v", in)
	params := models.VhostCreateScsiControllerParams{
		Ctrlr: in.VirtioScsiController.Id.Value,
	}
	var result models.VhostCreateScsiControllerResult
	err := s.rpc.Call("vhost_create_scsi_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
	}
	response := &pb.VirtioScsiController{}
	err = deepcopier.Copy(in.VirtioScsiController).To(response)
	if err != nil {
		log.Printf("Error at response creation: %v", err)
		return nil, status.Error(codes.Internal, "Failed to construct device create response")
	}
	return response, nil
}

// DeleteVirtioScsiController deletes a Virtio SCSI controller
func (s *Server) DeleteVirtioScsiController(_ context.Context, in *pb.DeleteVirtioScsiControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioScsiController: Received from client: %v", in)
	params := models.VhostDeleteControllerParams{
		Ctrlr: in.Name,
	}
	var result models.VhostDeleteControllerResult
	err := s.rpc.Call("vhost_delete_controller", &params, &result)
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

// UpdateVirtioScsiController updates a Virtio SCSI controller
func (s *Server) UpdateVirtioScsiController(_ context.Context, in *pb.UpdateVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiController{}, nil
}

// ListVirtioScsiControllers lists Virtio SCSI controllers
func (s *Server) ListVirtioScsiControllers(_ context.Context, in *pb.ListVirtioScsiControllersRequest) (*pb.ListVirtioScsiControllersResponse, error) {
	log.Printf("ListVirtioScsiControllers: Received from client: %v", in)
	var result []models.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if in.PageSize > 0 && int(in.PageSize) < len(result) {
		log.Printf("Limiting result to: %d", in.PageSize)
		result = result[:in.PageSize]
	}
	Blobarray := make([]*pb.VirtioScsiController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiController{Id: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.ListVirtioScsiControllersResponse{VirtioScsiControllers: Blobarray}, nil
}

// GetVirtioScsiController gets a Virtio SCSI controller
func (s *Server) GetVirtioScsiController(_ context.Context, in *pb.GetVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("GetVirtioScsiController: Received from client: %v", in)
	params := models.VhostGetControllersParams{
		Name: in.Name,
	}
	var result []models.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", &params, &result)
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
	return &pb.VirtioScsiController{Id: &pc.ObjectKey{Value: result[0].Ctrlr}}, nil
}

// VirtioScsiControllerStats gets a Virtio SCSI controller stats
func (s *Server) VirtioScsiControllerStats(_ context.Context, in *pb.VirtioScsiControllerStatsRequest) (*pb.VirtioScsiControllerStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiControllerStatsResponse{}, nil
}

// CreateVirtioScsiLun creates a Virtio SCSI LUN
func (s *Server) CreateVirtioScsiLun(_ context.Context, in *pb.CreateVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("CreateVirtioScsiLun: Received from client: %v", in)
	params := struct {
		Name string `json:"ctrlr"`
		Num  int    `json:"scsi_target_num"`
		Bdev string `json:"bdev_name"`
	}{
		Name: in.VirtioScsiLun.TargetId.Value,
		Num:  5,
		Bdev: in.VirtioScsiLun.VolumeId.Value,
	}
	var result int
	err := s.rpc.Call("vhost_scsi_controller_add_target", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.VirtioScsiLun{}, nil
}

// DeleteVirtioScsiLun deletes a Virtio SCSI LUN
func (s *Server) DeleteVirtioScsiLun(_ context.Context, in *pb.DeleteVirtioScsiLunRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioScsiLun: Received from client: %v", in)
	params := struct {
		Name string `json:"ctrlr"`
		Num  int    `json:"scsi_target_num"`
	}{
		Name: in.Name,
		Num:  5,
	}
	var result bool
	err := s.rpc.Call("vhost_scsi_controller_remove_target", &params, &result)
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

// UpdateVirtioScsiLun updates a Virtio SCSI LUN
func (s *Server) UpdateVirtioScsiLun(_ context.Context, in *pb.UpdateVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiLun{}, nil
}

// ListVirtioScsiLuns lists Virtio SCSI LUNs
func (s *Server) ListVirtioScsiLuns(_ context.Context, in *pb.ListVirtioScsiLunsRequest) (*pb.ListVirtioScsiLunsResponse, error) {
	log.Printf("ListVirtioScsiLuns: Received from client: %v", in)
	var result []models.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if in.PageSize > 0 && int(in.PageSize) < len(result) {
		log.Printf("Limiting result to: %d", in.PageSize)
		result = result[:in.PageSize]
	}
	Blobarray := make([]*pb.VirtioScsiLun, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiLun{VolumeId: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.ListVirtioScsiLunsResponse{VirtioScsiLuns: Blobarray}, nil
}

// GetVirtioScsiLun gets a Virtio SCSI LUN
func (s *Server) GetVirtioScsiLun(_ context.Context, in *pb.GetVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("GetVirtioScsiLun: Received from client: %v", in)
	params := models.VhostGetControllersParams{
		Name: in.Name,
	}
	var result []models.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", &params, &result)
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
	return &pb.VirtioScsiLun{VolumeId: &pc.ObjectKey{Value: result[0].Ctrlr}}, nil
}

// VirtioScsiLunStats gets a Virtio SCSI LUN stats
func (s *Server) VirtioScsiLunStats(_ context.Context, in *pb.VirtioScsiLunStatsRequest) (*pb.VirtioScsiLunStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiLunStatsResponse{}, nil
}
