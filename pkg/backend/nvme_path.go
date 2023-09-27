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
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNvmePaths(paths []*pb.NvmePath) {
	sort.Slice(paths, func(i int, j int) bool {
		return paths[i].Name < paths[j].Name
	})
}

// CreateNvmePath creates a new Nvme path
func (s *Server) CreateNvmePath(_ context.Context, in *pb.CreateNvmePathRequest) (*pb.NvmePath, error) {
	// check input correctness
	if err := s.validateCreateNvmePathRequest(in); err != nil {
		return nil, err
	}

	resourceID := resourceid.NewSystemGenerated()
	if in.NvmePathId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmePathId, in.NvmePath.Name)
		resourceID = in.NvmePathId
	}
	in.NvmePath.Name = utils.ResourceIDToVolumeName(resourceID)

	nvmePath := new(pb.NvmePath)
	found, err := s.store.Get(in.NvmePath.Name, nvmePath)
	if err != nil {
		return nil, err
	}
	if found {
		log.Printf("Already existing NvmePath with id %v", in.NvmePath.Name)
		return nvmePath, nil
	}

	controller := new(pb.NvmeRemoteController)
	found, err = s.store.Get(in.NvmePath.ControllerNameRef, controller)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find NvmeRemoteController by key %s", in.NvmePath.ControllerNameRef)
		return nil, err
	}

	// TODO: consider moving to _validate.go
	if in.NvmePath.Trtype == pb.NvmeTransportType_NVME_TRANSPORT_PCIE && controller.Tcp != nil {
		err := status.Errorf(codes.FailedPrecondition, "pcie transport on tcp controller is not allowed")
		return nil, err
	}

	multipath := ""
	if numberOfPaths := s.numberOfPathsForController(controller.Name); numberOfPaths > 0 {
		// set multipath parameter only when at least one path already exists
		multipath = s.opiMultipathToSpdk(controller.Multipath)
	}
	psk := ""
	if len(controller.GetTcp().GetPsk()) > 0 {
		log.Printf("Notice, TLS is used to establish connection: to %v", in.NvmePath)
		keyFile, err := s.keyToTemporaryFile(controller.Tcp.Psk)
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
		Trtype:    s.opiTransportToSpdk(in.NvmePath.GetTrtype()),
		Traddr:    in.NvmePath.GetTraddr(),
		Adrfam:    s.opiAdressFamilyToSpdk(in.NvmePath.GetFabrics().GetAdrfam()),
		Trsvcid:   fmt.Sprint(in.NvmePath.GetFabrics().GetTrsvcid()),
		Subnqn:    in.NvmePath.GetFabrics().GetSubnqn(),
		Hostnqn:   in.NvmePath.GetFabrics().GetHostnqn(),
		Multipath: multipath,
		Hdgst:     controller.GetTcp().GetHdgst(),
		Ddgst:     controller.GetTcp().GetDdgst(),
		Psk:       psk,
	}
	var result []spdk.BdevNvmeAttachControllerResult
	err = s.rpc.Call("bdev_nvme_attach_controller", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	response := utils.ProtoClone(in.NvmePath)
	err = s.store.Set(in.NvmePath.Name, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// DeleteNvmePath deletes a Nvme path
func (s *Server) DeleteNvmePath(_ context.Context, in *pb.DeleteNvmePathRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteNvmePathRequest(in); err != nil {
		return nil, err
	}
	nvmePath := new(pb.NvmePath)
	found, err := s.store.Get(in.Name, nvmePath)
	if err != nil {
		return nil, err
	}
	if !found {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	controller := new(pb.NvmeRemoteController)
	found, err = s.store.Get(nvmePath.ControllerNameRef, controller)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.Internal, "unable to find NvmeRemoteController by key %s", nvmePath.ControllerNameRef)
		return nil, err
	}

	params := spdk.BdevNvmeDetachControllerParams{
		Name:    path.Base(controller.Name),
		Trtype:  s.opiTransportToSpdk(nvmePath.GetTrtype()),
		Traddr:  nvmePath.GetTraddr(),
		Adrfam:  s.opiAdressFamilyToSpdk(nvmePath.GetFabrics().GetAdrfam()),
		Trsvcid: fmt.Sprint(nvmePath.GetFabrics().GetTrsvcid()),
		Subnqn:  nvmePath.GetFabrics().GetSubnqn(),
	}

	var result spdk.BdevNvmeDetachControllerResult
	err = s.rpc.Call("bdev_nvme_detach_controller", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete Nvme Path: %s", path.Base(in.Name))
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	err = s.store.Delete(nvmePath.Name)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// UpdateNvmePath updates an Nvme path
func (s *Server) UpdateNvmePath(_ context.Context, in *pb.UpdateNvmePathRequest) (*pb.NvmePath, error) {
	// check input correctness
	if err := s.validateUpdateNvmePathRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume := new(pb.NvmePath)
	found, err := s.store.Get(in.NvmePath.Name, volume)
	if err != nil {
		return nil, err
	}
	if !found {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmePath.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmePath); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	response := utils.ProtoClone(in.NvmePath)
	// err = s.store.Set(in.NvmePath.Name, response)
	// if err != nil {
	// 	return nil, err
	// }
	return response, nil
}

// ListNvmePaths lists Nvme path
func (s *Server) ListNvmePaths(_ context.Context, in *pb.ListNvmePathsRequest) (*pb.ListNvmePathsResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}
	var result []spdk.BdevNvmeGetControllerResult
	err := s.rpc.Call("bdev_nvme_get_controllers", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
	result, hasMoreElements := utils.LimitPagination(result, offset, size)
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
	// check input correctness
	if err := s.validateGetNvmePathRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	path := new(pb.NvmePath)
	found, err := s.store.Get(in.Name, path)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}

	var result []spdk.BdevNvmeGetControllerResult
	err = s.rpc.Call("bdev_nvme_get_controllers", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	for i := range result {
		r := &result[i]
		if r.Name != "" {
			return &pb.NvmePath{ /* TODO: fill this */ }, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", path.Fabrics.Subnqn)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// StatsNvmePath gets Nvme path stats
func (s *Server) StatsNvmePath(_ context.Context, in *pb.StatsNvmePathRequest) (*pb.StatsNvmePathResponse, error) {
	// check input correctness
	if err := s.validateStatsNvmePathRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume := new(pb.NvmePath)
	found, err := s.store.Get(in.Name, volume)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	var result spdk.NvmfGetSubsystemStatsResult
	err = s.rpc.Call("nvmf_get_stats", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.StatsNvmePathResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

func (s *Server) opiTransportToSpdk(transport pb.NvmeTransportType) string {
	return strings.ReplaceAll(transport.String(), "NVME_TRANSPORT_", "")
}

func (s *Server) opiAdressFamilyToSpdk(adrfam pb.NvmeAddressFamily) string {
	if adrfam == pb.NvmeAddressFamily_NVME_ADDRESS_FAMILY_UNSPECIFIED {
		return ""
	}

	return strings.ReplaceAll(adrfam.String(), "NVME_ADRFAM_", "")
}

func (s *Server) opiMultipathToSpdk(multipath pb.NvmeMultipath) string {
	return strings.ToLower(
		strings.ReplaceAll(multipath.String(), "NVME_MULTIPATH_", ""),
	)
}

func (s *Server) numberOfPathsForController(controllerName string) int {
	numberOfPaths := 0
	// for _, path := range s.Volumes.NvmePaths {
	// 	if path.ControllerNameRef == controllerName {
	// 		numberOfPaths++
	// 	}
	// }
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
