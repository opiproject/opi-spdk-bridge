// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
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
	testNvmeCtrl     = pb.NvmeRemoteController{
		Hdgst:     false,
		Ddgst:     false,
		Multipath: pb.NvmeMultipath_NVME_MULTIPATH_MULTIPATH,
	}
)

func TestBackEnd_CreateNvmeRemoteController(t *testing.T) {
	tests := map[string]struct {
		id      string
		in      *pb.NvmeRemoteController
		out     *pb.NvmeRemoteController
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
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = &testNvmeCtrl
			}
			if tt.out != nil {
				tt.out.Name = testNvmeCtrlName
			}

			request := &pb.CreateNvmeRemoteControllerRequest{NvmeRemoteController: tt.in, NvmeRemoteControllerId: tt.id}
			response, err := testEnv.client.CreateNvmeRemoteController(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
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

func TestBackEnd_NvmeRemoteControllerReset(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request without SPDK": {
			testNvmeCtrlID,
			&emptypb.Empty{},
			[]string{},
			codes.OK,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			request := &pb.NvmeRemoteControllerResetRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeRemoteControllerReset(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
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

func TestBackEnd_ListNvmeRemoteControllers(t *testing.T) {
	tests := map[string]struct {
		in                  string
		out                 []*pb.NvmeRemoteController
		errCode             codes.Code
		errMsg              string
		size                int32
		token               string
		existingControllers map[string]*pb.NvmeRemoteController
	}{
		"valid request with valid SPDK response": {
			testNvmeCtrlID,
			[]*pb.NvmeRemoteController{
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
			map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination overflow": {
			testNvmeCtrlID,
			[]*pb.NvmeRemoteController{
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
			map[string]*pb.NvmeRemoteController{
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
			map[string]*pb.NvmeRemoteController{
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
			map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination": {
			testNvmeCtrlID,
			[]*pb.NvmeRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme12"),
				},
			},
			codes.OK,
			"",
			1,
			"",
			map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination offset": {
			testNvmeCtrlID,
			[]*pb.NvmeRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme13"),
				},
			},
			codes.OK,
			"",
			1,
			"existing-pagination-token",
			map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1
			testEnv.opiSpdkServer.Volumes.NvmeControllers = tt.existingControllers

			request := &pb.ListNvmeRemoteControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeRemoteControllers(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetNvmeRemoteControllers(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNvmeRemoteControllers())
			}

			// Empty NextPageToken indicates end of results list
			if tt.size != 1 && response.GetNextPageToken() != "" {
				t.Error("Expected end of results, received non-empty next page token", response.GetNextPageToken())
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

func TestBackEnd_GetNvmeRemoteController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeRemoteController
		errCode codes.Code
		errMsg  string
	}{
		"valid request": {
			testNvmeCtrlID,
			&pb.NvmeRemoteController{
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
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlID] = &testNvmeCtrl

			request := &pb.GetNvmeRemoteControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeRemoteController(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
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

func TestBackEnd_NvmeRemoteControllerStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with valid SPDK response": {
			testNvmeCtrlID,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
		},
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlID] = &testNvmeCtrl

			request := &pb.NvmeRemoteControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeRemoteControllerStats(testEnv.ctx, request)

			if !proto.Equal(response.GetStats(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetStats())
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

func TestBackEnd_DeleteNvmeRemoteController(t *testing.T) {
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
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			fname1 := server.ResourceIDToVolumeName(tt.in)
			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = &testNvmeCtrl

			request := &pb.DeleteNvmeRemoteControllerRequest{Name: fname1, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeRemoteController(testEnv.ctx, request)

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
