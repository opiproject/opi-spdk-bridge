// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implements the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"context"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
				Uuid: &pc.Uuid{Value: bdev.UUID},
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
