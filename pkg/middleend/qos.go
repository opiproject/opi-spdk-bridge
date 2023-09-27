// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/google/uuid"
	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortQosVolumes(volumes []*pb.QosVolume) {
	sort.Slice(volumes, func(i int, j int) bool {
		return volumes[i].Name < volumes[j].Name
	})
}

// CreateQosVolume creates a QoS volume
func (s *Server) CreateQosVolume(_ context.Context, in *pb.CreateQosVolumeRequest) (*pb.QosVolume, error) {
	log.Printf("CreateQosVolume: Received from client: %v", in)
	// check input correctness
	if err := s.validateCreateQosVolumeRequest(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.QosVolumeId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.QosVolumeId, in.QosVolume.Name)
		resourceID = in.QosVolumeId
	}
	in.QosVolume.Name = utils.ResourceIDToVolumeName(resourceID)

	if err := s.verifyQosVolume(in.QosVolume); err != nil {
		log.Println("error:", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if volume, ok := s.volumes.qosVolumes[in.QosVolume.Name]; ok {
		log.Printf("Already existing QosVolume with name %v", in.QosVolume.Name)
		return volume, nil
	}

	if err := s.setMaxLimit(in.QosVolume.VolumeNameRef, in.QosVolume.Limits.Max); err != nil {
		return nil, err
	}

	response := utils.ProtoClone(in.QosVolume)
	s.volumes.qosVolumes[in.QosVolume.Name] = response
	log.Printf("CreateQosVolume: Sending to client: %v", response)
	return response, nil
}

// DeleteQosVolume deletes a QoS volume
func (s *Server) DeleteQosVolume(_ context.Context, in *pb.DeleteQosVolumeRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteQosVolume: Received from client: %v", in)
	// check input correctness
	if err := s.validateDeleteQosVolumeRequest(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	qosVolume, ok := s.volumes.qosVolumes[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	if err := s.cleanMaxLimit(qosVolume.VolumeNameRef); err != nil {
		return nil, err
	}

	delete(s.volumes.qosVolumes, in.Name)
	return &emptypb.Empty{}, nil
}

// UpdateQosVolume updates a QoS volume
func (s *Server) UpdateQosVolume(_ context.Context, in *pb.UpdateQosVolumeRequest) (*pb.QosVolume, error) {
	log.Printf("UpdateQosVolume: Received from client: %v", in)
	// check input correctness
	if err := s.validateUpdateQosVolumeRequest(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	if err := s.verifyQosVolume(in.QosVolume); err != nil {
		log.Println("error:", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	name := in.QosVolume.Name
	volume, ok := s.volumes.qosVolumes[name]
	if !ok {
		log.Printf("Non-existing QoS volume with name %v", name)
		return nil, status.Errorf(codes.NotFound, "unable to find key %s", name)
	}

	if volume.VolumeNameRef != in.QosVolume.VolumeNameRef {
		msg := fmt.Sprintf("Change of underlying volume %v to a new one %v is forbidden",
			volume.VolumeNameRef, in.QosVolume.VolumeNameRef)
		log.Println("error:", msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	log.Println("Set new max limit values")
	if err := s.setMaxLimit(in.QosVolume.VolumeNameRef, in.QosVolume.Limits.Max); err != nil {
		return nil, err
	}

	s.volumes.qosVolumes[name] = in.QosVolume
	return in.QosVolume, nil
}

// ListQosVolumes lists QoS volumes
func (s *Server) ListQosVolumes(_ context.Context, in *pb.ListQosVolumesRequest) (*pb.ListQosVolumesResponse, error) {
	log.Printf("ListQosVolume: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	size, offset, err := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	volumes := []*pb.QosVolume{}
	for _, qosVolume := range s.volumes.qosVolumes {
		volumes = append(volumes, utils.ProtoClone(qosVolume))
	}
	sortQosVolumes(volumes)

	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(volumes), offset, size)
	volumes, hasMoreElements := utils.LimitPagination(volumes, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}

	return &pb.ListQosVolumesResponse{QosVolumes: volumes, NextPageToken: token}, nil
}

// GetQosVolume gets a QoS volume
func (s *Server) GetQosVolume(_ context.Context, in *pb.GetQosVolumeRequest) (*pb.QosVolume, error) {
	log.Printf("GetQosVolume: Received from client: %v", in)
	// check input correctness
	if err := s.validateGetQosVolumeRequest(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.volumes.qosVolumes[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	return volume, nil
}

// StatsQosVolume gets a QoS volume stats
func (s *Server) StatsQosVolume(_ context.Context, in *pb.StatsQosVolumeRequest) (*pb.StatsQosVolumeResponse, error) {
	log.Printf("StatsQosVolume: Received from client: %v", in)
	// check input correctness
	if err := s.validateStatsQosVolumeRequest(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.volumes.qosVolumes[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.BdevGetIostatParams{
		Name: volume.VolumeNameRef,
	}
	var result spdk.BdevGetIostatResult
	err := s.rpc.Call("bdev_get_iostat", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, spdk.ErrFailedSpdkCall
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result.Bdevs) != 1 {
		log.Printf("error: expect to find one bdev in response")
		return nil, spdk.ErrUnexpectedSpdkCallResult
	}

	return &pb.StatsQosVolumeResponse{
		Stats: &pb.VolumeStats{
			ReadBytesCount:    int32(result.Bdevs[0].BytesRead),
			ReadOpsCount:      int32(result.Bdevs[0].NumReadOps),
			WriteBytesCount:   int32(result.Bdevs[0].BytesWritten),
			WriteOpsCount:     int32(result.Bdevs[0].NumWriteOps),
			UnmapBytesCount:   int32(result.Bdevs[0].BytesUnmapped),
			UnmapOpsCount:     int32(result.Bdevs[0].NumUnmapOps),
			ReadLatencyTicks:  int32(result.Bdevs[0].ReadLatencyTicks),
			WriteLatencyTicks: int32(result.Bdevs[0].WriteLatencyTicks),
			UnmapLatencyTicks: int32(result.Bdevs[0].UnmapLatencyTicks),
		}}, nil
}

func (s *Server) setMaxLimit(underlyingVolume string, limit *pb.QosLimit) error {
	params := spdk.BdevQoSParams{
		Name:           underlyingVolume,
		RwIosPerSec:    int(limit.RwIopsKiops * 1000),
		RwMbytesPerSec: int(limit.RwBandwidthMbs),
		RMbytesPerSec:  int(limit.RdBandwidthMbs),
		WMbytesPerSec:  int(limit.WrBandwidthMbs),
	}
	var result spdk.BdevQoSResult
	err := s.rpc.Call("bdev_set_qos_limit", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return spdk.ErrFailedSpdkCall
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not set max QoS limit: %s on %v", limit, underlyingVolume)
		log.Print(msg)
		return spdk.ErrUnexpectedSpdkCallResult
	}

	return nil
}

func (s *Server) cleanMaxLimit(underlyingVolume string) error {
	return s.setMaxLimit(underlyingVolume, &pb.QosLimit{})
}
