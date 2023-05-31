// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"fmt"
	"net"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

var (
	testSubsystemID = "subsystem-test"
	testSubsystem   = pb.NvmeSubsystem{
		Spec: &pb.NvmeSubsystemSpec{
			Nqn: "nqn.2022-09.io.spdk:opi3",
		},
	}
	testControllerID = "controller-test"
	testController   = pb.NvmeController{
		Spec: &pb.NvmeControllerSpec{
			SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
			PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
			NvmeControllerId: 17,
		},
		Status: &pb.NvmeControllerStatus{
			Active: true,
		},
	}
	testNamespaceID = "namespace-test"
	testNamespace   = pb.NvmeNamespace{
		Spec: &pb.NvmeNamespaceSpec{
			HostNsid:    22,
			SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
		},
		Status: &pb.NvmeNamespaceStatus{
			PciState:     2,
			PciOperState: 1,
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
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"valid request with invalid SPDK response": {
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

			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceID] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			}
			if tt.out != nil {
				tt.out.Name = testSubsystemID
			}

			request := &pb.CreateNvmeSubsystemRequest{NvmeSubsystem: tt.in, NvmeSubsystemId: testSubsystemID}
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_UpdateNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"unimplemented method": {
			&pb.NvmeSubsystem{},
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNvmeSubsystem"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.UpdateNvmeSubsystemRequest{NvmeSubsystem: tt.in}
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
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

			request := &pb.ListNvmeSubsystemsRequest{PageSize: tt.size, PageToken: tt.token}
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
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
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
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
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem

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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
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
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json: cannot unmarshal array into Go value of type spdk.NvmfGetSubsystemStatsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem

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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_CreateNvmeController(t *testing.T) {
	tests := map[string]struct {
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"valid request with invalid SPDK response": {
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create CTRL: %v", testControllerID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "json response error: Invalid parameters"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 17,
				},
			},
			&pb.NvmeController{
				Name: testControllerID,
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: -1,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 17,
				},
			},
			&testController,
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

			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceID] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController
			}
			if tt.out != nil {
				tt.out.Name = testControllerID
			}

			request := &pb.CreateNvmeControllerRequest{NvmeController: tt.in, NvmeControllerId: testControllerID}
			response, err := testEnv.client.CreateNvmeController(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_UpdateNvmeController(t *testing.T) {
	spec := &pb.NvmeControllerSpec{
		SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
		NvmeControllerId: 17,
	}
	tests := map[string]struct {
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request without SPDK": {
			&pb.NvmeController{
				Name: testControllerID,
				Spec: spec,
			},
			&pb.NvmeController{
				Name: testControllerID,
				Spec: spec,
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			},
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

			request := &pb.UpdateNvmeControllerRequest{NvmeController: tt.in}
			response, err := testEnv.client.UpdateNvmeController(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_ListNvmeControllers(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     []*pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request without SPDK": {
			testSubsystemID,
			[]*pb.NvmeController{
				{
					Name: testControllerID,
					Spec: &pb.NvmeControllerSpec{
						SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
						PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
						NvmeControllerId: 17,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				},
				{
					Name: "controller-test1",
					Spec: &pb.NvmeControllerSpec{
						SubsystemId:      &pc.ObjectKey{Value: "subsystem-test1"},
						PcieId:           &pb.PciEndpoint{PhysicalFunction: 2, VirtualFunction: 2},
						NvmeControllerId: 17,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[{"subnqn": "nqn.2022-09.io.spdk:opi3", "cntlid": 1, "name": "NvmeEmu0pf1", "type": "nvme", "pci_index": 1, "pci_bdf": "ca:00.3"},{"subnqn": "nqn.2022-09.io.spdk:opi3", "cntlid": 2, "name": "NvmeEmu0pf1", "type": "nvme", "pci_index": 2, "pci_bdf": "ca:00.4"},{"subnqn": "nqn.2022-09.io.spdk:opi3", "cntlid": 3, "name": "NvmeEmu0pf1", "type": "nvme", "pci_index": 3, "pci_bdf": "ca:00.5"}]}`},
			codes.OK,
			"",
			false,
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Nvme.Controllers["controller-test1"] = &pb.NvmeController{
				Name: "controller-test1",
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: "subsystem-test1"},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 2, VirtualFunction: 2},
					NvmeControllerId: 17,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			}

			request := &pb.ListNvmeControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeControllers(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvmeControllers, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvmeControllers)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_GetNvmeController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with valid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerID,
				Spec: &pb.NvmeControllerSpec{
					NvmeControllerId: 17,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			},
			[]string{""},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			"unknown-controller-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %s", "unknown-controller-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController

			request := &pb.GetNvmeControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeController(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_NvmeControllerStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with valid SPDK response": {
			testSubsystemID,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
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

			request := &pb.NvmeControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeControllerStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_CreateNvmeNamespace(t *testing.T) {
	spec := &pb.NvmeNamespaceSpec{
		SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
		HostNsid:    0,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	namespaceSpec := &pb.NvmeNamespaceSpec{
		SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
		HostNsid:    22,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	tests := map[string]struct {
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"valid request with invalid SPDK response": {
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":-1}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NS: %v", testNamespaceID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_ns: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":-1}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_ns: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":-1}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_ns: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			&pb.NvmeNamespace{
				Spec: namespaceSpec,
			},
			&pb.NvmeNamespace{
				Spec: namespaceSpec,
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":22}`},
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			&pb.NvmeNamespace{
				Spec: spec,
			},
			&testNamespace,
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

			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceID] = &testNamespace
			}
			if tt.out != nil {
				tt.out.Name = testNamespaceID
			}

			request := &pb.CreateNvmeNamespaceRequest{NvmeNamespace: tt.in, NvmeNamespaceId: testNamespaceID}
			response, err := testEnv.client.CreateNvmeNamespace(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_UpdateNvmeNamespace(t *testing.T) {
	spec := &pb.NvmeNamespaceSpec{
		SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
		HostNsid:    22,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	tests := map[string]struct {
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request without SPDK": {
			&pb.NvmeNamespace{
				Name: testNamespaceID,
				Spec: spec,
			},
			&pb.NvmeNamespace{
				Name: testNamespaceID,
				Spec: spec,
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
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

			request := &pb.UpdateNvmeNamespaceRequest{NvmeNamespace: tt.in}
			response, err := testEnv.client.UpdateNvmeNamespace(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_ListNvmeNamespaces(t *testing.T) {
	testNamespaces := []pb.NvmeNamespace{
		{
			Spec: &pb.NvmeNamespaceSpec{
				HostNsid: 11,
			},
		},
		{
			Spec: &pb.NvmeNamespaceSpec{
				HostNsid: 12,
			},
		},
		{
			Spec: &pb.NvmeNamespaceSpec{
				HostNsid: 13,
			},
		},
	}
	tests := map[string]struct {
		in      string
		out     []*pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find any namespaces for NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json: cannot unmarshal bool into Go value of type []spdk.NvmfGetSubsystemsResult"),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
			[]*pb.NvmeNamespace{
				&testNamespaces[0],
				&testNamespaces[1],
				&testNamespaces[2],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},` +
				`{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[` +
				`{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},` +
				`{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},` +
				`{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}` +
				`]}]}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
		"pagination overflow": {
			testSubsystemID,
			[]*pb.NvmeNamespace{
				&testNamespaces[0],
				&testNamespaces[1],
				&testNamespaces[2],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
			1000,
			"",
		},
		"pagination negative": {
			"volume-test",
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			"volume-test",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			testSubsystemID,
			[]*pb.NvmeNamespace{
				&testNamespaces[0],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination offset": {
			testSubsystemID,
			[]*pb.NvmeNamespace{
				&testNamespaces[1],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find subsystem %v", "unknown-namespace-id"),
			false,
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces["ns0"] = &testNamespaces[0]
			testEnv.opiSpdkServer.Nvme.Namespaces["ns1"] = &testNamespaces[1]
			testEnv.opiSpdkServer.Nvme.Namespaces["ns2"] = &testNamespaces[2]
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeNamespacesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeNamespaces(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvmeNamespaces, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvmeNamespaces)
				}
				// Empty NextPageToken indicates end of results list
				if tt.size != 1 && response.NextPageToken != "" {
					t.Error("Expected end of results, receieved non-empty next page token", response.NextPageToken)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_GetNvmeNamespace(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json: cannot unmarshal bool into Go value of type []spdk.NvmfGetSubsystemsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testNamespaceID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceID,
				Spec: &pb.NvmeNamespaceSpec{
					HostNsid: 22,
				},
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":22,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.GetNvmeNamespaceRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeNamespace(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_NvmeNamespaceStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with valid SPDK response": {
			testSubsystemID,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
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

			request := &pb.NvmeNamespaceStatsRequest{NamespaceId: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeNamespaceStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_DeleteNvmeNamespace(t *testing.T) {
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
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NS: %v", testNamespaceID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testNamespaceID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_ns: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_ns: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_ns: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testNamespaceID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.DeleteNvmeNamespaceRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeNamespace(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
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

func TestFrontEnd_DeleteNvmeController(t *testing.T) {
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
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN:ID %v", "nqn.2022-09.io.spdk:opi3:17"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testControllerID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testControllerID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-controller-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-controller-id"),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerID] = &testController

			request := &pb.DeleteNvmeControllerRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeController(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
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
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemID] = &testSubsystem

			request := &pb.DeleteNvmeSubsystemRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeSubsystem(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
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

func TestFrontEnd_NewTcpSubsystemListener(t *testing.T) {
	tests := map[string]struct {
		listenAddress string
		wantPanic     bool
		protocol      string
	}{
		"ipv4 valid address": {
			listenAddress: "10.10.10.10:12345",
			wantPanic:     false,
			protocol:      ipv4NvmeTCPProtocol,
		},
		"valid ipv6 addresses": {
			listenAddress: "[2002:0db0:8833:0000:0000:8a8a:0330:7337]:54321",
			wantPanic:     false,
			protocol:      ipv6NvmeTCPProtocol,
		},
		"empty string as listen address": {
			listenAddress: "",
			wantPanic:     true,
			protocol:      "",
		},
		"missing port": {
			listenAddress: "10.10.10.10",
			wantPanic:     true,
			protocol:      "",
		},
		"valid port invalid ip": {
			listenAddress: "wrong:12345",
			wantPanic:     true,
			protocol:      "",
		},
		"meaningless listen address": {
			listenAddress: "some string which is not ip address",
			wantPanic:     true,
			protocol:      "",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("NewTCPSubsystemListener() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			gotSubsysListener := NewTCPSubsystemListener(tt.listenAddress)
			host, port, _ := net.SplitHostPort(tt.listenAddress)
			wantSubsysListener := &tcpSubsystemListener{
				listenAddr: net.ParseIP(host),
				listenPort: port,
				protocol:   tt.protocol,
			}

			if !reflect.DeepEqual(gotSubsysListener, wantSubsysListener) {
				t.Errorf("Expect %v subsystem listener, received %v", wantSubsysListener, gotSubsysListener)
			}
		})
	}
}
