// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/opiproject/gospdk/spdk"
	_go "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNVMfRemoteControllers(controllers []*pb.NVMfRemoteController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Name < controllers[j].Name
	})
}

// CreateNVMfRemoteController creates an NVMf remote controller
func (s *Server) CreateNVMfRemoteController(_ context.Context, in *pb.CreateNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("CreateNVMfRemoteController: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvMfRemoteControllerId != "" {
		err := resourceid.ValidateUserSettable(in.NvMfRemoteControllerId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMfRemoteControllerId, in.NvMfRemoteController.Name)
		resourceID = in.NvMfRemoteControllerId
	}
	in.NvMfRemoteController.Name = server.ResourceIDToVolumeName(resourceID)
	// idempotent API when called with same key, should return same object
	volume, ok := s.Volumes.NvmeControllers[in.NvMfRemoteController.Name]
	if ok {
		log.Printf("Already existing NVMfRemoteController with id %v", in.NvMfRemoteController.Name)
		return volume, nil
	}
	// not found, so create a new one
	response := server.ProtoClone(in.NvMfRemoteController)
	s.Volumes.NvmeControllers[in.NvMfRemoteController.Name] = response
	log.Printf("CreateNVMfRemoteController: Sending to client: %v", response)
	return response, nil
}

// DeleteNVMfRemoteController deletes an NVMf remote controller
func (s *Server) DeleteNVMfRemoteController(_ context.Context, in *pb.DeleteNVMfRemoteControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMfRemoteController: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v -> %v", err, volume)
		return nil, err
	}
	if s.Volumes.NvmeNumberOfPaths[in.Name] != 0 {
		return nil, status.Error(codes.FailedPrecondition, "NvmfPaths exist for controller")
	}
	delete(s.Volumes.NvmeControllers, volume.Name)
	return &emptypb.Empty{}, nil
}

// NVMfRemoteControllerReset resets an NVMf remote controller
func (s *Server) NVMfRemoteControllerReset(_ context.Context, in *pb.NVMfRemoteControllerResetRequest) (*emptypb.Empty, error) {
	log.Printf("Received: %v", in.GetId())
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Id.Value); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ListNVMfRemoteControllers lists an NVMf remote controllers
func (s *Server) ListNVMfRemoteControllers(_ context.Context, in *pb.ListNVMfRemoteControllersRequest) (*pb.ListNVMfRemoteControllersResponse, error) {
	log.Printf("ListNVMfRemoteControllers: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}

	Blobarray := []*pb.NVMfRemoteController{}
	for _, controller := range s.Volumes.NvmeControllers {
		Blobarray = append(Blobarray, controller)
	}
	sortNVMfRemoteControllers(Blobarray)

	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(Blobarray), offset, size)
	Blobarray, hasMoreElements := server.LimitPagination(Blobarray, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	return &pb.ListNVMfRemoteControllersResponse{NvMfRemoteControllers: Blobarray, NextPageToken: token}, nil
}

// GetNVMfRemoteController gets an NVMf remote controller
func (s *Server) GetNVMfRemoteController(_ context.Context, in *pb.GetNVMfRemoteControllerRequest) (*pb.NVMfRemoteController, error) {
	log.Printf("GetNVMfRemoteController: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	response := server.ProtoClone(volume)
	return response, nil
}

// NVMfRemoteControllerStats gets NVMf remote controller stats
func (s *Server) NVMfRemoteControllerStats(_ context.Context, in *pb.NVMfRemoteControllerStatsRequest) (*pb.NVMfRemoteControllerStatsResponse, error) {
	log.Printf("Received: %v", in.GetId())
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Id.Value); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmeControllers[in.Id.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Id.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	name := path.Base(volume.Name)
	log.Printf("TODO: send anme to SPDK and get back stats: %v", name)
	return &pb.NVMfRemoteControllerStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNVMfPath creates a new NVMf path
func (s *Server) CreateNVMfPath(_ context.Context, in *pb.CreateNVMfPathRequest) (*pb.NVMfPath, error) {
	log.Printf("CreateNVMfPath: Received from client: %v", in)
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	resourceID := resourceid.NewSystemGenerated()
	if in.NvMfPathId != "" {
		err := resourceid.ValidateUserSettable(in.NvMfPathId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMfPathId, in.NvMfPath.Name)
		resourceID = in.NvMfPathId
	}
	in.NvMfPath.Name = server.ResourceIDToVolumeName(resourceID)

	nvmfPath, ok := s.Volumes.NvmePaths[in.NvMfPath.Name]
	if ok {
		log.Printf("Already existing NVMfPath with id %v", in.NvMfPath.Name)
		return nvmfPath, nil
	}

	controller, ok := s.Volumes.NvmeControllers[in.NvMfPath.ControllerId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find NVMfRemoteController by key %s", in.NvMfPath.ControllerId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	multipath := ""
	if numberOfPaths, ok := s.Volumes.NvmeNumberOfPaths[in.NvMfPath.ControllerId.Value]; ok {
		if numberOfPaths < 1 {
			log.Printf("error: Entry with number of paths for %v exists with zero paths", in.NvMfPath.ControllerId.Value)
			return nil, status.Error(codes.Internal, "Internal error")
		}
		// set multipath parameter only when at least one path already exists
		multipath = strings.ToLower(
			strings.ReplaceAll(controller.Multipath.String(), "NVME_MULTIPATH_", ""),
		)
	}
	params := spdk.BdevNvmeAttachControllerParams{
		Name:      path.Base(controller.Name),
		Trtype:    s.opiTransportToSpdk(in.NvMfPath.Trtype),
		Traddr:    in.NvMfPath.Traddr,
		Adrfam:    s.opiAdressFamilyToSpdk(in.NvMfPath.Adrfam),
		Trsvcid:   fmt.Sprint(in.NvMfPath.Trsvcid),
		Subnqn:    in.NvMfPath.Subnqn,
		Hostnqn:   in.NvMfPath.Hostnqn,
		Multipath: multipath,
		Hdgst:     controller.Hdgst,
		Ddgst:     controller.Ddgst,
	}
	var result []spdk.BdevNvmeAttachControllerResult
	err := s.rpc.Call("bdev_nvme_attach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	response := server.ProtoClone(in.NvMfPath)
	s.Volumes.NvmePaths[in.NvMfPath.Name] = response
	s.Volumes.NvmeNumberOfPaths[in.NvMfPath.ControllerId.Value]++
	log.Printf("CreateNVMfPath: Sending to client: %v", response)
	return response, nil
}

// DeleteNVMfPath deletes a NVMf path
func (s *Server) DeleteNVMfPath(_ context.Context, in *pb.DeleteNVMfPathRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMfPath: Received from client: %v", in)
	nvmfPath, ok := s.Volumes.NvmePaths[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	controller, ok := s.Volumes.NvmeControllers[nvmfPath.ControllerId.Value]
	if !ok {
		err := status.Errorf(codes.Internal, "unable to find NVMfRemoteController by key %s", nvmfPath.ControllerId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	resourceID := path.Base(controller.Name)
	params := spdk.BdevNvmeDetachControllerParams{
		Name:    resourceID,
		Trtype:  s.opiTransportToSpdk(nvmfPath.Trtype),
		Traddr:  nvmfPath.Traddr,
		Adrfam:  s.opiAdressFamilyToSpdk(nvmfPath.Adrfam),
		Trsvcid: fmt.Sprint(nvmfPath.Trsvcid),
		Subnqn:  nvmfPath.Subnqn,
	}

	var result spdk.BdevNvmeDetachControllerResult
	err := s.rpc.Call("bdev_nvme_detach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, spdk.ErrFailedSpdkCall
	}
	log.Printf("Received from SPDK: %v", result)

	s.Volumes.NvmeNumberOfPaths[nvmfPath.ControllerId.Value]--
	if s.Volumes.NvmeNumberOfPaths[nvmfPath.ControllerId.Value] == 0 {
		delete(s.Volumes.NvmeNumberOfPaths, nvmfPath.ControllerId.Value)
	}
	delete(s.Volumes.NvmePaths, in.Name)

	return &emptypb.Empty{}, nil
}

// ListNVMfRemoteNamespaces lists NVMfRemoteNamespaces exposed by connected NVMfRemoteController.
func (s *Server) ListNVMfRemoteNamespaces(_ context.Context, in *pb.ListNVMfRemoteNamespacesRequest) (*pb.ListNVMfRemoteNamespacesResponse, error) {
	log.Printf("ListNVMfRemoteNamespaces: Received from client: %v", in)
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	if _, ok := s.Volumes.NvmeControllers[in.Parent]; !ok {
		err := status.Errorf(codes.InvalidArgument, "unable to find key %s", in.Parent)
		log.Printf("error: %v", err)
		return nil, err
	}

	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}

	var result []spdk.BdevGetBdevsResult
	err := s.rpc.Call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	Blobarray := []*pb.NVMfRemoteNamespace{}
	resourceID := path.Base(in.Parent)
	namespacePrefix := resourceID + "n"
	for _, bdev := range result {
		if strings.HasPrefix(bdev.Name, namespacePrefix) {
			Blobarray = append(Blobarray, &pb.NVMfRemoteNamespace{
				Name: server.ResourceIDToVolumeName(bdev.Name),
				Uuid: &_go.Uuid{Value: bdev.UUID},
			})
		}
	}

	sortNVMfRemoteNamespaces(Blobarray)

	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(Blobarray), offset, size)
	Blobarray, hasMoreElements := server.LimitPagination(Blobarray, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}

	return &pb.ListNVMfRemoteNamespacesResponse{
		NvMfRemoteNamespaces: Blobarray,
		NextPageToken:        token,
	}, nil
}

func sortNVMfRemoteNamespaces(namespaces []*pb.NVMfRemoteNamespace) {
	sort.Slice(namespaces, func(i int, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})
}

func (s *Server) opiTransportToSpdk(transport pb.NvmeTransportType) string {
	return strings.ReplaceAll(transport.String(), "NVME_TRANSPORT_", "")
}

func (s *Server) opiAdressFamilyToSpdk(adrfam pb.NvmeAddressFamily) string {
	return strings.ReplaceAll(adrfam.String(), "NVMF_ADRFAM_", "")
}
