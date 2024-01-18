// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func (s *Server) validateCreateNvmePathRequest(in *pb.CreateNvmePathRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// see https://google.aip.dev/133#user-specified-ids
	if in.NvmePathId != "" {
		if err := resourceid.ValidateUserSettable(in.NvmePathId); err != nil {
			return err
		}
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Parent); err != nil {
		return err
	}
	// validate Fabrics and Type coordinated
	switch in.NvmePath.Trtype {
	case pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE:
		if in.NvmePath.Fabrics != nil {
			err := status.Errorf(codes.InvalidArgument, "fabrics field is not allowed for pcie transport")
			return err
		}
	case pb.NvmeTransportType_NVME_TRANSPORT_TYPE_TCP:
		fallthrough
	case pb.NvmeTransportType_NVME_TRANSPORT_TYPE_RDMA:
		if in.NvmePath.Fabrics == nil {
			err := status.Errorf(codes.InvalidArgument, "missing required field for fabrics transports: fabrics")
			return err
		}
	default:
		err := status.Errorf(codes.InvalidArgument, "not supported transport type: %v", in.NvmePath.Trtype)
		return err
	}
	return nil
}

func (s *Server) validateDeleteNvmePathRequest(in *pb.DeleteNvmePathRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateUpdateNvmePathRequest(in *pb.UpdateNvmePathRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.NvmePath.Name)
}

func (s *Server) validateGetNvmePathRequest(in *pb.GetNvmePathRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateStatsNvmePathRequest(in *pb.StatsNvmePathRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}
