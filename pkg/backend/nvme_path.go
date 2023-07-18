// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implements the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNvmePaths(paths []*pb.NvmePath) {
	sort.Slice(paths, func(i int, j int) bool {
		return paths[i].Subnqn < paths[j].Subnqn
	})
}

// CreateNvmePath creates a new Nvme path
func (s *Server) CreateNvmePath(_ context.Context, in *pb.CreateNvmePathRequest) (*pb.NvmePath, error) {
	log.Printf("CreateNvmePath: Received from client: %v", in)
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}

	resourceID := resourceid.NewSystemGenerated()
	if in.NvmePathId != "" {
		err := resourceid.ValidateUserSettable(in.NvmePathId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmePathId, in.NvmePath.Name)
		resourceID = in.NvmePathId
	}
	in.NvmePath.Name = server.ResourceIDToVolumeName(resourceID)

	nvmePath, ok := s.Volumes.NvmePaths[in.NvmePath.Name]
	if ok {
		log.Printf("Already existing NvmePath with id %v", in.NvmePath.Name)
		return nvmePath, nil
	}

	controller, ok := s.Volumes.NvmeControllers[in.NvmePath.ControllerId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find NvmeRemoteController by key %s", in.NvmePath.ControllerId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	multipath := ""
	if numberOfPaths := s.numberOfPathsForController(controller.Name); numberOfPaths > 0 {
		// set multipath parameter only when at least one path already exists
		multipath = s.opiMultipathToSpdk(controller.Multipath)
	}
	psk := ""
	if len(controller.Psk) > 0 {
		log.Printf("Notice, TLS is used to establish connection: to %v", in.NvmePath)
		keyFile, err := s.keyToTemporaryFile(controller.Psk)
		if err != nil {
			return nil, err
		}
		defer func() {
			err := os.Remove(keyFile)
			log.Printf("Cleanup key file %v: %v", keyFile, err)
		}()

		psk = keyFile
	}
	params := spdk.BdevNvmeAttachControllerParams{
		Name:      path.Base(controller.Name),
		Trtype:    s.opiTransportToSpdk(in.NvmePath.Trtype),
		Traddr:    in.NvmePath.Traddr,
		Adrfam:    s.opiAdressFamilyToSpdk(in.NvmePath.Adrfam),
		Trsvcid:   fmt.Sprint(in.NvmePath.Trsvcid),
		Subnqn:    in.NvmePath.Subnqn,
		Hostnqn:   in.NvmePath.Hostnqn,
		Multipath: multipath,
		Hdgst:     controller.Hdgst,
		Ddgst:     controller.Ddgst,
		Psk:       psk,
	}
	var result []spdk.BdevNvmeAttachControllerResult
	err := s.rpc.Call("bdev_nvme_attach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	response := server.ProtoClone(in.NvmePath)
	s.Volumes.NvmePaths[in.NvmePath.Name] = response
	log.Printf("CreateNvmePath: Sending to client: %v", response)
	return response, nil
}

// DeleteNvmePath deletes a Nvme path
func (s *Server) DeleteNvmePath(_ context.Context, in *pb.DeleteNvmePathRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNvmePath: Received from client: %v", in)

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

	nvmePath, ok := s.Volumes.NvmePaths[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	controller, ok := s.Volumes.NvmeControllers[nvmePath.ControllerId.Value]
	if !ok {
		err := status.Errorf(codes.Internal, "unable to find NvmeRemoteController by key %s", nvmePath.ControllerId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.BdevNvmeDetachControllerParams{
		Name:    path.Base(controller.Name),
		Trtype:  s.opiTransportToSpdk(nvmePath.Trtype),
		Traddr:  nvmePath.Traddr,
		Adrfam:  s.opiAdressFamilyToSpdk(nvmePath.Adrfam),
		Trsvcid: fmt.Sprint(nvmePath.Trsvcid),
		Subnqn:  nvmePath.Subnqn,
	}

	var result spdk.BdevNvmeDetachControllerResult
	err := s.rpc.Call("bdev_nvme_detach_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete Nvme Path: %s", path.Base(in.Name))
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	delete(s.Volumes.NvmePaths, in.Name)

	return &emptypb.Empty{}, nil
}

// UpdateNvmePath updates an Nvme path
func (s *Server) UpdateNvmePath(_ context.Context, in *pb.UpdateNvmePathRequest) (*pb.NvmePath, error) {
	log.Printf("UpdateNvmePath: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.NvmePath.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Volumes.NvmePaths[in.NvmePath.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmePath.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmePath); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	response := server.ProtoClone(in.NvmePath)
	// s.Volumes.NvmePaths[in.NvmePath.Name] = response
	return response, nil
}

// ListNvmePaths lists Nvme path
func (s *Server) ListNvmePaths(_ context.Context, in *pb.ListNvmePathsRequest) (*pb.ListNvmePathsResponse, error) {
	log.Printf("ListNvmePaths: Received from client: %v", in)
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
	var result []spdk.BdevNvmeGetControllerResult
	err := s.rpc.Call("bdev_nvme_get_controllers", nil, &result)
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
	Blobarray := make([]*pb.NvmePath, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NvmePath{Name: r.Name /* TODO: fill this */}
	}
	sortNvmePaths(Blobarray)
	return &pb.ListNvmePathsResponse{NvmePaths: Blobarray, NextPageToken: token}, nil
}

// GetNvmePath gets Nvme path
func (s *Server) GetNvmePath(_ context.Context, in *pb.GetNvmePathRequest) (*pb.NvmePath, error) {
	log.Printf("GetNvmePath: Received from client: %v", in)
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
	path, ok := s.Volumes.NvmePaths[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	var result []spdk.BdevNvmeGetControllerResult
	err := s.rpc.Call("bdev_nvme_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	for i := range result {
		r := &result[i]
		if r.Name != "" {
			return &pb.NvmePath{ /* TODO: fill this */ }, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", path.Subnqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// NvmePathStats gets Nvme path stats
func (s *Server) NvmePathStats(_ context.Context, in *pb.NvmePathStatsRequest) (*pb.NvmePathStatsResponse, error) {
	log.Printf("NvmePathStats: Received from client: %v", in)
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
	volume, ok := s.Volumes.NvmePaths[in.Id.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Id.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	var result spdk.NvmfGetSubsystemStatsResult
	err := s.rpc.Call("nvmf_get_stats", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NvmePathStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

func (s *Server) opiTransportToSpdk(transport pb.NvmeTransportType) string {
	return strings.ReplaceAll(transport.String(), "NVME_TRANSPORT_", "")
}

func (s *Server) opiAdressFamilyToSpdk(adrfam pb.NvmeAddressFamily) string {
	return strings.ReplaceAll(adrfam.String(), "NVME_ADRFAM_", "")
}

func (s *Server) opiMultipathToSpdk(multipath pb.NvmeMultipath) string {
	return strings.ToLower(
		strings.ReplaceAll(multipath.String(), "NVME_MULTIPATH_", ""),
	)
}

func (s *Server) numberOfPathsForController(controllerName string) int {
	numberOfPaths := 0
	for _, path := range s.Volumes.NvmePaths {
		if path.ControllerId.Value == controllerName {
			numberOfPaths++
		}
	}
	return numberOfPaths
}

func (s *Server) keyToTemporaryFile(pskKey []byte) (string, error) {
	keyFile, err := s.psk.createTempFile("", "opikey")
	if err != nil {
		log.Printf("error: failed to create file for key: %v", err)
		return "", status.Error(codes.Internal, "failed to handle key")
	}

	const keyPermissions = 0600
	if err := s.psk.writeKey(keyFile.Name(), pskKey, keyPermissions); err != nil {
		log.Printf("error: failed to write to key file: %v", err)
		removeErr := os.Remove(keyFile.Name())
		log.Printf("Delete key file after key write: %v", removeErr)
		return "", status.Error(codes.Internal, "failed to handle key")
	}

	return keyFile.Name(), nil
}
