// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNvmeControllers(controllers []*pb.NvmeController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Name < controllers[j].Name
	})
}

// CreateNvmeController creates an Nvme controller
func (s *Server) CreateNvmeController(ctx context.Context, in *pb.CreateNvmeControllerRequest) (*pb.NvmeController, error) {
	// check input correctness
	if err := s.validateCreateNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvmeControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeControllerId, in.NvmeController.Name)
		resourceID = in.NvmeControllerId
	}
	in.NvmeController.Name = utils.ResourceIDToControllerName(utils.GetSubsystemIDFromNvmeName(in.Parent), resourceID)
	// idempotent API when called with same key, should return same object
	controller, ok := s.Nvme.Controllers[in.NvmeController.Name]
	if ok {
		log.Printf("Already existing NvmeController with name %v", in.NvmeController.Name)
		return controller, nil
	}
	// not found, so create a new one
	subsys, ok := s.Nvme.Subsystems[in.Parent]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.Parent)
		return nil, err
	}

	transport, ok := s.Nvme.transports[in.NvmeController.Spec.Trtype]
	if !ok {
		return nil, status.Errorf(codes.NotFound,
			"handler for transport type %v is not registered", in.NvmeController.Spec.Trtype)
	}

	err := transport.CreateController(ctx, in.NvmeController, subsys)
	if err != nil {
		return nil, err
	}

	response := utils.ProtoClone(in.NvmeController)
	response.Spec.NvmeControllerId = proto.Int32(-1)
	response.Status = &pb.NvmeControllerStatus{Active: true}
	s.Nvme.Controllers[in.NvmeController.Name] = response

	return response, nil
}

// DeleteNvmeController deletes an Nvme controller
func (s *Server) DeleteNvmeController(ctx context.Context, in *pb.DeleteNvmeControllerRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller, ok := s.Nvme.Controllers[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	subsysName := utils.ResourceIDToSubsystemName(utils.GetSubsystemIDFromNvmeName(in.Name))
	subsys, ok := s.Nvme.Subsystems[subsysName]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", subsysName)
		return nil, err
	}

	transport, ok := s.Nvme.transports[controller.Spec.Trtype]
	if !ok {
		return nil, status.Errorf(codes.NotFound,
			"handler for transport type %v is not registered", controller.Spec.Trtype)
	}

	err := transport.DeleteController(ctx, controller, subsys)
	if err != nil {
		return nil, err
	}

	delete(s.Nvme.Controllers, controller.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNvmeController updates an Nvme controller
func (s *Server) UpdateNvmeController(_ context.Context, in *pb.UpdateNvmeControllerRequest) (*pb.NvmeController, error) {
	// check input correctness
	if err := s.validateUpdateNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	ctrlr, ok := s.Nvme.Controllers[in.NvmeController.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeController.Name)
		return nil, err
	}
	resourceID := path.Base(ctrlr.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmeController); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	response := utils.ProtoClone(in.NvmeController)
	response.Status = &pb.NvmeControllerStatus{Active: true}
	s.Nvme.Controllers[in.NvmeController.Name] = response
	return response, nil
}

// ListNvmeControllers lists Nvme controllers
func (s *Server) ListNvmeControllers(_ context.Context, in *pb.ListNvmeControllersRequest) (*pb.ListNvmeControllersResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	Blobarray := []*pb.NvmeController{}
	for _, controller := range s.Nvme.Controllers {
		Blobarray = append(Blobarray, controller)
	}
	sortNvmeControllers(Blobarray)
	token := uuid.New().String()
	s.Pagination[token] = int(in.PageSize)
	return &pb.ListNvmeControllersResponse{NvmeControllers: Blobarray, NextPageToken: token}, nil
}

// GetNvmeController gets an Nvme controller
func (s *Server) GetNvmeController(_ context.Context, in *pb.GetNvmeControllerRequest) (*pb.NvmeController, error) {
	// check input correctness
	if err := s.validateGetNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller, ok := s.Nvme.Controllers[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	return &pb.NvmeController{Name: in.Name, Spec: &pb.NvmeControllerSpec{NvmeControllerId: controller.Spec.NvmeControllerId}, Status: &pb.NvmeControllerStatus{Active: true}}, nil
}

// StatsNvmeController gets an Nvme controller stats
func (s *Server) StatsNvmeController(_ context.Context, in *pb.StatsNvmeControllerRequest) (*pb.StatsNvmeControllerResponse, error) {
	// check input correctness
	if err := s.validateStatsNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	ctrlr, ok := s.Nvme.Controllers[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(ctrlr.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	return &pb.StatsNvmeControllerResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}
