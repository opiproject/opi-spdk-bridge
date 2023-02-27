// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"context"
	"fmt"
	"log"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"github.com/ulule/deepcopier"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	errFailedSpdkCall           = status.Error(codes.Unknown, "Failed to execute SPDK call")
	errUnexpectedSpdkCallResult = status.Error(codes.FailedPrecondition, "Unexpected SPDK call result.")
)

// Server contains frontend related OPI services
type Server struct {
	pb.UnimplementedFrontendNvmeServiceServer
	pb.UnimplementedFrontendVirtioBlkServiceServer
	pb.UnimplementedFrontendVirtioScsiServiceServer
}

// CreateVirtioBlk creates a Virtio block device
func (s *Server) CreateVirtioBlk(ctx context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("CreateVirtioBlk: Received from client: %v", in)
	params := server.VhostCreateBlkControllerParams{
		Ctrlr:   in.VirtioBlk.Id.Value,
		DevName: in.VirtioBlk.VolumeId.Value,
	}
	var result server.VhostCreateBlkControllerResult
	err := server.Call("vhost_create_blk_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, fmt.Errorf("%w for %v", errFailedSpdkCall, in)
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
		return nil, fmt.Errorf("%w for %v", errUnexpectedSpdkCallResult, in)
	}

	response := &pb.VirtioBlk{}
	err = deepcopier.Copy(in.VirtioBlk).To(response)
	if err != nil {
		log.Printf("Error at response creation: %v", err)
		return nil, status.Error(codes.Internal, "Failed to construct device create response")
	}
	return response, nil
}

// DeleteVirtioBlk deletes a Virtio block device
func (s *Server) DeleteVirtioBlk(ctx context.Context, in *pb.DeleteVirtioBlkRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioBlk: Received from client: %v", in)
	params := server.VhostDeleteControllerParams{
		Ctrlr: in.Name,
	}
	var result server.VhostDeleteControllerResult
	err := server.Call("vhost_delete_controller", &params, &result)
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

// UpdateVirtioBlk updates a Virtio block device
func (s *Server) UpdateVirtioBlk(ctx context.Context, in *pb.UpdateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioBlk{}, nil
}

// ListVirtioBlks lists Virtio block devices
func (s *Server) ListVirtioBlks(ctx context.Context, in *pb.ListVirtioBlksRequest) (*pb.ListVirtioBlksResponse, error) {
	log.Printf("ListVirtioBlks: Received from client: %v", in)
	var result []server.VhostGetControllersResult
	err := server.Call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.VirtioBlk, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioBlk{Id: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.ListVirtioBlksResponse{VirtioBlks: Blobarray}, nil
}

// GetVirtioBlk gets a Virtio block device
func (s *Server) GetVirtioBlk(ctx context.Context, in *pb.GetVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("GetVirtioBlk: Received from client: %v", in)
	params := server.VhostGetControllersParams{
		Name: in.Name,
	}
	var result []server.VhostGetControllersResult
	err := server.Call("vhost_get_controllers", &params, &result)
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
	return &pb.VirtioBlk{Id: &pc.ObjectKey{Value: result[0].Ctrlr}}, nil
}

// VirtioBlkStats gets a Virtio block device stats
func (s *Server) VirtioBlkStats(ctx context.Context, in *pb.VirtioBlkStatsRequest) (*pb.VirtioBlkStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioBlkStatsResponse{}, nil
}

// ////////////////////////////////////////////////////////

// CreateVirtioScsiController creates a Virtio SCSI controller
func (s *Server) CreateVirtioScsiController(ctx context.Context, in *pb.CreateVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("CreateVirtioScsiController: Received from client: %v", in)
	params := server.VhostCreateScsiControllerParams{
		Ctrlr: in.VirtioScsiController.Id.Value,
	}
	var result server.VhostCreateScsiControllerResult
	err := server.Call("vhost_create_scsi_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
	}
	return &pb.VirtioScsiController{}, nil
}

// DeleteVirtioScsiController deletes a Virtio SCSI controller
func (s *Server) DeleteVirtioScsiController(ctx context.Context, in *pb.DeleteVirtioScsiControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioScsiController: Received from client: %v", in)
	params := server.VhostDeleteControllerParams{
		Ctrlr: in.Name,
	}
	var result server.VhostDeleteControllerResult
	err := server.Call("vhost_delete_controller", &params, &result)
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
func (s *Server) UpdateVirtioScsiController(ctx context.Context, in *pb.UpdateVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiController{}, nil
}

// ListVirtioScsiControllers lists Virtio SCSI controllers
func (s *Server) ListVirtioScsiControllers(ctx context.Context, in *pb.ListVirtioScsiControllersRequest) (*pb.ListVirtioScsiControllersResponse, error) {
	log.Printf("ListVirtioScsiControllers: Received from client: %v", in)
	var result []server.VhostGetControllersResult
	err := server.Call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.VirtioScsiController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiController{Id: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.ListVirtioScsiControllersResponse{VirtioScsiControllers: Blobarray}, nil
}

// GetVirtioScsiController gets a Virtio SCSI controller
func (s *Server) GetVirtioScsiController(ctx context.Context, in *pb.GetVirtioScsiControllerRequest) (*pb.VirtioScsiController, error) {
	log.Printf("GetVirtioScsiController: Received from client: %v", in)
	params := server.VhostGetControllersParams{
		Name: in.Name,
	}
	var result []server.VhostGetControllersResult
	err := server.Call("vhost_get_controllers", &params, &result)
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
func (s *Server) VirtioScsiControllerStats(ctx context.Context, in *pb.VirtioScsiControllerStatsRequest) (*pb.VirtioScsiControllerStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiControllerStatsResponse{}, nil
}

// ////////////////////////////////////////////////////////

// CreateVirtioScsiLun creates a Virtio SCSI LUN
func (s *Server) CreateVirtioScsiLun(ctx context.Context, in *pb.CreateVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
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
	err := server.Call("vhost_scsi_controller_add_target", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.VirtioScsiLun{}, nil
}

// DeleteVirtioScsiLun deletes a Virtio SCSI LUN
func (s *Server) DeleteVirtioScsiLun(ctx context.Context, in *pb.DeleteVirtioScsiLunRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteVirtioScsiLun: Received from client: %v", in)
	params := struct {
		Name string `json:"ctrlr"`
		Num  int    `json:"scsi_target_num"`
	}{
		Name: in.Name,
		Num:  5,
	}
	var result bool
	err := server.Call("vhost_scsi_controller_remove_target", &params, &result)
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
func (s *Server) UpdateVirtioScsiLun(ctx context.Context, in *pb.UpdateVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiLun{}, nil
}

// ListVirtioScsiLuns lists Virtio SCSI LUNs
func (s *Server) ListVirtioScsiLuns(ctx context.Context, in *pb.ListVirtioScsiLunsRequest) (*pb.ListVirtioScsiLunsResponse, error) {
	log.Printf("ListVirtioScsiLuns: Received from client: %v", in)
	var result []server.VhostGetControllersResult
	err := server.Call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.VirtioScsiLun, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiLun{VolumeId: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.ListVirtioScsiLunsResponse{VirtioScsiLuns: Blobarray}, nil
}

// GetVirtioScsiLun gets a Virtio SCSI LUN
func (s *Server) GetVirtioScsiLun(ctx context.Context, in *pb.GetVirtioScsiLunRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("GetVirtioScsiLun: Received from client: %v", in)
	params := server.VhostGetControllersParams{
		Name: in.Name,
	}
	var result []server.VhostGetControllersResult
	err := server.Call("vhost_get_controllers", &params, &result)
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
func (s *Server) VirtioScsiLunStats(ctx context.Context, in *pb.VirtioScsiLunStatsRequest) (*pb.VirtioScsiLunStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiLunStatsResponse{}, nil
}
