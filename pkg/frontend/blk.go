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

	"github.com/google/uuid"
	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"github.com/ulule/deepcopier"

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
	// see https://google.aip.dev/133#user-specified-ids
	name := uuid.New().String()
	if in.VirtioBlkId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioBlkId, in.VirtioBlk.Name)
		name = in.VirtioBlkId
	}
	in.VirtioBlk.Name = name
	// idempotent API when called with same key, should return same object
	controller, ok := s.Virt.BlkCtrls[in.VirtioBlk.Name]
	if ok {
		log.Printf("Already existing NvmeController with id %v", in.VirtioBlk.Name)
		return controller, nil
	}
	// not found, so create a new one
	params := spdk.VhostCreateBlkControllerParams{
		Ctrlr:   name,
		DevName: in.VirtioBlk.VolumeId.Value,
	}
	var result spdk.VhostCreateBlkControllerResult
	err := s.rpc.Call("vhost_create_blk_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, fmt.Errorf("%w for %v", spdk.ErrFailedSpdkCall, in)
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
		return nil, fmt.Errorf("%w for %v", spdk.ErrUnexpectedSpdkCallResult, in)
	}
	s.Virt.BlkCtrls[in.VirtioBlk.Name] = in.VirtioBlk
	// s.VirtioCtrls[in.VirtioBlk.Name].Status = &pb.NvmeControllerStatus{Active: true}
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
	delete(s.Virt.BlkCtrls, controller.Name)
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
			Name:     r.Ctrlr,
			PcieId:   &pb.PciEndpoint{PhysicalFunction: 1},
			VolumeId: &pc.ObjectKey{Value: "TBD"}}
	}
	sortVirtioBlks(Blobarray)

	return &pb.ListVirtioBlksResponse{VirtioBlks: Blobarray, NextPageToken: token}, nil
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
	return &pb.VirtioBlk{
		Name:     result[0].Ctrlr,
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 1},
		VolumeId: &pc.ObjectKey{Value: "TBD"}}, nil
}

// VirtioBlkStats gets a Virtio block device stats
func (s *Server) VirtioBlkStats(_ context.Context, in *pb.VirtioBlkStatsRequest) (*pb.VirtioBlkStatsResponse, error) {
	log.Printf("VirtioBlkStats: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "VirtioBlkStats method is not implemented")
}
