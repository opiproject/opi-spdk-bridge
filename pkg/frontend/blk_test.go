// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

var (
	testVirtioCtrlID = "virtio-blk-42"
	testVirtioCtrl   = pb.VirtioBlk{
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 42},
		VolumeId: &pc.ObjectKey{Value: "Malloc42"},
		MaxIoQps: 1,
	}
)

func TestFrontEnd_CreateVirtioBlk(t *testing.T) {
	tests := map[string]struct {
		in          *pb.VirtioBlk
		out         *pb.VirtioBlk
		spdk        []string
		expectedErr error
	}{
		"valid virtio-blk creation": {
			in:          &testVirtioCtrl,
			out:         &testVirtioCtrl,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			expectedErr: status.Error(codes.OK, ""),
		},
		"spdk virtio-blk creation error": {
			in:          &testVirtioCtrl,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":1,"message":"some internal error"},"result":false}`},
			expectedErr: spdk.ErrFailedSpdkCall,
		},
		"spdk virtio-blk creation returned false response with no error": {
			in:          &testVirtioCtrl,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			expectedErr: spdk.ErrUnexpectedSpdkCallResult,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testEnv := createTestEnvironment(true, test.spdk)
			defer testEnv.Close()

			if test.out != nil {
				test.out.Name = testVirtioCtrlID
			}

			request := &pb.CreateVirtioBlkRequest{VirtioBlk: test.in, VirtioBlkId: testVirtioCtrlID}
			response, err := testEnv.client.CreateVirtioBlk(testEnv.ctx, request)
			if response != nil {
				wantOut, _ := proto.Marshal(test.out)
				gotOut, _ := proto.Marshal(response)

				if !bytes.Equal(wantOut, gotOut) {
					t.Error("response: expected", test.out, "received", response)
				}
			} else if test.out != nil {
				t.Error("response: expected", test.out, "received nil")
			}

			if err != nil {
				if !strings.Contains(err.Error(), test.expectedErr.Error()) {
					t.Error("expected err contains", test.expectedErr, "received", err)
				}
			} else {
				if test.expectedErr != nil {
					t.Error("expected err contains", test.expectedErr, "received nil")
				}
			}
		})
	}
}

func TestFrontEnd_UpdateVirtioBlk(t *testing.T) {
	tests := map[string]struct {
		in      *pb.VirtioBlk
		out     *pb.VirtioBlk
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"unimplemented method": {
			&pb.VirtioBlk{},
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateVirtioBlk"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.UpdateVirtioBlkRequest{VirtioBlk: tt.in}
			response, err := testEnv.client.UpdateVirtioBlk(testEnv.ctx, request)
			if response != nil {
				t.Error("response: expected", codes.Unimplemented, "received", response)
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
		"valid request with invalid SPDK response": {
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NQN: %v", "nqn.2022-09.io.spdk:opi3"),
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
					Name:     "VblkEmu0pf0",
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     "VblkEmu0pf2",
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     testVirtioCtrlID,
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
					Name:     "VblkEmu0pf0",
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     "VblkEmu0pf2",
					PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
					VolumeId: &pc.ObjectKey{Value: "TBD"},
				},
				{
					Name:     testVirtioCtrlID,
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
			"subsystem-test",
			[]*pb.VirtioBlk{
				{
					Name:     "VblkEmu0pf0",
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
					Name:     testVirtioCtrlID,
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

			if response != nil {
				if !reflect.DeepEqual(response.VirtioBlks, tt.out) {
					t.Error("response: expected", tt.out, "received", response.VirtioBlks)
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
			testVirtioCtrlID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %d", 0),
			true,
		},
		"valid request with empty SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testVirtioCtrlID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("vhost_get_controllers: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testVirtioCtrlID,
			&pb.VirtioBlk{
				Name:     testVirtioCtrlID,
				PcieId:   &pb.PciEndpoint{PhysicalFunction: int32(1)},
				VolumeId: &pc.ObjectKey{Value: "TBD"},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"ctrlr":"virtio-blk-42","iops_threshold":60000,"cpumask":"0x2","delay_base_us":100}],"error":{"code":0,"message":""}}`},
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

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID] = &testVirtioCtrl

			request := &pb.GetVirtioBlkRequest{Name: tt.in}
			response, err := testEnv.client.GetVirtioBlk(testEnv.ctx, request)
			if response != nil {
				wantOut, _ := proto.Marshal(tt.out)
				gotOut, _ := proto.Marshal(response)
				if !bytes.Equal(wantOut, gotOut) {
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
			"test",
			&pb.VolumeStats{},
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "VirtioBlkStats"),
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
			if response != nil {
				t.Error("response: expected", codes.Unimplemented, "received", response)
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
			fmt.Sprintf("Could not delete NQN:ID %v", "nqn.2022-09.io.spdk:opi3:17"),
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
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Virt.BlkCtrls[testVirtioCtrlID] = &testVirtioCtrl

			request := &pb.DeleteVirtioBlkRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteVirtioBlk(testEnv.ctx, request)
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
