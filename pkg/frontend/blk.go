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

	"github.com/google/uuid"
	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// VirtioBlkQosProvider provides QoS capabilities for virtio-blk
// At the moment it is just a couple of methods from middleend QoS service, but
// it can be changed to less verbose later.
// If client uses VirtioBlkQosProviderFromMiddleendQosService to create an instance,
// the interface can be changed without affecting the client code.
type VirtioBlkQosProvider interface {
	CreateQosVolume(context.Context, *pb.CreateQosVolumeRequest) (*pb.QosVolume, error)
	DeleteQosVolume(context.Context, *pb.DeleteQosVolumeRequest) (*emptypb.Empty, error)
}

func sortVirtioBlks(virtioBlks []*pb.VirtioBlk) {
	sort.Slice(virtioBlks, func(i int, j int) bool {
		return virtioBlks[i].Name < virtioBlks[j].Name
	})
}

// CreateVirtioBlk creates a Virtio block device
func (s *Server) CreateVirtioBlk(ctx context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("CreateVirtioBlk: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := uuid.New().String()
	if in.VirtioBlkId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.VirtioBlkId, in.VirtioBlk.Name)
		resourceID = in.VirtioBlkId
	}
	in.VirtioBlk.Name = resourceID
	// idempotent API when called with same key, should return same object
	controller, ok := s.Virt.BlkCtrls[in.VirtioBlk.Name]
	if ok {
		log.Printf("Already existing NvmeController with id %v", in.VirtioBlk.Name)
		return controller, nil
	}
	// not found, so create a new one
	if s.needToSetVirtioBlkQos(in.VirtioBlk) {
		out, err := s.Virt.qosProvider.CreateQosVolume(ctx, &pb.CreateQosVolumeRequest{
			QosVolumeId: s.qosVolumeIDFromVirtioBlkResourceID(resourceID),
			QosVolume: &pb.QosVolume{
				VolumeId: in.VirtioBlk.VolumeId,
				MaxLimit: in.VirtioBlk.MaxLimit,
				MinLimit: in.VirtioBlk.MinLimit,
			},
		})
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		s.Virt.qosVolumeNames[in.VirtioBlk.Name] = out.Name
	}

	params := spdk.VhostCreateBlkControllerParams{
		Ctrlr:   resourceID,
		DevName: in.VirtioBlk.VolumeId.Value,
	}
	var result spdk.VhostCreateBlkControllerResult
	err := s.rpc.Call("vhost_create_blk_controller", &params, &result)
	if err != nil {
		// TODO: cleanup QoS if needed
		log.Printf("error: %v", err)
		return nil, fmt.Errorf("%w for %v", spdk.ErrFailedSpdkCall, in)
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		// TODO: cleanup QoS if needed
		log.Printf("Could not create: %v", in)
		return nil, fmt.Errorf("%w for %v", spdk.ErrUnexpectedSpdkCallResult, in)
	}
	s.Virt.BlkCtrls[in.VirtioBlk.Name] = in.VirtioBlk
	// s.VirtioCtrls[in.VirtioBlk.Name].Status = &pb.NvmeControllerStatus{Active: true}
	response := server.ProtoClone(in.VirtioBlk)
	return response, nil
}

// DeleteVirtioBlk deletes a Virtio block device
func (s *Server) DeleteVirtioBlk(ctx context.Context, in *pb.DeleteVirtioBlkRequest) (*emptypb.Empty, error) {
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
		return nil, spdk.ErrUnexpectedSpdkCallResult
	}

	if s.needToSetVirtioBlkQos(controller) {
		qosVolumeName, ok := s.Virt.qosVolumeNames[controller.Name]
		if !ok {
			return nil, status.Errorf(codes.Internal, "Underlying QosVolume name is not found")
		}
		if _, err := s.Virt.qosProvider.DeleteQosVolume(ctx,
			&pb.DeleteQosVolumeRequest{
				Name: qosVolumeName,
			}); err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
	}
	delete(s.Virt.BlkCtrls, controller.Name)
	return &emptypb.Empty{}, nil
}

// UpdateVirtioBlk updates a Virtio block device
func (s *Server) UpdateVirtioBlk(_ context.Context, in *pb.UpdateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	log.Printf("UpdateVirtioBlk: Received from client: %v", in)
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
	log.Printf("TODO: use resourceID=%v", resourceID)
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
		Name:     result[0].Ctrlr,
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 1},
		VolumeId: &pc.ObjectKey{Value: "TBD"}}, nil
}

// VirtioBlkStats gets a Virtio block device stats
func (s *Server) VirtioBlkStats(_ context.Context, in *pb.VirtioBlkStatsRequest) (*pb.VirtioBlkStatsResponse, error) {
	log.Printf("VirtioBlkStats: Received from client: %v", in)
	volume, ok := s.Virt.BlkCtrls[in.ControllerId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.ControllerId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send anme to SPDK and get back stats: %v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "VirtioBlkStats method is not implemented")
}

func (s *Server) needToSetVirtioBlkQos(virtioBlk *pb.VirtioBlk) bool {
	return (virtioBlk.MaxLimit != nil && !proto.Equal(virtioBlk.MaxLimit, &pb.QosLimit{})) ||
		(virtioBlk.MinLimit != nil && !proto.Equal(virtioBlk.MinLimit, &pb.QosLimit{}))
}

func (s *Server) qosVolumeIDFromVirtioBlkResourceID(id string) string {
	const virtioBlkRelatedQosVolumePrefix = "__opi-internal-"
	return virtioBlkRelatedQosVolumePrefix + id
}
