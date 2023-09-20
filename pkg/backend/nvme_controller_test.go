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

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testNvmeCtrlID   = "opi-nvme8"
	testNvmeCtrlName = server.ResourceIDToVolumeName(testNvmeCtrlID)
	testNvmeCtrl     = pb.NvmeRemoteController{
		Tcp: &pb.TcpController{
			Hdgst: false,
			Ddgst: false,
		},
		Multipath: pb.NvmeMultipath_NVME_MULTIPATH_MULTIPATH,
	}

	testNvmeCtrlWithName = pb.NvmeRemoteController{
		Name:      testNvmeCtrlName,
		Tcp:       testNvmeCtrl.Tcp,
		Multipath: testNvmeCtrl.Multipath,
	}
)

func TestBackEnd_CreateNvmeRemoteController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		id      string
		in      *pb.NvmeRemoteController
		out     *pb.NvmeRemoteController
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			id:      "CapitalLettersNotAllowed",
			in:      &testNvmeCtrl,
			out:     nil,
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			exist:   false,
		},

		"valid request": {
			id:      testNvmeCtrlID,
			in:      &testNvmeCtrl,
			out:     &testNvmeCtrl,
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
		},
		"already exists": {
			id:      testNvmeCtrlID,
			in:      &testNvmeCtrl,
			out:     &testNvmeCtrl,
			errCode: codes.OK,
			errMsg:  "",
			exist:   true,
		},
		"no required field": {
			id:      testAioVolumeID,
			in:      nil,
			out:     nil,
			errCode: codes.Unknown,
			errMsg:  "missing required field: nvme_remote_controller",
			exist:   false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = server.ProtoClone(&testNvmeCtrl)
				testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName].Name = testNvmeCtrlName
			}
			if tt.out != nil {
				tt.out = server.ProtoClone(tt.out)
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

func TestBackEnd_ResetNvmeRemoteController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request without SPDK": {
			in:      testNvmeCtrlID,
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			request := &pb.ResetNvmeRemoteControllerRequest{Name: tt.in}
			response, err := testEnv.client.ResetNvmeRemoteController(testEnv.ctx, request)

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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
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
			in: testNvmeCtrlID,
			out: []*pb.NvmeRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme12"),
				},
				{
					Name: server.ResourceIDToVolumeName("OpiNvme13"),
				},
			},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
			existingControllers: map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination overflow": {
			in: testNvmeCtrlID,
			out: []*pb.NvmeRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme12"),
				},
				{
					Name: server.ResourceIDToVolumeName("OpiNvme13"),
				},
			},
			errCode: codes.OK,
			errMsg:  "",
			size:    1000,
			token:   "",
			existingControllers: map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination negative": {
			in:      testNvmeCtrlID,
			out:     nil,
			errCode: codes.InvalidArgument,
			errMsg:  "negative PageSize is not allowed",
			size:    -10,
			token:   "",
			existingControllers: map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination error": {
			in:      testNvmeCtrlID,
			out:     nil,
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			size:    0,
			token:   "unknown-pagination-token",
			existingControllers: map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination": {
			in: testNvmeCtrlID,
			out: []*pb.NvmeRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme12"),
				},
			},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
			existingControllers: map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"pagination offset": {
			in: testNvmeCtrlID,
			out: []*pb.NvmeRemoteController{
				{
					Name: server.ResourceIDToVolumeName("OpiNvme13"),
				},
			},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "existing-pagination-token",
			existingControllers: map[string]*pb.NvmeRemoteController{
				server.ResourceIDToVolumeName("OpiNvme12"): {Name: server.ResourceIDToVolumeName("OpiNvme12")},
				server.ResourceIDToVolumeName("OpiNvme13"): {Name: server.ResourceIDToVolumeName("OpiNvme13")},
			},
		},
		"no required field": {
			in:                  "",
			out:                 []*pb.NvmeRemoteController{},
			errCode:             codes.Unknown,
			errMsg:              "missing required field: parent",
			size:                0,
			token:               "",
			existingControllers: map[string]*pb.NvmeRemoteController{},
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1
			for k, v := range tt.existingControllers {
				testEnv.opiSpdkServer.Volumes.NvmeControllers[k] = server.ProtoClone(v)
			}

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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.NvmeRemoteController
		errCode codes.Code
		errMsg  string
	}{
		"valid request": {
			in: testNvmeCtrlID,
			out: &pb.NvmeRemoteController{
				Name:      testNvmeCtrlName,
				Multipath: testNvmeCtrl.Multipath,
				Tcp:       testNvmeCtrl.Tcp,
			},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      "unknown-id",
			out:     nil,
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-id"),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"no required field": {
			in:      "",
			out:     nil,
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlID] = server.ProtoClone(&testNvmeCtrl)
			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlID].Name = testNvmeCtrlName

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

func TestBackEnd_StatsNvmeRemoteController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with valid SPDK response": {
			in: testNvmeCtrlID,
			out: &pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      "unknown-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-id"),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlID] = server.ProtoClone(&testNvmeCtrl)

			request := &pb.StatsNvmeRemoteControllerRequest{Name: tt.in}
			response, err := testEnv.client.StatsNvmeRemoteController(testEnv.ctx, request)

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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request": {
			in:      testNvmeCtrlName,
			out:     &emptypb.Empty{},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			in:      server.ResourceIDToVolumeName("unknown-id"),
			out:     nil,
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			missing: false,
		},
		"unknown key with missing allowed": {
			in:      server.ResourceIDToVolumeName("unknown-id"),
			out:     &emptypb.Empty{},
			errCode: codes.OK,
			errMsg:  "",
			missing: true,
		},
		"malformed name": {
			in:      server.ResourceIDToVolumeName("-ABC-DEF"),
			out:     &emptypb.Empty{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			missing: false,
		},
		"no required field": {
			in:      "",
			out:     &emptypb.Empty{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
			missing: false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment([]string{})
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = server.ProtoClone(&testNvmeCtrl)
			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName].Name = testNvmeCtrlName

			request := &pb.DeleteNvmeRemoteControllerRequest{Name: tt.in, AllowMissing: tt.missing}
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
