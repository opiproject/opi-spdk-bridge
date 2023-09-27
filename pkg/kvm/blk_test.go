// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"testing"

	"github.com/philippgille/gokv/gomap"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	testVirtioBlkID            = "virtio-blk-42"
	testVirtioBlkName          = utils.ResourceIDToVolumeName(testVirtioBlkID)
	testCreateVirtioBlkRequest = &pb.CreateVirtioBlkRequest{VirtioBlkId: testVirtioBlkID, VirtioBlk: &pb.VirtioBlk{
		PcieId: &pb.PciEndpoint{
			PhysicalFunction: wrapperspb.Int32(42),
			VirtualFunction:  wrapperspb.Int32(0),
			PortId:           wrapperspb.Int32(0),
		},
		VolumeNameRef: "Malloc42",
		MaxIoQps:      1,
	}}
	testDeleteVirtioBlkRequest = &pb.DeleteVirtioBlkRequest{Name: testVirtioBlkName}
)

func TestCreateVirtioBlk(t *testing.T) {
	expectNotNilOut := utils.ProtoClone(testCreateVirtioBlkRequest.VirtioBlk)
	expectNotNilOut.Name = testVirtioBlkName
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(expectNotNilOut)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		jsonRPC              spdk.JSONRPC
		errCode              codes.Code
		errMsg               string
		nonDefaultQmpAddress string
		buses                []string

		in  *pb.CreateVirtioBlkRequest
		out *pb.VirtioBlk

		mockQmpCalls *mockQmpCalls
	}{
		"valid virtio-blk creation": {
			in:      testCreateVirtioBlkRequest,
			jsonRPC: alwaysSuccessfulJSONRPC,
			out:     expectNotNilOut,
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).
				ExpectAddVirtioBlk(testVirtioBlkID, testVirtioBlkID).
				ExpectQueryPci(testVirtioBlkID),
		},
		"spdk failed to create virtio-blk": {
			in:      testCreateVirtioBlkRequest,
			jsonRPC: alwaysFailingJSONRPC,
			errCode: status.Convert(errStub).Code(),
			errMsg:  status.Convert(errStub).Message(),
		},
		"qemu chardev add failed": {
			in:      testCreateVirtioBlkRequest,
			jsonRPC: alwaysSuccessfulJSONRPC,
			errCode: status.Convert(errAddChardevFailed).Code(),
			errMsg:  status.Convert(errAddChardevFailed).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).WithErrorResponse(),
		},
		"qemu device add failed": {
			in:      testCreateVirtioBlkRequest,
			jsonRPC: alwaysSuccessfulJSONRPC,
			errCode: status.Convert(errAddDeviceFailed).Code(),
			errMsg:  status.Convert(errAddDeviceFailed).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).
				ExpectAddVirtioBlk(testVirtioBlkID, testVirtioBlkID).WithErrorResponse().
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"failed to create monitor": {
			in:                   testCreateVirtioBlkRequest,
			nonDefaultQmpAddress: "/dev/null",
			jsonRPC:              alwaysSuccessfulJSONRPC,
			errCode:              status.Convert(errMonitorCreation).Code(),
			errMsg:               status.Convert(errMonitorCreation).Message(),
		},
		"valid virtio-blk creation with on first bus location": {
			in: &pb.CreateVirtioBlkRequest{VirtioBlk: &pb.VirtioBlk{
				PcieId: &pb.PciEndpoint{
					PhysicalFunction: wrapperspb.Int32(1),
					VirtualFunction:  wrapperspb.Int32(0),
					PortId:           wrapperspb.Int32(0)},
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			}, VirtioBlkId: testVirtioBlkID},
			out: &pb.VirtioBlk{
				Name: testVirtioBlkName,
				PcieId: &pb.PciEndpoint{
					PhysicalFunction: wrapperspb.Int32(1),
					VirtualFunction:  wrapperspb.Int32(0),
					PortId:           wrapperspb.Int32(0)},
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			},
			jsonRPC: alwaysSuccessfulJSONRPC,
			buses:   []string{"pci.opi.0", "pci.opi.1"},
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).
				ExpectAddVirtioBlkWithAddress(testVirtioBlkID, testVirtioBlkID, "pci.opi.0", 1).
				ExpectQueryPci(testVirtioBlkID),
		},
		"valid virtio-blk creation with on second bus location": {
			in:      testCreateVirtioBlkRequest,
			out:     expectNotNilOut,
			jsonRPC: alwaysSuccessfulJSONRPC,
			buses:   []string{"pci.opi.0", "pci.opi.1"},
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).
				ExpectAddVirtioBlkWithAddress(testVirtioBlkID, testVirtioBlkID, "pci.opi.1", 10).
				ExpectQueryPci(testVirtioBlkID),
		},
		"virtio-blk creation with physical function goes out of buses": {
			in:      testCreateVirtioBlkRequest,
			out:     nil,
			errCode: status.Convert(errDeviceEndpoint).Code(),
			errMsg:  status.Convert(errDeviceEndpoint).Message(),
			jsonRPC: alwaysSuccessfulJSONRPC,
			buses:   []string{"pci.opi.0"},
		},
		"negative physical function": {
			in: &pb.CreateVirtioBlkRequest{VirtioBlk: &pb.VirtioBlk{
				PcieId: &pb.PciEndpoint{
					PhysicalFunction: wrapperspb.Int32(-1),
					VirtualFunction:  wrapperspb.Int32(0),
					PortId:           wrapperspb.Int32(0)},
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			}, VirtioBlkId: testVirtioBlkID},
			out:     nil,
			errCode: status.Convert(errDeviceEndpoint).Code(),
			errMsg:  status.Convert(errDeviceEndpoint).Message(),
			jsonRPC: alwaysSuccessfulJSONRPC,
			buses:   []string{"pci.opi.0"},
		},
		"nil pcie endpoint": {
			in: &pb.CreateVirtioBlkRequest{VirtioBlk: &pb.VirtioBlk{
				PcieId:        nil,
				VolumeNameRef: "Malloc42",
				MaxIoQps:      1,
			}, VirtioBlkId: testVirtioBlkID},
			out:     nil,
			errCode: status.Convert(errNoPcieEndpoint).Code(),
			errMsg:  status.Convert(errNoPcieEndpoint).Message(),
			jsonRPC: alwaysSuccessfulJSONRPC,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			options := gomap.DefaultOptions
			options.Codec = utils.ProtoCodec{}
			store := gomap.NewStore(options)
			opiSpdkServer := frontend.NewServer(tt.jsonRPC, store)
			qmpServer := startMockQmpServer(t, tt.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if tt.nonDefaultQmpAddress != "" {
				qmpAddress = tt.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir, tt.buses)
			kvmServer.timeout = qmplibTimeout
			request := utils.ProtoClone(tt.in)

			out, err := kvmServer.CreateVirtioBlk(context.Background(), request)

			if !proto.Equal(out, tt.out) {
				t.Error("response: expected", tt.out, "received", out)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expected grpc error status")
			}

			if !qmpServer.WereExpectedCallsPerformed() {
				t.Errorf("Not all expected calls were performed")
			}
		})
	}
}

func TestDeleteVirtioBlk(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		jsonRPC              spdk.JSONRPC
		errCode              codes.Code
		errMsg               string
		nonDefaultQmpAddress string

		mockQmpCalls *mockQmpCalls
	}{
		"valid virtio-blk deletion": {
			jsonRPC: alwaysSuccessfulJSONRPC,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlkWithEvent(testVirtioBlkID).
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"qemu device delete failed": {
			jsonRPC: alwaysSuccessfulJSONRPC,
			errCode: status.Convert(errDevicePartiallyDeleted).Code(),
			errMsg:  status.Convert(errDevicePartiallyDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlk(testVirtioBlkID).WithErrorResponse().
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"qemu device delete failed by timeout": {
			jsonRPC: alwaysSuccessfulJSONRPC,
			errCode: status.Convert(errDevicePartiallyDeleted).Code(),
			errMsg:  status.Convert(errDevicePartiallyDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlk(testVirtioBlkID).
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"qemu chardev delete failed": {
			jsonRPC: alwaysSuccessfulJSONRPC,
			errCode: status.Convert(errDevicePartiallyDeleted).Code(),
			errMsg:  status.Convert(errDevicePartiallyDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlkWithEvent(testVirtioBlkID).
				ExpectDeleteChardev(testVirtioBlkID).WithErrorResponse(),
		},
		"spdk failed to delete virtio-blk": {
			jsonRPC: alwaysFailingJSONRPC,
			errCode: status.Convert(errDevicePartiallyDeleted).Code(),
			errMsg:  status.Convert(errDevicePartiallyDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlkWithEvent(testVirtioBlkID).
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"all qemu and spdk calls failed": {
			jsonRPC: alwaysFailingJSONRPC,
			errCode: status.Convert(errDeviceNotDeleted).Code(),
			errMsg:  status.Convert(errDeviceNotDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlk(testVirtioBlkID).WithErrorResponse().
				ExpectDeleteChardev(testVirtioBlkID).WithErrorResponse(),
		},
		"failed to create monitor": {
			nonDefaultQmpAddress: "/dev/null",
			jsonRPC:              alwaysSuccessfulJSONRPC,
			errCode:              status.Convert(errMonitorCreation).Code(),
			errMsg:               status.Convert(errMonitorCreation).Message(),
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			options := gomap.DefaultOptions
			options.Codec = utils.ProtoCodec{}
			store := gomap.NewStore(options)
			opiSpdkServer := frontend.NewServer(tt.jsonRPC, store)
			opiSpdkServer.Virt.BlkCtrls[testVirtioBlkName] =
				utils.ProtoClone(testCreateVirtioBlkRequest.VirtioBlk)
			opiSpdkServer.Virt.BlkCtrls[testVirtioBlkName].Name = testVirtioBlkName
			qmpServer := startMockQmpServer(t, tt.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if tt.nonDefaultQmpAddress != "" {
				qmpAddress = tt.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir, nil)
			kvmServer.timeout = qmplibTimeout
			request := utils.ProtoClone(testDeleteVirtioBlkRequest)

			_, err := kvmServer.DeleteVirtioBlk(context.Background(), request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expected grpc error status")
			}

			if !qmpServer.WereExpectedCallsPerformed() {
				t.Errorf("Not all expected calls were performed")
			}
		})
	}
}
