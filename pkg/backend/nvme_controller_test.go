// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	controllerID   = "opi-nvme8"
	controllerName = server.ResourceIDToVolumeName(controllerID)
	controller     = pb.NVMfRemoteController{
		Multipath: pb.NvmeMultipath_NVME_MULTIPATH_MULTIPATH,
	}
)

func TestBackEnd_CreateNVMfRemoteController(t *testing.T) {
	tests := map[string]struct {
		id      string
		in      *pb.NVMfRemoteController
		out     *pb.NVMfRemoteController
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&controller,
			nil,
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
		},

		"valid request": {
			controllerID,
			&controller,
			&controller,
			codes.OK,
			"",
			false,
		},
		"already exists": {
			controllerID,
			&controller,
			&controller,
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.Volumes.NvmeControllers[controllerName] = &controller
			}
			if tt.out != nil {
				tt.out.Name = controllerName
			}

			request := &pb.CreateNVMfRemoteControllerRequest{NvMfRemoteController: tt.in, NvMfRemoteControllerId: tt.id}
			response, err := testEnv.client.CreateNVMfRemoteController(testEnv.ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NVMfRemoteControllerReset(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request without SPDK": {
			controllerID,
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.NVMfRemoteControllerResetRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMfRemoteControllerReset(testEnv.ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_ListNVMfRemoteControllers(t *testing.T) {
	tests := map[string]struct {
		in                  string
		out                 []*pb.NVMfRemoteController
		errCode             codes.Code
		errMsg              string
		size                int32
		token               string
		existingControllers map[string]*pb.NVMfRemoteController
	}{
		"valid request with valid SPDK response": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme12"),
				},
				{
					Name: server.ResourceIDToVolumeName("OpiNvme13"),
				},
			},
			codes.OK,
			"",
			0,
			"",
			map[string]*pb.NVMfRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination overflow": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme12"),
				},
				{
					Name: server.ResourceIDToVolumeName("OpiNvme13"),
				},
			},
			codes.OK,
			"",
			1000,
			"",
			map[string]*pb.NVMfRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination negative": {
			controllerID,
			nil,
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			-10,
			"",
			map[string]*pb.NVMfRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination error": {
			controllerID,
			nil,
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			0,
			"unknown-pagination-token",
			map[string]*pb.NVMfRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme12"),
				},
			},
			codes.OK,
			"",
			1,
			"",
			map[string]*pb.NVMfRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination offset": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme13"),
				},
			},
			codes.OK,
			"",
			1,
			"existing-pagination-token",
			map[string]*pb.NVMfRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1
			testEnv.opiSpdkServer.Volumes.NvmeControllers = tt.existingControllers

			request := &pb.ListNVMfRemoteControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNVMfRemoteControllers(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvMfRemoteControllers, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvMfRemoteControllers)
				}
				// Empty NextPageToken indicates end of results list
				if tt.size != 1 && response.NextPageToken != "" {
					t.Error("Expected end of results, receieved non-empty next page token", response.NextPageToken)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_GetNVMfRemoteController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NVMfRemoteController
		errCode codes.Code
		errMsg  string
	}{
		"valid request": {
			controllerID,
			&pb.NVMfRemoteController{
				Name:      controllerName,
				Multipath: controller.Multipath,
			},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[controllerID] = &controller

			request := &pb.GetNVMfRemoteControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNVMfRemoteController(testEnv.ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NVMfRemoteControllerStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with valid SPDK response": {
			controllerID,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{""},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[controllerID] = &controller

			request := &pb.NVMfRemoteControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMfRemoteControllerStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_DeleteNVMfRemoteController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request": {
			controllerID,
			&emptypb.Empty{},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()

			fname1 := server.ResourceIDToVolumeName(tt.in)
			testEnv.opiSpdkServer.Volumes.NvmeControllers[controllerName] = &controller

			request := &pb.DeleteNVMfRemoteControllerRequest{Name: fname1, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNVMfRemoteController(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}
