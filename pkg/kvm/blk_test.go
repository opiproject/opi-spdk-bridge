// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"

	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/ulule/deepcopier"
	"google.golang.org/protobuf/proto"
)

var (
	testVirtioBlkID            = "virtio-blk-42"
	testCreateVirtioBlkRequest = &pb.CreateVirtioBlkRequest{VirtioBlkId: testVirtioBlkID, VirtioBlk: &pb.VirtioBlk{
		Name:     "",
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 42},
		VolumeId: &pc.ObjectKey{Value: "Malloc42"},
		MaxIoQps: 1,
	}}
	testDeleteVirtioBlkRequest = &pb.DeleteVirtioBlkRequest{Name: testVirtioBlkID}
)

func TestCreateVirtioBlk(t *testing.T) {
	expectNotNilOut := &pb.VirtioBlk{}
	if deepcopier.Copy(testCreateVirtioBlkRequest.VirtioBlk).To(expectNotNilOut) != nil {
		log.Panicf("Failed to copy structure")
	}
	expectNotNilOut.Name = testVirtioBlkID

	tests := map[string]struct {
		jsonRPC              spdk.JSONRPC
		expectError          error
		nonDefaultQmpAddress string

		out *pb.VirtioBlk

		mockQmpCalls *mockQmpCalls
	}{
		"valid virtio-blk creation": {
			jsonRPC: alwaysSuccessfulJSONRPC,
			out:     expectNotNilOut,
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).
				ExpectAddVirtioBlk(testVirtioBlkID, testVirtioBlkID).
				ExpectQueryPci(testVirtioBlkID),
		},
		"spdk failed to create virtio-blk": {
			jsonRPC:     alwaysFailingJSONRPC,
			expectError: spdk.ErrFailedSpdkCall,
		},
		"qemu chardev add failed": {
			jsonRPC:     alwaysSuccessfulJSONRPC,
			expectError: errAddChardevFailed,
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).WithErrorResponse(),
		},
		"qemu device add failed": {
			jsonRPC:     alwaysSuccessfulJSONRPC,
			expectError: errAddDeviceFailed,
			mockQmpCalls: newMockQmpCalls().
				ExpectAddChardev(testVirtioBlkID).
				ExpectAddVirtioBlk(testVirtioBlkID, testVirtioBlkID).WithErrorResponse().
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"failed to create monitor": {
			nonDefaultQmpAddress: "/dev/null",
			jsonRPC:              alwaysSuccessfulJSONRPC,
			expectError:          errMonitorCreation,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opiSpdkServer := frontend.NewServer(test.jsonRPC)
			qmpServer := startMockQmpServer(t, test.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if test.nonDefaultQmpAddress != "" {
				qmpAddress = test.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir, nil, nil)
			kvmServer.timeout = qmplibTimeout

			out, err := kvmServer.CreateVirtioBlk(context.Background(), testCreateVirtioBlkRequest)
			if !errors.Is(err, test.expectError) {
				t.Errorf("Expected error %v, got %v", test.expectError, err)
			}
			gotOut, _ := proto.Marshal(out)
			wantOut, _ := proto.Marshal(test.out)
			if !bytes.Equal(gotOut, wantOut) {
				t.Errorf("Expected out %v, got %v", test.out, out)
			}
			if !qmpServer.WereExpectedCallsPerformed() {
				t.Errorf("Not all expected calls were performed")
			}
		})
	}
}

func TestDeleteVirtioBlk(t *testing.T) {
	tests := map[string]struct {
		jsonRPC              spdk.JSONRPC
		expectError          error
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
			jsonRPC:     alwaysSuccessfulJSONRPC,
			expectError: errDevicePartiallyDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlk(testVirtioBlkID).WithErrorResponse().
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"qemu device delete failed by timeout": {
			jsonRPC:     alwaysSuccessfulJSONRPC,
			expectError: errDevicePartiallyDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlk(testVirtioBlkID).
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"qemu chardev delete failed": {
			jsonRPC:     alwaysSuccessfulJSONRPC,
			expectError: errDevicePartiallyDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlkWithEvent(testVirtioBlkID).
				ExpectDeleteChardev(testVirtioBlkID).WithErrorResponse(),
		},
		"spdk failed to delete virtio-blk": {
			jsonRPC:     alwaysFailingJSONRPC,
			expectError: errDevicePartiallyDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlkWithEvent(testVirtioBlkID).
				ExpectDeleteChardev(testVirtioBlkID),
		},
		"all qemu and spdk calls failed": {
			jsonRPC:     alwaysFailingJSONRPC,
			expectError: errDeviceNotDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteVirtioBlk(testVirtioBlkID).WithErrorResponse().
				ExpectDeleteChardev(testVirtioBlkID).WithErrorResponse(),
		},
		"failed to create monitor": {
			nonDefaultQmpAddress: "/dev/null",
			jsonRPC:              alwaysSuccessfulJSONRPC,
			expectError:          errMonitorCreation,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opiSpdkServer := frontend.NewServer(test.jsonRPC)
			opiSpdkServer.Virt.BlkCtrls[testVirtioBlkID] = testCreateVirtioBlkRequest.VirtioBlk
			qmpServer := startMockQmpServer(t, test.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if test.nonDefaultQmpAddress != "" {
				qmpAddress = test.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir, nil, nil)
			kvmServer.timeout = qmplibTimeout

			_, err := kvmServer.DeleteVirtioBlk(context.Background(), testDeleteVirtioBlkRequest)
			if !errors.Is(err, test.expectError) {
				t.Errorf("Expected %v, got %v", test.expectError, err)
			}
			if !qmpServer.WereExpectedCallsPerformed() {
				t.Errorf("Not all expected calls were performed")
			}
		})
	}
}
