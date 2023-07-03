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
	testNvmeCtrlID   = "opi-nvme8"
	testNvmeCtrlName = server.ResourceIDToVolumeName(testNvmeCtrlID)
	testNvmeCtrl     = pb.NVMfRemoteController{
		Hdgst:     false,
		Ddgst:     false,
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
			&testNvmeCtrl,
			nil,
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
		},

		"valid request": {
			testNvmeCtrlID,
			&testNvmeCtrl,
			&testNvmeCtrl,
			codes.OK,
			"",
			false,
		},
		"already exists": {
			testNvmeCtrlID,
			&testNvmeCtrl,
			&testNvmeCtrl,
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
				testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = &testNvmeCtrl
			}
			if tt.out != nil {
				tt.out.Name = testNvmeCtrlName
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

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
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
			testNvmeCtrlID,
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

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
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
			testNvmeCtrlID,
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
			testNvmeCtrlID,
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
			testNvmeCtrlID,
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
			testNvmeCtrlID,
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
			testNvmeCtrlID,
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
			testNvmeCtrlID,
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

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
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
			testNvmeCtrlID,
			&pb.NVMfRemoteController{
				Name:      testNvmeCtrlName,
				Multipath: testNvmeCtrl.Multipath,
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
		"malformed name": {
			"-ABC-DEF",
			nil,
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlID] = &testNvmeCtrl

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

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
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
			testNvmeCtrlID,
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
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlID] = &testNvmeCtrl

			request := &pb.NVMfRemoteControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMfRemoteControllerStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
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
			testNvmeCtrlID,
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
		"malformed name": {
			"-ABC-DEF",
			&emptypb.Empty{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()

			fname1 := server.ResourceIDToVolumeName(tt.in)
			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = &testNvmeCtrl

			request := &pb.DeleteNVMfRemoteControllerRequest{Name: fname1, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNVMfRemoteController(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}

			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}
