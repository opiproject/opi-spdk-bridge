// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"fmt"
	"regexp"

	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func (s *Server) validateCreateNvmeSubsystemRequest(in *pb.CreateNvmeSubsystemRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// see https://google.aip.dev/133#user-specified-ids
	if in.NvmeSubsystemId != "" {
		if err := resourceid.ValidateUserSettable(in.NvmeSubsystemId); err != nil {
			return err
		}
	}
	// check Nqn length
	if len(in.NvmeSubsystem.Spec.Nqn) > 223 {
		msg := fmt.Sprintf("Nqn value (%s) is too long, have to be between 1 and 223", in.NvmeSubsystem.Spec.Nqn)
		return status.Errorf(codes.InvalidArgument, msg)
	}
	// check SerialNumber length
	if len(in.NvmeSubsystem.Spec.SerialNumber) > 20 {
		msg := fmt.Sprintf("SerialNumber value (%s) is too long, have to be between 1 and 20", in.NvmeSubsystem.Spec.SerialNumber)
		return status.Errorf(codes.InvalidArgument, msg)
	}
	// check ModelNumber length
	if len(in.NvmeSubsystem.Spec.ModelNumber) > 40 {
		msg := fmt.Sprintf("ModelNumber value (%s) is too long, have to be between 1 and 40", in.NvmeSubsystem.Spec.ModelNumber)
		return status.Errorf(codes.InvalidArgument, msg)
	}
	// check if the NQN matches the pattern
	regex := regexp.MustCompile(`^nqn\.[0-9]{4}-[0-9]{2}(\.[a-zA-Z0-9]+)+(:[a-zA-Z0-9-.]+)+$`)
	if !regex.MatchString(in.NvmeSubsystem.Spec.Nqn) {
		msg := fmt.Sprintf("NQN value (%s) does not match pattern", in.NvmeSubsystem.Spec.Nqn)
		return status.Errorf(codes.InvalidArgument, msg)
	}
	return nil
}

func (s *Server) validateDeleteNvmeSubsystemRequest(in *pb.DeleteNvmeSubsystemRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateUpdateNvmeSubsystemRequest(in *pb.UpdateNvmeSubsystemRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.NvmeSubsystem.Name)
}

func (s *Server) validateGetNvmeSubsystemRequest(in *pb.GetNvmeSubsystemRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateStatsNvmeSubsystemRequest(in *pb.StatsNvmeSubsystemRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}
