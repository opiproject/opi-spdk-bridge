// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package middleend implememnts the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"fmt"

	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func (s *Server) validateCreateQosVolumeRequest(in *pb.CreateQosVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// see https://google.aip.dev/133#user-specified-ids
	if in.QosVolumeId != "" {
		if err := resourceid.ValidateUserSettable(in.QosVolumeId); err != nil {
			return err
		}
	}
	// TODO: validate also: block_size, blocks_count, uuid, filename
	return nil
}

func (s *Server) validateDeleteQosVolumeRequest(in *pb.DeleteQosVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateUpdateQosVolumeRequest(in *pb.UpdateQosVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	// TODO: return resourcename.Validate(in.QosVolume.Name)
	return nil
}

func (s *Server) validateGetQosVolumeRequest(in *pb.GetQosVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateStatsQosVolumeRequest(in *pb.StatsQosVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) verifyQosVolume(volume *pb.QosVolume) error {
	if volume.Name == "" {
		return fmt.Errorf("QoS volume name cannot be empty")
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(volume.Name); err != nil {
		return err
	}

	if volume.Limits.Min != nil {
		return fmt.Errorf("QoS volume min_limit is not supported")
	}
	if volume.Limits.Max.RdIopsKiops != 0 {
		return fmt.Errorf("QoS volume max_limit rd_iops_kiops is not supported")
	}
	if volume.Limits.Max.WrIopsKiops != 0 {
		return fmt.Errorf("QoS volume max_limit wr_iops_kiops is not supported")
	}

	if volume.Limits.Max.RdBandwidthMbs == 0 &&
		volume.Limits.Max.WrBandwidthMbs == 0 &&
		volume.Limits.Max.RwBandwidthMbs == 0 &&
		volume.Limits.Max.RdIopsKiops == 0 &&
		volume.Limits.Max.WrIopsKiops == 0 &&
		volume.Limits.Max.RwIopsKiops == 0 {
		return fmt.Errorf("QoS volume max_limit should set limit")
	}

	if volume.Limits.Max.RwIopsKiops < 0 {
		return fmt.Errorf("QoS volume max_limit rw_iops_kiops cannot be negative")
	}
	if volume.Limits.Max.RdBandwidthMbs < 0 {
		return fmt.Errorf("QoS volume max_limit rd_bandwidth_mbs cannot be negative")
	}
	if volume.Limits.Max.WrBandwidthMbs < 0 {
		return fmt.Errorf("QoS volume max_limit wr_bandwidth_mbs cannot be negative")
	}
	if volume.Limits.Max.RwBandwidthMbs < 0 {
		return fmt.Errorf("QoS volume max_limit rw_bandwidth_mbs cannot be negative")
	}

	return nil
}
