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
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortVirtioBlks(virtioBlks []*pb.VirtioBlk) {
	sort.Slice(virtioBlks, func(i int, j int) bool {
		return virtioBlks[i].Name < virtioBlks[j].Name
	})
}

// CreateVirtioBlk creates a Virtio block device
func (s *Server) CreateVirtioBlk(_ context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("CreateVirtioBlk: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.VirtioBlkId != "" {
		err := resourceid.ValidateUserSettable(in.VirtioBlkId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioBlkId, in.VirtioBlk.Name)
		resourceID = in.VirtioBlkId
	}
	in.VirtioBlk.Name = server.ResourceIDToVolumeName(resourceID)

	// idempotent API when called with same key, should return same object
	controller, ok := s.Virt.BlkCtrls[in.VirtioBlk.Name]
	if ok {
		log.Printf("Already existing NvmeController with id %v", in.VirtioBlk.Name)
		return controller, nil
	}
	// not found, so create a new one
	params := spdk.VhostCreateBlkControllerParams{
		Ctrlr:   resourceID,
		DevName: in.VirtioBlk.VolumeNameRef,
	}
	var result spdk.VhostCreateBlkControllerResult
	err := s.rpc.Call("vhost_create_blk_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not create virtio-blk: %s", resourceID)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := server.ProtoClone(in.VirtioBlk)
	// response.Status = &pb.NvmeControllerStatus{Active: true}
	s.Virt.BlkCtrls[in.VirtioBlk.Name] = response
	return response, nil
}

// DeleteVirtioBlk deletes a Virtio block device
func (s *Server) DeleteVirtioBlk(_ context.Context, in *pb.DeleteVirtioBlkRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioBlk: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	controller, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(controller.Name)
	params := spdk.VhostDeleteControllerParams{
		Ctrlr: resourceID,
	}
	var result spdk.VhostDeleteControllerResult
	err := s.rpc.Call("vhost_delete_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete virtio-blk: %s", resourceID)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Virt.BlkCtrls, controller.Name)
	return &emptypb.Empty{}, nil
}

// UpdateVirtioBlk updates a Virtio block device
func (s *Server) UpdateVirtioBlk(_ context.Context, in *pb.UpdateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("UpdateVirtioBlk: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.VirtioBlk.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.BlkCtrls[in.VirtioBlk.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.VirtioBlk.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.VirtioBlk); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "UpdateVirtioBlk method is not implemented")
}

// ListVirtioBlks lists Virtio block devices
func (s *Server) ListVirtioBlks(_ context.Context, in *pb.ListVirtioBlksRequest) (*pb.ListVirtioBlksResponse, error) {
	log.Printf("ListVirtioBlks: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
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
	Blobarray := make([]*pb.VirtioBlk, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioBlk{
			Name:          server.ResourceIDToVolumeName(r.Ctrlr),
			PcieId:        &pb.PciEndpoint{PhysicalFunction: 1},
			VolumeNameRef: "TBD"}
	}
	sortVirtioBlks(Blobarray)

	return &pb.ListVirtioBlksResponse{VirtioBlks: Blobarray, NextPageToken: token}, nil
}

// GetVirtioBlk gets a Virtio block device
func (s *Server) GetVirtioBlk(_ context.Context, in *pb.GetVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("GetVirtioBlk: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.VhostGetControllersParams{
		Name: resourceID,
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
	return &pb.VirtioBlk{
		Name:          in.Name,
		PcieId:        &pb.PciEndpoint{PhysicalFunction: 1},
		VolumeNameRef: "TBD"}, nil
}

// StatsVirtioBlk gets a Virtio block device stats
func (s *Server) StatsVirtioBlk(_ context.Context, in *pb.StatsVirtioBlkRequest) (*pb.StatsVirtioBlkResponse, error) {
	log.Printf("StatsVirtioBlk: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "StatsVirtioBlk method is not implemented")
}
