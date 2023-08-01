// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testControllerID   = "controller-test"
	testControllerName = server.ResourceIDToVolumeName(testControllerID)
	testController     = pb.NvmeController{
		Spec: &pb.NvmeControllerSpec{
			SubsystemNameRef: testSubsystemName,
			PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
			NvmeControllerId: 17,
		},
		Status: &pb.NvmeControllerStatus{
			Active: true,
		},
	}
)

func TestFrontEnd_CreateNvmeController(t *testing.T) {
	tests := map[string]struct {
		id      string
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
		},
		"valid request with invalid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create CTRL: %v", testControllerName),
			false,
		},
		"valid request with empty SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 1,
				},
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "json response error: Invalid parameters"),
			false,
		},
		"valid request with valid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 17,
				},
			},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
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
			false,
		},
		"already exists": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
					NvmeControllerId: 17,
				},
			},
			&testController,
			[]string{},
			codes.OK,
			"",
			true,
		},
		"no required field": {
			testControllerID,
			nil,
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: nvme_controller",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController
			}
			if tt.out != nil {
				tt.out.Name = testControllerName
			}

			request := &pb.CreateNvmeControllerRequest{NvmeController: tt.in, NvmeControllerId: tt.id}
			response, err := testEnv.client.CreateNvmeController(testEnv.ctx, request)

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

func TestFrontEnd_DeleteNvmeController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN:ID %v", "nqn.2022-09.io.spdk:opi3:17"),
			false,
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			testControllerName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-controller-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-controller-id")),
			false,
		},
		"unknown key with missing allowed": {
			server.ResourceIDToVolumeName("unknown-id"),
			&emptypb.Empty{},
			[]string{},
			codes.OK,
			"",
			true,
		},
		"malformed name": {
			"-ABC-DEF",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
		"no required field": {
			"",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			"missing required field: name",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController

			request := &pb.DeleteNvmeControllerRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeController(testEnv.ctx, request)

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

func TestFrontEnd_UpdateNvmeController(t *testing.T) {
	spec := &pb.NvmeControllerSpec{
		SubsystemNameRef: testSubsystemName,
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
		NvmeControllerId: 17,
	}
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
		},
		"valid request without SPDK": {
			nil,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			},
			[]string{},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeController{
				Name: server.ResourceIDToVolumeName("unknown-id"),
				Spec: spec,
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
		},
		"unknown key with missing allowed": {
			nil,
			&pb.NvmeController{
				Name: server.ResourceIDToVolumeName("unknown-id"),
				Spec: spec,
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			true,
		},
		"malformed name": {
			nil,
			&pb.NvmeController{Name: "-ABC-DEF"},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController

			request := &pb.UpdateNvmeControllerRequest{NvmeController: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateNvmeController(testEnv.ctx, request)

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

func TestFrontEnd_ListNvmeControllers(t *testing.T) {
	secondSubsystemName := server.ResourceIDToVolumeName("controller-test1")
	tests := map[string]struct {
		in      string
		out     []*pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request without SPDK": {
			testSubsystemName,
			[]*pb.NvmeController{
				{
					Name: testControllerName,
					Spec: &pb.NvmeControllerSpec{
						SubsystemNameRef: testSubsystemName,
						PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
						NvmeControllerId: 17,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				},
				{
					Name: secondSubsystemName,
					Spec: &pb.NvmeControllerSpec{
						SubsystemNameRef: server.ResourceIDToVolumeName("subsystem-test1"),
						PcieId:           &pb.PciEndpoint{PhysicalFunction: 2, VirtualFunction: 2},
						NvmeControllerId: 17,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				},
			},
			[]string{},
			codes.OK,
			"",
			0,
			"",
		},
		"no required field": {
			"",
			[]*pb.NvmeController{},
			[]string{},
			codes.Unknown,
			"missing required field: parent",
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Nvme.Controllers[secondSubsystemName] = &pb.NvmeController{
				Name: secondSubsystemName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: server.ResourceIDToVolumeName("subsystem-test1"),
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 2, VirtualFunction: 2},
					NvmeControllerId: 17,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			}

			request := &pb.ListNvmeControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeControllers(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetNvmeControllers(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNvmeControllers())
			}

			// Empty NextPageToken indicates end of results list
			// TODO: uncomment when method is properly implemented
			// if tt.size != 1 && response.GetNextPageToken() != "" {
			// 	t.Error("Expected end of results, received non-empty next page token", response.GetNextPageToken())
			// }

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

func TestFrontEnd_GetNvmeController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with valid SPDK response": {
			testControllerName,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					NvmeControllerId: 17,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			},
			[]string{},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			"unknown-controller-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %s", "unknown-controller-id"),
		},
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"no required field": {
			"",
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: name",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController

			request := &pb.GetNvmeControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeController(testEnv.ctx, request)

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

func TestFrontEnd_NvmeControllerStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with valid SPDK response": {
			testControllerName,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
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

			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController

			request := &pb.NvmeControllerStatsRequest{Name: tt.in}
			response, err := testEnv.client.NvmeControllerStats(testEnv.ctx, request)

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
