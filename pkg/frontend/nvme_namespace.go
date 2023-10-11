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

func sortNvmeNamespaces(namespaces []*pb.NvmeNamespace) {
	sort.Slice(namespaces, func(i int, j int) bool {
		return namespaces[i].Spec.HostNsid < namespaces[j].Spec.HostNsid
	})
}

// CreateNvmeNamespace creates an Nvme namespace
func (s *Server) CreateNvmeNamespace(ctx context.Context, in *pb.CreateNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	// check input correctness
	if err := s.validateCreateNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvmeNamespaceId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeNamespaceId, in.NvmeNamespace.Name)
		resourceID = in.NvmeNamespaceId
	}
	in.NvmeNamespace.Name = utils.ResourceIDToNamespaceName(utils.GetSubsystemIDFromNvmeName(in.Parent), resourceID)
	// idempotent API when called with same key, should return same object
	// fetch object from the database
	namespace, ok := s.Nvme.Namespaces[in.NvmeNamespace.Name]
	if ok {
		log.Printf("Already existing NvmeNamespace with id %v", in.NvmeNamespace.Name)
		return namespace, nil
	}
	// not found, so create a new one
	subsys, ok := s.Nvme.Subsystems[in.Parent]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.Parent)
		return nil, err
	}

	params := spdk.NvmfSubsystemAddNsParams{
		Nqn: subsys.Spec.Nqn,
	}

	// TODO: using bdev for volume id as a middle end handle for now
	params.Namespace.Nsid = int(in.NvmeNamespace.Spec.HostNsid)
	params.Namespace.BdevName = in.NvmeNamespace.Spec.VolumeNameRef

	var result spdk.NvmfSubsystemAddNsResult
	err := s.rpc.Call(ctx, "nvmf_subsystem_add_ns", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result < 0 {
		msg := fmt.Sprintf("Could not create NS: %s", in.NvmeNamespace.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	response := utils.ProtoClone(in.NvmeNamespace)
	response.Status = &pb.NvmeNamespaceStatus{
		State:     pb.NvmeNamespaceStatus_STATE_ENABLED,
		OperState: pb.NvmeNamespaceStatus_OPER_STATE_ONLINE,
	}
	response.Spec.HostNsid = int32(result)
	s.Nvme.Namespaces[in.NvmeNamespace.Name] = response
	return response, nil
}

// DeleteNvmeNamespace deletes an Nvme namespace
func (s *Server) DeleteNvmeNamespace(ctx context.Context, in *pb.DeleteNvmeNamespaceRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	namespace, ok := s.Nvme.Namespaces[in.Name]
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

	params := spdk.NvmfSubsystemRemoveNsParams{
		Nqn:  subsys.Spec.Nqn,
		Nsid: int(namespace.Spec.HostNsid),
	}
	var result spdk.NvmfSubsystemRemoveNsResult
	err := s.rpc.Call(ctx, "nvmf_subsystem_remove_ns", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete NS: %s", in.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Nvme.Namespaces, namespace.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNvmeNamespace updates an Nvme namespace
func (s *Server) UpdateNvmeNamespace(_ context.Context, in *pb.UpdateNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	// check input correctness
	if err := s.validateUpdateNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	namespace, ok := s.Nvme.Namespaces[in.NvmeNamespace.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeNamespace.Name)
		return nil, err
	}
	resourceID := path.Base(namespace.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmeNamespace); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	response := utils.ProtoClone(in.NvmeNamespace)
	response.Status = &pb.NvmeNamespaceStatus{
		State:     pb.NvmeNamespaceStatus_STATE_ENABLED,
		OperState: pb.NvmeNamespaceStatus_OPER_STATE_ONLINE,
	}
	s.Nvme.Namespaces[in.NvmeNamespace.Name] = response

	return response, nil
}

// ListNvmeNamespaces lists Nvme namespaces
func (s *Server) ListNvmeNamespaces(ctx context.Context, in *pb.ListNvmeNamespacesRequest) (*pb.ListNvmeNamespacesResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}

	subsys, ok := s.Nvme.Subsystems[in.Parent]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.Parent)
		return nil, err
	}
	nqn := subsys.Spec.Nqn

	var result []spdk.NvmfGetSubsystemsResult
	err := s.rpc.Call(ctx, "nvmf_get_subsystems", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	Blobarray := []*pb.NvmeNamespace{}
	for i := range result {
		rr := &result[i]
		if rr.Nqn == nqn || nqn == "" {
			log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
			hasMoreElements := false
			rr.Namespaces, hasMoreElements = utils.LimitPagination(rr.Namespaces, offset, size)
			if hasMoreElements {
				token = uuid.New().String()
				s.Pagination[token] = offset + size
			}
			for j := range rr.Namespaces {
				r := &rr.Namespaces[j]
				Blobarray = append(Blobarray, &pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: int32(r.Nsid)}})
			}
		}
	}
	if len(Blobarray) > 0 {
		sortNvmeNamespaces(Blobarray)
		return &pb.ListNvmeNamespacesResponse{NvmeNamespaces: Blobarray, NextPageToken: token}, nil
	}

	msg := fmt.Sprintf("Could not find any namespaces for NQN: %s", nqn)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// GetNvmeNamespace gets an Nvme namespace
func (s *Server) GetNvmeNamespace(ctx context.Context, in *pb.GetNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	// check input correctness
	if err := s.validateGetNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	namespace, ok := s.Nvme.Namespaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	// TODO: do we even query SPDK to confirm if namespace is present?
	// return namespace, nil

	// fetch subsystems -> namespaces from Server, match the nsid to find the corresponding namespace
	subsysName := utils.ResourceIDToSubsystemName(utils.GetSubsystemIDFromNvmeName(in.Name))
	subsys, ok := s.Nvme.Subsystems[subsysName]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", subsysName)
		return nil, err
	}

	var result []spdk.NvmfGetSubsystemsResult
	err := s.rpc.Call(ctx, "nvmf_get_subsystems", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	for i := range result {
		rr := &result[i]
		if rr.Nqn == subsys.Spec.Nqn {
			for j := range rr.Namespaces {
				r := &rr.Namespaces[j]
				if int32(r.Nsid) == namespace.Spec.HostNsid {
					return &pb.NvmeNamespace{
						Name: namespace.Name,
						Spec: &pb.NvmeNamespaceSpec{HostNsid: namespace.Spec.HostNsid},
						Status: &pb.NvmeNamespaceStatus{
							State:     pb.NvmeNamespaceStatus_STATE_ENABLED,
							OperState: pb.NvmeNamespaceStatus_OPER_STATE_ONLINE,
						},
					}, nil
				}
			}
			msg := fmt.Sprintf("Could not find NSID: %d", namespace.Spec.HostNsid)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Spec.Nqn)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// StatsNvmeNamespace gets an Nvme namespace stats
func (s *Server) StatsNvmeNamespace(_ context.Context, in *pb.StatsNvmeNamespaceRequest) (*pb.StatsNvmeNamespaceResponse, error) {
	// check input correctness
	if err := s.validateStatsNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	namespace, ok := s.Nvme.Namespaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	resourceID := path.Base(namespace.Name)
	log.Printf("TODO: send name to SPDK and get back stats: %v", resourceID)
	return &pb.StatsNvmeNamespaceResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}
