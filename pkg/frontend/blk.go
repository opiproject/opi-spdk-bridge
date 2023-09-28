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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func sortVirtioBlks(virtioBlks []*pb.VirtioBlk) {
	sort.Slice(virtioBlks, func(i int, j int) bool {
		return virtioBlks[i].Name < virtioBlks[j].Name
	})
}

type vhostUserBlkTransport struct{}

// NewVhostUserBlkTransport creates objects to handle vhost user blk transport
// specifics
func NewVhostUserBlkTransport() VirtioBlkTransport {
	return &vhostUserBlkTransport{}
}

func (v vhostUserBlkTransport) CreateParams(virtioBlk *pb.VirtioBlk) (any, error) {
	v.verifyTransportSpecificParams(virtioBlk)

	resourceID := path.Base(virtioBlk.Name)
	return spdk.VhostCreateBlkControllerParams{
		Ctrlr:   resourceID,
		DevName: virtioBlk.VolumeNameRef,
	}, nil
}

func (v vhostUserBlkTransport) DeleteParams(virtioBlk *pb.VirtioBlk) (any, error) {
	v.verifyTransportSpecificParams(virtioBlk)

	resourceID := path.Base(virtioBlk.Name)
	return spdk.VhostDeleteControllerParams{
		Ctrlr: resourceID,
	}, nil
}

func (v vhostUserBlkTransport) verifyTransportSpecificParams(virtioBlk *pb.VirtioBlk) {
	pcieID := virtioBlk.PcieId
	if pcieID.PortId.Value != 0 {
		log.Printf("WARNING: only port 0 is supported for vhost user. Will be replaced with an error")
	}

	if pcieID.VirtualFunction.Value != 0 {
		log.Println("WARNING: virtual functions are not supported for vhost user. Will be replaced with an error")
	}
}

// CreateVirtioBlk creates a Virtio block device
func (s *Server) CreateVirtioBlk(_ context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	// check input correctness
	if err := s.validateCreateVirtioBlkRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.VirtioBlkId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioBlkId, in.VirtioBlk.Name)
		resourceID = in.VirtioBlkId
	}
	in.VirtioBlk.Name = utils.ResourceIDToVolumeName(resourceID)

	// idempotent API when called with same key, should return same object
	controller, ok := s.Virt.BlkCtrls[in.VirtioBlk.Name]
	if ok {
		log.Printf("Already existing NvmeController with id %v", in.VirtioBlk.Name)
		return controller, nil
	}
	// not found, so create a new one
	params, err := s.Virt.transport.CreateParams(in.VirtioBlk)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var result spdk.VhostCreateBlkControllerResult
	err = s.rpc.Call("vhost_create_blk_controller", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not create virtio-blk: %s", resourceID)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := utils.ProtoClone(in.VirtioBlk)
	// response.Status = &pb.NvmeControllerStatus{Active: true}
	s.Virt.BlkCtrls[in.VirtioBlk.Name] = response
	return response, nil
}

// DeleteVirtioBlk deletes a Virtio block device
func (s *Server) DeleteVirtioBlk(_ context.Context, in *pb.DeleteVirtioBlkRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteVirtioBlkRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}

	params, err := s.Virt.transport.DeleteParams(controller)
	if err != nil {
		log.Printf("error: failed to create params for spdk call: %v. Inconsistent entry in db?", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var result spdk.VhostDeleteControllerResult
	err = s.rpc.Call("vhost_delete_controller", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete virtio-blk: %s", in.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Virt.BlkCtrls, controller.Name)
	return &emptypb.Empty{}, nil
}

// UpdateVirtioBlk updates a Virtio block device
func (s *Server) UpdateVirtioBlk(_ context.Context, in *pb.UpdateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	// check input correctness
	if err := s.validateUpdateVirtioBlkRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.BlkCtrls[in.VirtioBlk.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.VirtioBlk.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.VirtioBlk); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "UpdateVirtioBlk method is not implemented")
}

// ListVirtioBlks lists Virtio block devices
func (s *Server) ListVirtioBlks(_ context.Context, in *pb.ListVirtioBlksRequest) (*pb.ListVirtioBlksResponse, error) {
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
	err := s.rpc.Call("vhost_get_controllers", nil, &result)
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
	Blobarray := make([]*pb.VirtioBlk, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioBlk{
			Name: utils.ResourceIDToVolumeName(r.Ctrlr),
			PcieId: &pb.PciEndpoint{
				PhysicalFunction: wrapperspb.Int32(1),
				VirtualFunction:  wrapperspb.Int32(0),
				PortId:           wrapperspb.Int32(0),
			},
			VolumeNameRef: "TBD"}
	}
	sortVirtioBlks(Blobarray)

	return &pb.ListVirtioBlksResponse{VirtioBlks: Blobarray, NextPageToken: token}, nil
}

// GetVirtioBlk gets a Virtio block device
func (s *Server) GetVirtioBlk(_ context.Context, in *pb.GetVirtioBlkRequest) (*pb.VirtioBlk, error) {
	// check input correctness
	if err := s.validateGetVirtioBlkRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	params := spdk.VhostGetControllersParams{
		Name: resourceID,
	}
	var result []spdk.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result))
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.VirtioBlk{
		Name: in.Name,
		PcieId: &pb.PciEndpoint{
			PhysicalFunction: wrapperspb.Int32(1),
			VirtualFunction:  wrapperspb.Int32(0),
			PortId:           wrapperspb.Int32(0),
		},
		VolumeNameRef: "TBD"}, nil
}

// StatsVirtioBlk gets a Virtio block device stats
func (s *Server) StatsVirtioBlk(_ context.Context, in *pb.StatsVirtioBlkRequest) (*pb.StatsVirtioBlkResponse, error) {
	log.Printf("StatsVirtioBlk: Received from client: %v", in)
	// check input correctness
	if err := s.validateStatsVirtioBlkRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "StatsVirtioBlk method is not implemented")
}
