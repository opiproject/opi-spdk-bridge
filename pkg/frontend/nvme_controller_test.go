// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var (
	testControllerID   = "controller-test"
	testControllerName = utils.ResourceIDToControllerName(testSubsystemID, testControllerID)
	testController     = pb.NvmeController{
		Spec: &pb.NvmeControllerSpec{
			Endpoint: &pb.NvmeControllerSpec_FabricsId{
				FabricsId: &pb.FabricsEndpoint{
					Traddr:  "127.0.0.1",
					Trsvcid: "4420",
					Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
				},
			},
			Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
			NvmeControllerId: proto.Int32(17),
		},
		Status: &pb.NvmeControllerStatus{
			Active: true,
		},
	}
)

func TestFrontEnd_CreateNvmeController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		id      string
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
		subsys  string
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
			testSubsystemName,
		},
		"valid request with invalid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create CTRL: %v", testControllerName),
			false,
			testSubsystemName,
		},
		"valid request with empty SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "EOF"),
			false,
			testSubsystemName,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "json response ID mismatch"),
			false,
			testSubsystemName,
		},
		"valid request with error code from SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_listener: %v", "json response error: Invalid parameters"),
			false,
			testSubsystemName,
		},
		"valid request with valid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(17),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(-1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			codes.OK,
			"",
			false,
			testSubsystemName,
		},
		"already exists": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(17),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			&testController,
			[]string{},
			codes.OK,
			"",
			true,
			testSubsystemName,
		},
		"malformed subsystem name": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
			"-ABC-DEF",
		},
		"no required ctrl field": {
			testControllerID,
			nil,
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: nvme_controller",
			false,
			testSubsystemName,
		},
		"no required parent field": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					NvmeControllerId: proto.Int32(1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: parent",
			false,
			"",
		},
		"not corresponding endpoint for pcie transport type": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint: &pb.NvmeControllerSpec_FabricsId{
						FabricsId: &pb.FabricsEndpoint{
							Traddr:  "127.0.0.1",
							Trsvcid: "4420",
							Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
						},
					},
					Trtype: pb.NvmeTransportType_NVME_TRANSPORT_PCIE,
				},
			},
			nil,
			[]string{},
			codes.Unknown,
			"invalid endpoint type passed for transport",
			false,
			testSubsystemName,
		},
		"not corresponding endpoint for tcp transport type": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint: &pb.NvmeControllerSpec_PcieId{
						PcieId: &pb.PciEndpoint{
							PortId:           wrapperspb.Int32(0),
							PhysicalFunction: wrapperspb.Int32(0),
							VirtualFunction:  wrapperspb.Int32(0),
						},
					},
					Trtype: pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
			},
			nil,
			[]string{},
			codes.Unknown,
			"invalid endpoint type passed for transport",
			false,
			testSubsystemName,
		},
		"not supported transport type": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint: &pb.NvmeControllerSpec_PcieId{
						PcieId: &pb.PciEndpoint{
							PortId:           wrapperspb.Int32(0),
							PhysicalFunction: wrapperspb.Int32(0),
							VirtualFunction:  wrapperspb.Int32(0),
						},
					},
					Trtype: pb.NvmeTransportType_NVME_TRANSPORT_CUSTOM,
				},
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("not supported transport type: %v", pb.NvmeTransportType_NVME_TRANSPORT_CUSTOM),
			false,
			testSubsystemName,
		},
		"no transport registered": {
			testControllerID,
			&pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					Endpoint: &pb.NvmeControllerSpec_PcieId{
						PcieId: &pb.PciEndpoint{
							PortId:           wrapperspb.Int32(0),
							PhysicalFunction: wrapperspb.Int32(0),
							VirtualFunction:  wrapperspb.Int32(0),
						},
					},
					Trtype: pb.NvmeTransportType_NVME_TRANSPORT_PCIE,
				},
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("handler for transport type %v is not registered", pb.NvmeTransportType_NVME_TRANSPORT_PCIE),
			false,
			testSubsystemName,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespace)
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = utils.ProtoClone(&testController)
				testEnv.opiSpdkServer.Nvme.Controllers[testControllerName].Name = testControllerName
			}
			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
				tt.out.Name = testControllerName
			}

			request := &pb.CreateNvmeControllerRequest{Parent: tt.subsys, NvmeController: tt.in, NvmeControllerId: tt.id}
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in         string
		out        *emptypb.Empty
		spdk       []string
		errCode    codes.Code
		errMsg     string
		missing    bool
		controller *pb.NvmeController
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete CTRL: %v", testControllerName),
			false,
			&testController,
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "EOF"),
			false,
			&testController,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "json response ID mismatch"),
			false,
			&testController,
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_listener: %v", "json response error: myopierr"),
			false,
			&testController,
		},
		"valid request with valid SPDK response": {
			testControllerName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			false,
			&testController,
		},
		"valid request with unknown key": {
			utils.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id")),
			false,
			&testController,
		},
		"unknown key with missing allowed": {
			utils.ResourceIDToControllerName(testSubsystemID, "unknown-id"),
			&emptypb.Empty{},
			[]string{},
			codes.OK,
			"",
			true,
			&testController,
		},
		"malformed name": {
			"-ABC-DEF",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
			&testController,
		},
		"no required field": {
			"",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			"missing required field: name",
			false,
			&testController,
		},
		"no transport registered": {
			testControllerName,
			&emptypb.Empty{},
			[]string{},
			codes.NotFound,
			fmt.Sprintf("handler for transport type %v is not registered", pb.NvmeTransportType_NVME_TRANSPORT_PCIE),
			false,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					Endpoint: &pb.NvmeControllerSpec_PcieId{
						PcieId: &pb.PciEndpoint{
							PortId:           wrapperspb.Int32(0),
							PhysicalFunction: wrapperspb.Int32(0),
							VirtualFunction:  wrapperspb.Int32(0),
						},
					},
					Trtype: pb.NvmeTransportType_NVME_TRANSPORT_PCIE,
				},
			},
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = utils.ProtoClone(tt.controller)
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName].Name = testControllerName

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
		Endpoint:         testController.Spec.Endpoint,
		NvmeControllerId: proto.Int32(17),
		Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
	}
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(spec)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

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
				Name: utils.ResourceIDToVolumeName("unknown-id"),
				Spec: spec,
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			false,
		},
		"unknown key with missing allowed": {
			nil,
			&pb.NvmeController{
				Name: utils.ResourceIDToVolumeName("unknown-id"),
				Spec: spec,
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			true,
		},
		"malformed name": {
			nil,
			&pb.NvmeController{Name: "-ABC-DEF", Spec: spec},
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

			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = utils.ProtoClone(&testController)

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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	secondSubsystemName := utils.ResourceIDToVolumeName("controller-test1")
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
						Endpoint:         testController.Spec.Endpoint,
						NvmeControllerId: proto.Int32(17),
						Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				},
				{
					Name: secondSubsystemName,
					Spec: &pb.NvmeControllerSpec{
						Endpoint:         testController.Spec.Endpoint,
						NvmeControllerId: proto.Int32(18),
						Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = utils.ProtoClone(&pb.NvmeController{
				Name:   testControllerName,
				Spec:   testController.Spec,
				Status: testController.Status,
			})
			testEnv.opiSpdkServer.Nvme.Controllers[secondSubsystemName] = utils.ProtoClone(&pb.NvmeController{
				Name: secondSubsystemName,
				Spec: &pb.NvmeControllerSpec{
					Endpoint:         testController.Spec.Endpoint,
					NvmeControllerId: proto.Int32(18),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			})

			request := &pb.ListNvmeControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeControllers(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetNvmeControllers(), tt.out) {
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
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
					NvmeControllerId: proto.Int32(17),
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = utils.ProtoClone(&testController)

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

func TestFrontEnd_StatsNvmeController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
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
			utils.ResourceIDToVolumeName("unknown-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
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

			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = utils.ProtoClone(&testController)

			request := &pb.StatsNvmeControllerRequest{Name: tt.in}
			response, err := testEnv.client.StatsNvmeController(testEnv.ctx, request)

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
