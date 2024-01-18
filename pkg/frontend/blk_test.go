// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2024 Dell Inc, or its subsidiaries.
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
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var (
	testVirtioCtrlID   = "virtio-blk-42"
	testVirtioCtrlName = utils.ResourceIDToVolumeName(testVirtioCtrlID)
	testVirtioCtrl     = pb.VirtioBlk{
		PcieId: &pb.PciEndpoint{
			PhysicalFunction: wrapperspb.Int32(42),
			VirtualFunction:  wrapperspb.Int32(0),
			PortId:           wrapperspb.Int32(0)},
		VolumeNameRef: "Malloc42",
		MaxIoQps:      1,
	}
)

func TestFrontEnd_CreateVirtioBlk(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		id      string
		in      *pb.VirtioBlk
		out     *pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		// "illegal resource_id": {
		// 	id:      "CapitalLettersNotAllowed",
		// 	in:      &testVirtioCtrl,
		// 	out:     nil,
		// 	spdk:    []string{""},
		// 	errCode: codes.Unknown,
		// 	errMsg:  fmt.Sprintf("error: user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
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
		"no required field": {
			id:      testVirtioCtrlID,
			in:      nil,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: virtio_blk",
		},
		"no required volume field": {
			id:      testVirtioCtrlID,
			in:      &pb.VirtioBlk{PcieId: testVirtioCtrl.PcieId},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: virtio_blk.volume_name_ref",
		},
		"malformed volume name": {
			id:      testVirtioCtrlID,
			in:      &pb.VirtioBlk{VolumeNameRef: "-ABC-DEF", PcieId: testVirtioCtrl.PcieId},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"virtual functions are not supported for vhost user": {
			id: testVirtioCtrlID,
			in: &pb.VirtioBlk{
				PcieId: &pb.PciEndpoint{
					PhysicalFunction: wrapperspb.Int32(42),
					VirtualFunction:  wrapperspb.Int32(1),
					PortId:           wrapperspb.Int32(0)},
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "virtual functions are not supported for vhost-user-blk",
		},
		"only port 0 is supported for vhost user": {
			id: testVirtioCtrlID,
			in: &pb.VirtioBlk{
				PcieId: &pb.PciEndpoint{
					PhysicalFunction: wrapperspb.Int32(42),
					VirtualFunction:  wrapperspb.Int32(0),
					PortId:           wrapperspb.Int32(1)},
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only port 0 is supported for vhost-user-blk",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.VirtioBlk
		out     *pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.VirtioBlk{
				Name:          testVirtioCtrlName,
				VolumeNameRef: "TBD",
				PcieId:        testVirtioCtrl.PcieId,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
		},
		"unimplemented method": {
			nil,
			&pb.VirtioBlk{
				Name:          testVirtioCtrlName,
				VolumeNameRef: "TBD",
				PcieId:        testVirtioCtrl.PcieId,
			},
			nil,
			[]string{},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateVirtioBlk"),
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.VirtioBlk{
				Name:          utils.ResourceIDToVolumeName("unknown-id"),
				PcieId:        testVirtioCtrl.PcieId,
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			false,
		},
		"unknown key with missing allowed": {
			nil,
			&pb.VirtioBlk{
				Name:          utils.ResourceIDToVolumeName("unknown-id"),
				PcieId:        testVirtioCtrl.PcieId,
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			true,
		},
		"malformed name": {
			nil,
			&pb.VirtioBlk{Name: "-ABC-DEF", VolumeNameRef: "TBD", PcieId: testVirtioCtrl.PcieId},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
		"malformed volume name": {
			nil,
			&pb.VirtioBlk{Name: "TBD", VolumeNameRef: "-ABC-DEF", PcieId: testVirtioCtrl.PcieId},
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlName] = utils.ProtoClone(&testVirtioCtrl)

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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     []*pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with empty result SPDK response": {
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.OK,
			"",
			0,
			"",
		},
		"valid request with empty SPDK response": {
			"subsystem-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "EOF"),
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			"subsystem-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response ID mismatch"),
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response error: myopierr"),
			0,
			"",
		},
		"valid request with valid SPDK response": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:          utils.ResourceIDToVolumeName("VblkEmu0pf0"),
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
				{
					Name:          utils.ResourceIDToVolumeName("VblkEmu0pf2"),
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
				{
					Name:          utils.ResourceIDToVolumeName(testVirtioCtrlID),
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},` +
				`{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},` +
				`{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}` +
				`],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			0,
			"",
		},
		"pagination overflow": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:          utils.ResourceIDToVolumeName("VblkEmu0pf0"),
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
				{
					Name:          utils.ResourceIDToVolumeName("VblkEmu0pf2"),
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
				{
					Name:          testVirtioCtrlName,
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			1000,
			"",
		},
		"pagination negative": {
			testVirtioCtrlName,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			-10,
			"",
		},
		"pagination error": {
			testVirtioCtrlName,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:          utils.ResourceIDToVolumeName("VblkEmu0pf0"),
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			1,
			"",
		},
		"pagination offset": {
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:          testVirtioCtrlName,
					PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
					VolumeNameRef: "TBD",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"VblkEmu0pf0","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"virtio-blk-42","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"},{"ctrlr":"VblkEmu0pf2","emulation_manager":"mlx5_0","type":"virtio_blk","pci_index":0,"pci_bdf":"ca:00.4"}],"error":{"code":0,"message":""}}`},
			codes.OK,
			"",
			1,
			"existing-pagination-token",
		},
		"no required field": {
			"",
			[]*pb.VirtioBlk{},
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

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListVirtioBlksRequest{PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListVirtioBlks(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetVirtioBlks(), tt.out) {
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %d", 0),
		},
		"valid request with empty SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testVirtioCtrlName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testVirtioCtrlName,
			&pb.VirtioBlk{
				Name:          testVirtioCtrlName,
				PcieId:        &pb.PciEndpoint{PhysicalFunction: wrapperspb.Int32(1), VirtualFunction: wrapperspb.Int32(0), PortId: wrapperspb.Int32(0)},
				VolumeNameRef: "TBD",
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"virtio-blk-42","iops_threshold":60000,"cpumask":"0x2","delay_base_us":100}],"error":{"code":0,"message":""}}`},
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlName] = utils.ProtoClone(&testVirtioCtrl)
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

func TestFrontEnd_StatsVirtioBlk(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"unimplemented method": {
			testVirtioCtrlID,
			nil,
			[]string{},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "StatsVirtioBlk"),
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID] = utils.ProtoClone(&testVirtioCtrl)

			request := &pb.StatsVirtioBlkRequest{Name: tt.in}
			response, err := testEnv.client.StatsVirtioBlk(testEnv.ctx, request)

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
	pfIndex := 0
	// vfIndex := 1
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
		pfVf    int
	}{
		"valid request with invalid SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete virtio-blk: %s", testVirtioCtrlID),
			false,
			pfIndex,
		},
		"valid request with empty SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("vhost_delete_controller: %v", "EOF"),
			false,
			pfIndex,
		},
		"valid request with ID mismatch SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("vhost_delete_controller: %v", "json response ID mismatch"),
			false,
			pfIndex,
		},
		"valid request with error code from SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("vhost_delete_controller: %v", "json response error: myopierr"),
			false,
			pfIndex,
		},
		"valid request with valid SPDK response": {
			testVirtioCtrlID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			false,
			pfIndex,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
			false,
			pfIndex,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			[]string{},
			codes.OK,
			"",
			true,
			pfIndex,
		},
		"malformed name": {
			"-ABC-DEF",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
			pfIndex,
		},
		"no required field": {
			"",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			"missing required field: name",
			false,
			pfIndex,
		},
		// "entry with invalid address in db": {
		// 	testVirtioCtrlID,
		// 	&emptypb.Empty{},
		// 	[]string{},
		// 	codes.Internal,
		// 	"virtual functions are not supported for vhost user",
		// 	false,
		// 	vfIndex,
		// },
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID] = utils.ProtoClone(&testVirtioCtrl)
			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID].PcieId.VirtualFunction =
				wrapperspb.Int32(int32(tt.pfVf))
			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID].Name = testVirtioCtrlID

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
