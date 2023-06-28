// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testSubsystemID   = "subsystem-test"
	testSubsystemName = server.ResourceIDToVolumeName(testSubsystemID)
	testSubsystem     = pb.NvmeSubsystem{
		Spec: &pb.NvmeSubsystemSpec{
			Nqn: "nqn.2022-09.io.spdk:opi3",
		},
	}
)

func TestFrontEnd_CreateNvmeSubsystem(t *testing.T) {
	spec := &pb.NvmeSubsystemSpec{
		Nqn:          "nqn.2022-09.io.spdk:opi3",
		SerialNumber: "OpiSerialNumber",
		ModelNumber:  "OpiModelNumber",
	}
	tests := map[string]struct {
		id      string
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
			false,
		},
		"valid request with invalid SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_create_subsystem: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_create_subsystem: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_create_subsystem: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with error code from SPDK version response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("spdk_get_version: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			&pb.NvmeSubsystem{
				Spec: spec,
				Status: &pb.NvmeSubsystemStatus{
					FirmwareRevision: "SPDK v20.10",
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"jsonrpc":"2.0","id":%d,"result":{"version":"SPDK v20.10","fields":{"major":20,"minor":10,"patch":0,"suffix":""}}}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			&testSubsystem,
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			}
			if tt.out != nil {
				tt.out.Name = testSubsystemName
			}

			request := &pb.CreateNvmeSubsystemRequest{NvmeSubsystem: tt.in, NvmeSubsystemId: tt.id}
			response, err := testEnv.client.CreateNvmeSubsystem(testEnv.ctx, request)
			if response != nil {
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

func TestFrontEnd_DeleteNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-subsystem-id"),
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-subsystem-id")),
			false,
			false,
		},
		"unknown key with missing allowed": {
			server.ResourceIDToVolumeName("unknown-id"),
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
		"malformed name": {
			"-ABC-DEF",
			&emptypb.Empty{},
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem

			request := &pb.DeleteNvmeSubsystemRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeSubsystem(testEnv.ctx, request)
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

func TestFrontEnd_UpdateNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
			false,
		},
		"unimplemented method": {
			nil,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
			},
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNvmeSubsystem"),
			false,
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeSubsystem{
				Name: server.ResourceIDToVolumeName("unknown-id"),
				Spec: &pb.NvmeSubsystemSpec{
					Nqn: "nqn.2022-09.io.spdk:opi3",
				},
			},
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
			false,
		},
		"unknown key with missing allowed": {
			nil,
			&pb.NvmeSubsystem{
				Name: server.ResourceIDToVolumeName("unknown-id"),
				Spec: &pb.NvmeSubsystemSpec{
					Nqn: "nqn.2022-09.io.spdk:opi3",
				},
			},
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
			true,
		},
		"malformed name": {
			nil,
			&pb.NvmeSubsystem{Name: "-ABC-DEF"},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem

			request := &pb.UpdateNvmeSubsystemRequest{NvmeSubsystem: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateNvmeSubsystem(testEnv.ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out.Spec)
				mResponse, _ := proto.Marshal(response.Spec)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
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

func TestFrontEnd_ListNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		out     []*pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"valid request with valid SPDK response": {
			[]*pb.NvmeSubsystem{
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi1",
						SerialNumber: "OpiSerialNumber1",
						ModelNumber:  "OpiModelNumber1",
					},
				},
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi2",
						SerialNumber: "OpiSerialNumber2",
						ModelNumber:  "OpiModelNumber2",
					},
				},
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi3",
						SerialNumber: "OpiSerialNumber3",
						ModelNumber:  "OpiModelNumber3",
					},
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[` +
				`{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"},` +
				`{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},` +
				`{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"}` +
				`]}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
		"pagination negative": {
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			[]*pb.NvmeSubsystem{
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi1",
						SerialNumber: "OpiSerialNumber1",
						ModelNumber:  "OpiModelNumber1",
					},
				},
			},
			// {'jsonrpc': '2.0', 'id': 1, 'result': [{'nqn': 'nqn.2020-12.mlnx.snap', 'serial_number': 'Mellanox_Nvme_SNAP', 'model_number': 'Mellanox Nvme SNAP Controller', 'controllers': [{'name': 'NvmeEmu0pf1', 'cntlid': 0, 'pci_bdf': 'ca:00.3', 'pci_index': 1}]}]}
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"},{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"}]}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination offset": {
			[]*pb.NvmeSubsystem{
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi2",
						SerialNumber: "OpiSerialNumber2",
						ModelNumber:  "OpiModelNumber2",
					},
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"},{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"}]}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeSubsystemsRequest{Parent: "todo", PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeSubsystems(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvmeSubsystems, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvmeSubsystems)
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

func TestFrontEnd_GetNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&pb.NvmeSubsystem{
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          "nqn.2022-09.io.spdk:opi3",
					SerialNumber: "OpiSerialNumber3",
					ModelNumber:  "OpiModelNumber3",
				},
				Status: &pb.NvmeSubsystemStatus{
					FirmwareRevision: "TBD",
				},
			},
			// {'jsonrpc': '2.0', 'id': 1, 'result': [{'nqn': 'nqn.2020-12.mlnx.snap', 'serial_number': 'Mellanox_Nvme_SNAP', 'model_number': 'Mellanox Nvme SNAP Controller', 'controllers': [{'name': 'NvmeEmu0pf1', 'cntlid': 0, 'pci_bdf': 'ca:00.3', 'pci_index': 1}]}]}
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"},{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"}]}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem

			request := &pb.GetNvmeSubsystemRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeSubsystem(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Spec, tt.out.Spec) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
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

func TestFrontEnd_NvmeSubsystemStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid marshal SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json: cannot unmarshal array into Go value of type spdk.NvmfGetSubsystemStatsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"poll_groups":[{"name":"nvmf_tgt_poll_group_0","admin_qpairs":0,"io_qpairs":0,"current_admin_qpairs":0,"current_io_qpairs":0,"pending_bdev_io":0,"transports":[{"trtype":"TCP"},{"trtype":"VFIOUSER"}]}]}}`},
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem

			request := &pb.NvmeSubsystemStatsRequest{SubsystemId: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeSubsystemStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
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
