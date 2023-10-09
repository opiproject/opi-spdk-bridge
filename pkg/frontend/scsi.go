// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortScsiControllers(controllers []*pb.VirtioScsiController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Name < controllers[j].Name
	})
}

// CreateVirtioScsiController creates a Virtio SCSI controller
func (s *Server) CreateVirtioScsiController(ctx context.Context, in *pb.CreateVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.VirtioScsiControllerId != "" {
		err := resourceid.ValidateUserSettable(in.VirtioScsiControllerId)
		if err != nil {
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioScsiControllerId, in.VirtioScsiController.Name)
		resourceID = in.VirtioScsiControllerId
	}
	in.VirtioScsiController.Name = utils.ResourceIDToVolumeName(resourceID)

	// idempotent API when called with same key, should return same object
	controller, ok := s.Virt.ScsiCtrls[in.VirtioScsiController.Name]
	if ok {
		log.Printf("Already existing VirtioScsiController with id %v", in.VirtioScsiController.Name)
		return controller, nil
	}
	// not found, so create a new one
	params := spdk.VhostCreateScsiControllerParams{
		Ctrlr: resourceID,
	}
	var result spdk.VhostCreateScsiControllerResult
	err := s.rpc.Call(ctx, "vhost_create_scsi_controller", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
	}
	response := utils.ProtoClone(in.VirtioScsiController)
	// response.Status = &pb.VirtioScsiControllerStatus{Active: true}
	s.Virt.ScsiCtrls[in.VirtioScsiController.Name] = response
	return response, nil
}

// DeleteVirtioScsiController deletes a Virtio SCSI controller
func (s *Server) DeleteVirtioScsiController(ctx context.Context, in *pb.DeleteVirtioScsiControllerRequest) (*emptypb.Empty, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller, ok := s.Virt.ScsiCtrls[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(controller.Name)
	params := spdk.VhostDeleteControllerParams{
		Ctrlr: resourceID,
	}
	var result spdk.VhostDeleteControllerResult
	err := s.rpc.Call(ctx, "vhost_delete_controller", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	delete(s.Virt.ScsiCtrls, controller.Name)
	return &emptypb.Empty{}, nil
}

// UpdateVirtioScsiController updates a Virtio SCSI controller
func (s *Server) UpdateVirtioScsiController(_ context.Context, in *pb.UpdateVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.VirtioScsiController.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.ScsiCtrls[in.VirtioScsiController.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.VirtioScsiController.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.VirtioScsiController); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return &pb.VirtioScsiController{}, nil
}

// ListVirtioScsiControllers lists Virtio SCSI controllers
func (s *Server) ListVirtioScsiControllers(ctx context.Context, in *pb.ListVirtioScsiControllersRequest) (*pb.ListVirtioScsiControllersResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}
	var result []spdk.VhostGetControllersResult
	err := s.rpc.Call(ctx, "vhost_get_controllers", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
	result, hasMoreElements := utils.LimitPagination(result, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.VirtioScsiController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiController{Name: utils.ResourceIDToVolumeName(r.Ctrlr)}
	}
	sortScsiControllers(Blobarray)
	return &pb.ListVirtioScsiControllersResponse{VirtioScsiControllers: Blobarray, NextPageToken: token}, nil
}

// GetVirtioScsiController gets a Virtio SCSI controller
func (s *Server) GetVirtioScsiController(ctx context.Context, in *pb.GetVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.ScsiCtrls[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.VhostGetControllersParams{
		Name: resourceID,
	}
	var result []spdk.VhostGetControllersResult
	err := s.rpc.Call(ctx, "vhost_get_controllers", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result))
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.VirtioScsiController{Name: utils.ResourceIDToVolumeName(result[0].Ctrlr)}, nil
}

// StatsVirtioScsiController gets a Virtio SCSI controller stats
func (s *Server) StatsVirtioScsiController(_ context.Context, in *pb.StatsVirtioScsiControllerRequest) (*pb.StatsVirtioScsiControllerResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.ScsiCtrls[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	return &pb.StatsVirtioScsiControllerResponse{}, nil
}

// CreateVirtioScsiLun creates a Virtio SCSI LUN
func (s *Server) CreateVirtioScsiLun(ctx context.Context, in *pb.CreateVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.VirtioScsiLun.VolumeNameRef); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.VirtioScsiLunId != "" {
		err := resourceid.ValidateUserSettable(in.VirtioScsiLunId)
		if err != nil {
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioScsiLunId, in.VirtioScsiLun.Name)
		resourceID = in.VirtioScsiLunId
	}
	in.VirtioScsiLun.Name = utils.ResourceIDToVolumeName(resourceID)

	// idempotent API when called with same key, should return same object
	lun, ok := s.Virt.ScsiLuns[in.VirtioScsiLun.Name]
	if ok {
		log.Printf("Already existing VirtioScsiLun with id %v", in.VirtioScsiLun.Name)
		return lun, nil
	}
	// not found, so create a new one
	params := struct {
		Name string `json:"ctrlr"`
		Num  int    `json:"scsi_target_num"`
		Bdev string `json:"bdev_name"`
	}{
		Name: resourceID,
		Num:  5,
		Bdev: in.VirtioScsiLun.VolumeNameRef,
	}
	var result int
	err := s.rpc.Call(ctx, "vhost_scsi_controller_add_target", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	response := utils.ProtoClone(in.VirtioScsiLun)
	// response.Status = &pb.VirtioScsiLunStatus{Active: true}
	s.Virt.ScsiLuns[in.VirtioScsiLun.Name] = response
	return response, nil
}

// DeleteVirtioScsiLun deletes a Virtio SCSI LUN
func (s *Server) DeleteVirtioScsiLun(ctx context.Context, in *pb.DeleteVirtioScsiLunRequest) (*emptypb.Empty, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	lun, ok := s.Virt.ScsiLuns[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(lun.Name)
	params := struct {
		Name string `json:"ctrlr"`
		Num  int    `json:"scsi_target_num"`
	}{
		Name: resourceID,
		Num:  5,
	}
	var result bool
	err := s.rpc.Call(ctx, "vhost_scsi_controller_remove_target", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	delete(s.Virt.ScsiLuns, lun.Name)
	return &emptypb.Empty{}, nil
}

// UpdateVirtioScsiLun updates a Virtio SCSI LUN
func (s *Server) UpdateVirtioScsiLun(_ context.Context, in *pb.UpdateVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.VirtioScsiLun.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.ScsiLuns[in.VirtioScsiLun.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.VirtioScsiLun.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.VirtioScsiLun); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return &pb.VirtioScsiLun{}, nil
}

// ListVirtioScsiLuns lists Virtio SCSI LUNs
func (s *Server) ListVirtioScsiLuns(ctx context.Context, in *pb.ListVirtioScsiLunsRequest) (*pb.ListVirtioScsiLunsResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}
	var result []spdk.VhostGetControllersResult
	err := s.rpc.Call(ctx, "vhost_get_controllers", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
	result, hasMoreElements := utils.LimitPagination(result, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.VirtioScsiLun, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiLun{
			VolumeNameRef: utils.ResourceIDToVolumeName(r.Ctrlr),
		}
	}
	return &pb.ListVirtioScsiLunsResponse{VirtioScsiLuns: Blobarray, NextPageToken: token}, nil
}

// GetVirtioScsiLun gets a Virtio SCSI LUN
func (s *Server) GetVirtioScsiLun(ctx context.Context, in *pb.GetVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.ScsiLuns[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.VhostGetControllersParams{
		Name: resourceID,
	}
	var result []spdk.VhostGetControllersResult
	err := s.rpc.Call(ctx, "vhost_get_controllers", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result))
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.VirtioScsiLun{VolumeNameRef: utils.ResourceIDToVolumeName(result[0].Ctrlr)}, nil
}

// StatsVirtioScsiLun gets a Virtio SCSI LUN stats
func (s *Server) StatsVirtioScsiLun(_ context.Context, in *pb.StatsVirtioScsiLunRequest) (*pb.StatsVirtioScsiLunResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.ScsiLuns[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	return &pb.StatsVirtioScsiLunResponse{}, nil
}
