// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

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

func sortScsiControllers(controllers []*pb.VirtioScsiController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Id.Value < controllers[j].Id.Value
	})
}

// CreateVirtioScsiController creates a Virtio SCSI controller
func (s *Server) CreateVirtioScsiController(_ context.Context, in *pb.CreateVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("CreateVirtioScsiController: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	name := uuid.New().String()
	if in.VirtioScsiControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioScsiControllerId, in.VirtioScsiController.Id.Value)
		name = in.VirtioScsiControllerId
	}
	in.VirtioScsiController.Id.Value = name
	// idempotent API when called with same key, should return same object
	controller, ok := s.Virt.ScsiCtrls[in.VirtioScsiController.Id.Value]
	if ok {
		log.Printf("Already existing VirtioScsiController with id %v", in.VirtioScsiController.Id.Value)
		return controller, nil
	}
	// not found, so create a new one
	params := spdk.VhostCreateScsiControllerParams{
		Ctrlr: in.VirtioScsiController.Id.Value,
	}
	var result spdk.VhostCreateScsiControllerResult
	err := s.rpc.Call("vhost_create_scsi_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
	}
	s.Virt.ScsiCtrls[in.VirtioScsiController.Id.Value] = in.VirtioScsiController
	// s.VirtioCtrls[in.VirtioScsiController.Id.Value].Status = &pb.VirtioScsiControllerStatus{Active: true}
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
	controller, ok := s.Virt.ScsiCtrls[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.VhostDeleteControllerParams{
		Ctrlr: in.Name,
	}
	var result spdk.VhostDeleteControllerResult
	err := s.rpc.Call("vhost_delete_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	delete(s.Virt.ScsiCtrls, controller.Id.Value)
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
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
	var result []spdk.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", nil, &result)
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
	Blobarray := make([]*pb.VirtioScsiController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiController{Id: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	sortScsiControllers(Blobarray)
	return &pb.ListVirtioScsiControllersResponse{VirtioScsiControllers: Blobarray, NextPageToken: token}, nil
}

// GetVirtioScsiController gets a Virtio SCSI controller
func (s *Server) GetVirtioScsiController(_ context.Context, in *pb.GetVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("GetVirtioScsiController: Received from client: %v", in)
	params := spdk.VhostGetControllersParams{
		Name: in.Name,
	}
	var result []spdk.VhostGetControllersResult
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
	// see https://google.aip.dev/133#user-specified-ids
	name := uuid.New().String()
	if in.VirtioScsiLunId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioScsiLunId, in.VirtioScsiLun.Id.Value)
		name = in.VirtioScsiLunId
	}
	in.VirtioScsiLun.Id.Value = name
	// idempotent API when called with same key, should return same object
	lun, ok := s.Virt.ScsiLuns[in.VirtioScsiLun.Id.Value]
	if ok {
		log.Printf("Already existing VirtioScsiLun with id %v", in.VirtioScsiLun.Id.Value)
		return lun, nil
	}
	// not found, so create a new one
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
	s.Virt.ScsiLuns[in.VirtioScsiLun.Id.Value] = in.VirtioScsiLun
	// s.ScsiLuns[in.VirtioScsiLun.Id.Value].Status = &pb.VirtioScsiLunStatus{Active: true}
	return &pb.VirtioScsiLun{}, nil
}

// DeleteVirtioScsiLun deletes a Virtio SCSI LUN
func (s *Server) DeleteVirtioScsiLun(_ context.Context, in *pb.DeleteVirtioScsiLunRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioScsiLun: Received from client: %v", in)
	lun, ok := s.Virt.ScsiLuns[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
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
	delete(s.Virt.ScsiLuns, lun.Id.Value)
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
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
	var result []spdk.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", nil, &result)
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
	Blobarray := make([]*pb.VirtioScsiLun, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiLun{VolumeId: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.ListVirtioScsiLunsResponse{VirtioScsiLuns: Blobarray, NextPageToken: token}, nil
}

// GetVirtioScsiLun gets a Virtio SCSI LUN
func (s *Server) GetVirtioScsiLun(_ context.Context, in *pb.GetVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("GetVirtioScsiLun: Received from client: %v", in)
	params := spdk.VhostGetControllersParams{
		Name: in.Name,
	}
	var result []spdk.VhostGetControllersResult
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
