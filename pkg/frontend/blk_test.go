// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testVirtioCtrlID   = "virtio-blk-42"
	testVirtioCtrlName = server.ResourceIDToVolumeName(testVirtioCtrlID)
	testVirtioCtrl     = pb.VirtioBlk{
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 42},
		VolumeId: &pc.ObjectKey{Value: "Malloc42"},
		MaxIoQps: 1,
	}
)

func TestFrontEnd_CreateVirtioBlk(t *testing.T) {
	tests := map[string]struct {
		id      string
		in      *pb.VirtioBlk
		out     *pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		// "illegal resource_id": {
		// 	id:          "CapitalLettersNotAllowed",
		// 	in:          &testVirtioCtrl,
		// 	out:         nil,
		// 	spdk:        []string{""},
		// 	expectedErr: status.Error(codes.Unknown, fmt.Sprintf("error: user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0")),
		// },
		"valid virtio-blk creation": {
			id:      testVirtioCtrlID,
			in:      &testVirtioCtrl,
			out:     &testVirtioCtrl,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"spdk virtio-blk creation error": {
			id:      testVirtioCtrlID,
			in:      &testVirtioCtrl,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"some internal error"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  "vhost_create_blk_controller: json response error: some internal error",
		},
		"spdk virtio-blk creation returned false response with no error": {
			id:      testVirtioCtrlID,
			in:      &testVirtioCtrl,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create virtio-blk: %s", testVirtioCtrlID),
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			testEnv := createTestEnvironment(true, tt.spdk)
			defer testEnv.Close()

			if tt.out != nil {
				tt.out.Name = testVirtioCtrlName
			}

			request := &pb.CreateVirtioBlkRequest{VirtioBlk: tt.in, VirtioBlkId: tt.id}
			response, err := testEnv.client.CreateVirtioBlk(testEnv.ctx, request)

			if !proto.Equal(tt.out, response) {
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

func TestFrontEnd_UpdateVirtioBlk(t *testing.T) {
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.VirtioBlk
		out     *pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.VirtioBlk{
				Name: testVirtioCtrlName,
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
			&pb.VirtioBlk{
				Name: testVirtioCtrlName,
			},
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateVirtioBlk"),
			false,
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.VirtioBlk{
				Name:     server.ResourceIDToVolumeName("unknown-id"),
				PcieId:   &pb.PciEndpoint{PhysicalFunction: 42},
				VolumeId: &pc.ObjectKey{Value: "Malloc42"},
				MaxIoQps: 1,
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
			&pb.VirtioBlk{
				Name:     server.ResourceIDToVolumeName("unknown-id"),
				PcieId:   &pb.PciEndpoint{PhysicalFunction: 42},
				VolumeId: &pc.ObjectKey{Value: "Malloc42"},
				MaxIoQps: 1,
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
			&pb.VirtioBlk{Name: "-ABC-DEF"},
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlName] = &testVirtioCtrl

			request := &pb.UpdateVirtioBlkRequest{VirtioBlk: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateVirtioBlk(testEnv.ctx, request)

			if !proto.Equal(tt.out, response) {
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

func TestFrontEnd_ListVirtioBlks(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     []*pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with empty result SPDK response": {
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			"subsystem-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			"subsystem-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"valid request with valid SPDK response": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:     server.ResourceIDToVolumeName("VblkEmu0pf0"),
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     server.ResourceIDToVolumeName("VblkEmu0pf2"),
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     server.ResourceIDToVolumeName(testVirtioCtrlID),
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},` +
				`{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},` +
				`{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}` +
				`],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
		"pagination overflow": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:     server.ResourceIDToVolumeName("VblkEmu0pf0"),
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     server.ResourceIDToVolumeName("VblkEmu0pf2"),
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     testVirtioCtrlName,
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			true,
			1000,
			"",
		},
		"pagination negative": {
			testVirtioCtrlName,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			testVirtioCtrlName,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:     server.ResourceIDToVolumeName("VblkEmu0pf0"),
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination offset": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:     testVirtioCtrlName,
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}],"error":{"code":0,"message":""}}`},
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

			request := &pb.ListVirtioBlksRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListVirtioBlks(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetVirtioBlks(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetVirtioBlks())
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

func TestFrontEnd_GetVirtioBlk(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %d", 0),
			true,
		},
		"valid request with empty SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testVirtioCtrlName,
			&pb.VirtioBlk{
				Name:     testVirtioCtrlName,
				PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
				VolumeId: &pc.ObjectKey{Value: "TBD"},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"virtio-blk-42","iops_threshold":60000,"cpumask":"0x2","delay_base_us":100}],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			true,
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlName] = &testVirtioCtrl
			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlName].Name = testVirtioCtrlName

			request := &pb.GetVirtioBlkRequest{Name: tt.in}
			response, err := testEnv.client.GetVirtioBlk(testEnv.ctx, request)

			if !proto.Equal(tt.out, response) {
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

func TestFrontEnd_VirtioBlkStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"unimplemented method": {
			testVirtioCtrlID,
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "VirtioBlkStats"),
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID] = &testVirtioCtrl

			request := &pb.VirtioBlkStatsRequest{ControllerId: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.VirtioBlkStats(testEnv.ctx, request)

			if !proto.Equal(tt.out, response.GetStats()) {
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

func TestFrontEnd_DeleteVirtioBlk(t *testing.T) {
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
			testVirtioCtrlID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete virtio-blk: %s", testVirtioCtrlID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("vhost_delete_controller: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("vhost_delete_controller: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("vhost_delete_controller: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testVirtioCtrlID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID] = &testVirtioCtrl

			request := &pb.DeleteVirtioBlkRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteVirtioBlk(testEnv.ctx, request)

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
