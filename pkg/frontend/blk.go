// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
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
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"github.com/ulule/deepcopier"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateVirtioBlk creates a Virtio block device
func (s *Server) CreateVirtioBlk(_ context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("CreateVirtioBlk: Received from client: %v", in)
	// idempotent API when called with same key, should return same object
	controller, ok := s.Virt.BlkCtrls[in.VirtioBlk.Id.Value]
	if ok {
		log.Printf("Already existing NVMeController with id %v", in.VirtioBlk.Id.Value)
		return controller, nil
	}
	// not found, so create a new one
	params := models.VhostCreateBlkControllerParams{
		Ctrlr:   in.VirtioBlk.Id.Value,
		DevName: in.VirtioBlk.VolumeId.Value,
	}
	var result models.VhostCreateBlkControllerResult
	err := s.rpc.Call("vhost_create_blk_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, fmt.Errorf("%w for %v", server.ErrFailedSpdkCall, in)
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
		return nil, fmt.Errorf("%w for %v", server.ErrUnexpectedSpdkCallResult, in)
	}
	s.Virt.BlkCtrls[in.VirtioBlk.Id.Value] = in.VirtioBlk
	// s.VirtioCtrls[in.VirtioBlk.Id.Value].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.VirtioBlk{}
	err = deepcopier.Copy(in.VirtioBlk).To(response)
	if err != nil {
		log.Printf("Error at response creation: %v", err)
		return nil, status.Error(codes.Internal, "Failed to construct device create response")
	}
	return response, nil
}

// DeleteVirtioBlk deletes a Virtio block device
func (s *Server) DeleteVirtioBlk(_ context.Context, in *pb.DeleteVirtioBlkRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioBlk: Received from client: %v", in)
	controller, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		return nil, fmt.Errorf("unable to find key %s", in.Name)
	}
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
	delete(s.Virt.BlkCtrls, controller.Id.Value)
	return &emptypb.Empty{}, nil
}

// UpdateVirtioBlk updates a Virtio block device
func (s *Server) UpdateVirtioBlk(_ context.Context, in *pb.UpdateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("UpdateVirtioBlk: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateVirtioBlk method is not implemented")
}

// ListVirtioBlks lists Virtio block devices
func (s *Server) ListVirtioBlks(_ context.Context, in *pb.ListVirtioBlksRequest) (*pb.ListVirtioBlksResponse, error) {
	log.Printf("ListVirtioBlks: Received from client: %v", in)
	var result []models.VhostGetControllersResult
	err := s.rpc.Call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.VirtioBlk, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioBlk{
			Id:       &pc.ObjectKey{Value: r.Ctrlr},
			PcieId:   &pb.PciEndpoint{PhysicalFunction: 1},
			VolumeId: &pc.ObjectKey{Value: "TBD"}}
	}
	return &pb.ListVirtioBlksResponse{VirtioBlks: Blobarray}, nil
}

// GetVirtioBlk gets a Virtio block device
func (s *Server) GetVirtioBlk(_ context.Context, in *pb.GetVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("GetVirtioBlk: Received from client: %v", in)
	_, ok := s.Virt.BlkCtrls[in.Name]
	if !ok {
		msg := fmt.Sprintf("Could not find Controller: %s", in.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
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
	return &pb.VirtioBlk{
		Id:       &pc.ObjectKey{Value: result[0].Ctrlr},
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 1},
		VolumeId: &pc.ObjectKey{Value: "TBD"}}, nil
}

// VirtioBlkStats gets a Virtio block device stats
func (s *Server) VirtioBlkStats(_ context.Context, in *pb.VirtioBlkStatsRequest) (*pb.VirtioBlkStatsResponse, error) {
	log.Printf("VirtioBlkStats: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "VirtioBlkStats method is not implemented")
}
