// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"context"
	"fmt"
	"log"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateQosVolume creates a QoS volume
func (s *Server) CreateQosVolume(_ context.Context, in *pb.CreateQosVolumeRequest) (*pb.QosVolume, error) {
	log.Printf("CreateQosVolume: Received from client: %v", in)
	if err := s.verifyQosVolume(in.QosVolume); err != nil {
		log.Println("error:", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if volume, ok := s.volumes.qosVolumes[in.QosVolume.QosVolumeId.Value]; ok {
		log.Printf("Already existing QoS volume with id %v", in.QosVolume.QosVolumeId.Value)
		return volume, nil
	}

	if err := s.setMaxLimit(in.QosVolume.VolumeId.Value, in.QosVolume.LimitMax); err != nil {
		return nil, err
	}

	s.volumes.qosVolumes[in.QosVolume.QosVolumeId.Value] = proto.Clone(in.QosVolume).(*pb.QosVolume)
	return in.QosVolume, nil
}

// DeleteQosVolume deletes a QoS volume
func (s *Server) DeleteQosVolume(_ context.Context, in *pb.DeleteQosVolumeRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteQosVolume: Received from client: %v", in)
	qosVolume, ok := s.volumes.qosVolumes[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	if err := s.cleanMaxLimit(qosVolume.VolumeId.Value); err != nil {
		return nil, err
	}

	delete(s.volumes.qosVolumes, in.Name)
	return &emptypb.Empty{}, nil
}

// UpdateQosVolume updates a QoS volume
func (s *Server) UpdateQosVolume(_ context.Context, in *pb.UpdateQosVolumeRequest) (*pb.QosVolume, error) {
	log.Printf("UpdateQosVolume: Received from client: %v", in)
	if err := s.verifyQosVolume(in.QosVolume); err != nil {
		log.Println("error:", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	qosVolumeID := in.QosVolume.QosVolumeId.Value
	volume, ok := s.volumes.qosVolumes[qosVolumeID]
	if !ok {
		log.Printf("Non-existing QoS volume with id %v", qosVolumeID)
		return nil, status.Errorf(codes.NotFound, "volume_id %v does not exist", qosVolumeID)
	}

	if volume.VolumeId.Value != in.QosVolume.VolumeId.Value {
		msg := fmt.Sprintf("Change of underlying volume %v to a new one %v is forbidden",
			volume.VolumeId.Value, in.QosVolume.VolumeId.Value)
		log.Println("error:", msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	log.Println("Set new max limit values")
	if err := s.setMaxLimit(in.QosVolume.VolumeId.Value, in.QosVolume.LimitMax); err != nil {
		return nil, err
	}

	s.volumes.qosVolumes[qosVolumeID] = in.QosVolume
	return in.QosVolume, nil
}

// GetQosVolume gets a QoS volume
func (s *Server) GetQosVolume(_ context.Context, in *pb.GetQosVolumeRequest) (*pb.QosVolume, error) {
	log.Printf("GetQosVolume: Received from client: %v", in)
	volume, ok := s.volumes.qosVolumes[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	return volume, nil
}

// QosVolumeStats gets a QoS volume stats
func (s *Server) QosVolumeStats(_ context.Context, in *pb.QosVolumeStatsRequest) (*pb.QosVolumeStatsResponse, error) {
	log.Printf("QosVolumeStats: Received from client: %v", in)
	if in.VolumeId == nil || in.VolumeId.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "volume_id cannot be empty")
	}
	volume, ok := s.volumes.qosVolumes[in.VolumeId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.VolumeId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.BdevGetIostatParams{
		Name: volume.VolumeId.Value,
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

	return &pb.QosVolumeStatsResponse{
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
		},
		Id: in.VolumeId}, nil
}

func (s *Server) verifyQosVolume(volume *pb.QosVolume) error {
	if volume.QosVolumeId == nil || volume.QosVolumeId.Value == "" {
		return fmt.Errorf("qos_volume_id cannot be empty")
	}
	if volume.VolumeId == nil || volume.VolumeId.Value == "" {
		return fmt.Errorf("volume_id cannot be empty")
	}

	if volume.LimitMin != nil {
		return fmt.Errorf("QoS volume limit_min is not supported")
	}
	if volume.LimitMax.RdIopsKiops != 0 {
		return fmt.Errorf("QoS volume limit_max rd_iops_kiops is not supported")
	}
	if volume.LimitMax.WrIopsKiops != 0 {
		return fmt.Errorf("QoS volume limit_max wr_iops_kiops is not supported")
	}

	if volume.LimitMax.RdBandwidthMbs == 0 &&
		volume.LimitMax.WrBandwidthMbs == 0 &&
		volume.LimitMax.RwBandwidthMbs == 0 &&
		volume.LimitMax.RdIopsKiops == 0 &&
		volume.LimitMax.WrIopsKiops == 0 &&
		volume.LimitMax.RwIopsKiops == 0 {
		return fmt.Errorf("QoS volume limit_max should set limit")
	}

	if volume.LimitMax.RwIopsKiops < 0 {
		return fmt.Errorf("QoS volume limit_max rw_iops_kiops cannot be negative")
	}
	if volume.LimitMax.RdBandwidthMbs < 0 {
		return fmt.Errorf("QoS volume limit_max rd_bandwidth_mbs cannot be negative")
	}
	if volume.LimitMax.WrBandwidthMbs < 0 {
		return fmt.Errorf("QoS volume limit_max wr_bandwidth_mbs cannot be negative")
	}
	if volume.LimitMax.RwBandwidthMbs < 0 {
		return fmt.Errorf("QoS volume limit_max rw_bandwidth_mbs cannot be negative")
	}

	return nil
}

func (s *Server) setMaxLimit(qosVolumeID string, limit *pb.QosLimit) error {
	params := spdk.BdevQoSParams{
		Name:           qosVolumeID,
		RwIosPerSec:    int(limit.RwIopsKiops * 1000),
		RwMbytesPerSec: int(limit.RwBandwidthMbs),
		RMbytesPerSec:  int(limit.RdBandwidthMbs),
		WMbytesPerSec:  int(limit.RdBandwidthMbs),
	}
	var result spdk.BdevQoSResult
	err := s.rpc.Call("bdev_set_qos_limit", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return spdk.ErrFailedSpdkCall
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not set max QoS limit: %s on %v", limit, qosVolumeID)
		log.Print(msg)
		return spdk.ErrUnexpectedSpdkCallResult
	}

	return nil
}

func (s *Server) cleanMaxLimit(qosVolumeID string) error {
	return s.setMaxLimit(qosVolumeID, &pb.QosLimit{})
}
