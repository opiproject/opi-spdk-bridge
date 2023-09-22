// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func (s *Server) validateCreateVirtioBlkRequest(in *pb.CreateVirtioBlkRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.VirtioBlk.VolumeNameRef); err != nil {
		return err
	}
	// see https://google.aip.dev/133#user-specified-ids
	if in.VirtioBlkId != "" {
		if err := resourceid.ValidateUserSettable(in.VirtioBlkId); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) validateDeleteVirtioBlkRequest(in *pb.DeleteVirtioBlkRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateUpdateVirtioBlkRequest(in *pb.UpdateVirtioBlkRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.VirtioBlk.VolumeNameRef); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.VirtioBlk.Name)
}

func (s *Server) validateGetVirtioBlkRequest(in *pb.GetVirtioBlkRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateStatsVirtioBlkRequest(in *pb.StatsVirtioBlkRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}
