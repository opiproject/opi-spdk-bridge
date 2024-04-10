// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (c) 2024 Xsight Labs Inc

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func (s *Server) validateCreateMallocVolumeRequest(in *pb.CreateMallocVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// see https://google.aip.dev/133#user-specified-ids
	if in.MallocVolumeId != "" {
		if err := resourceid.ValidateUserSettable(in.MallocVolumeId); err != nil {
			return err
		}
	}
	// TODO: validate also: block_size, blocks_count, md_size, uuid
	return nil
}

func (s *Server) validateDeleteMallocVolumeRequest(in *pb.DeleteMallocVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateUpdateMallocVolumeRequest(in *pb.UpdateMallocVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.MallocVolume.Name)
}

func (s *Server) validateGetMallocVolumeRequest(in *pb.GetMallocVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateStatsMallocVolumeRequest(in *pb.StatsMallocVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}
