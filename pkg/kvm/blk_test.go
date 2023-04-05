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

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"github.com/ulule/deepcopier"
	"google.golang.org/protobuf/proto"
)

func TestCreateVirtioBlk(t *testing.T) {
	expectNotNilOut := &pb.VirtioBlk{}
	if deepcopier.Copy(testCreateVirtioBlkRequest.VirtioBlk).To(expectNotNilOut) != nil {
		log.Panicf("Failed to copy structure")
	}

	tests := map[string]struct {
		expectAddChardev      bool
		expectAddChardevError bool

		expectAddVirtioBlk      bool
		expectAddVirtioBlkError bool

		expectQueryPci bool

		expectDeleteChardev bool

		jsonRPC              server.JSONRPC
		expectError          error
		nonDefaultQmpAddress string

		out *pb.VirtioBlk
	}{
		"valid virtio-blk creation": {
			expectAddChardev:   true,
			expectAddVirtioBlk: true,
			expectQueryPci:     true,
			jsonRPC:            alwaysSuccessfulJSONRPC,
			out:                expectNotNilOut,
		},
		"spdk failed to create virtio-blk": {
			jsonRPC:     alwaysFailingJSONRPC,
			expectError: server.ErrFailedSpdkCall,
		},
		"qemu chardev add failed": {
			expectAddChardevError: true,
			jsonRPC:               alwaysSuccessfulJSONRPC,
			expectError:           errAddChardevFailed,
		},
		"qemu device add failed": {
			expectAddChardev:        true,
			expectAddVirtioBlkError: true,
			expectDeleteChardev:     true,
			jsonRPC:                 alwaysSuccessfulJSONRPC,
			expectError:             errAddDeviceFailed,
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
			qmpServer := startMockQmpServer(t)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if test.nonDefaultQmpAddress != "" {
				qmpAddress = test.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir)
			kvmServer.timeout = qmplibTimeout

			if test.expectAddChardev {
				qmpServer.ExpectAddChardev(testVirtioBlkID)
			}
			if test.expectAddChardevError {
				qmpServer.ExpectAddChardev(testVirtioBlkID).WithErrorResponse()
			}
			if test.expectAddVirtioBlk {
				qmpServer.ExpectAddVirtioBlk(testVirtioBlkID, testVirtioBlkID)
			}
			if test.expectAddVirtioBlkError {
				qmpServer.ExpectAddVirtioBlk(testVirtioBlkID, testVirtioBlkID).WithErrorResponse()
			}
			if test.expectQueryPci {
				qmpServer.ExpectQueryPci(testVirtioBlkID)
			}
			if test.expectDeleteChardev {
				qmpServer.ExpectDeleteChardev(testVirtioBlkID)
			}

			out, err := kvmServer.CreateVirtioBlk(context.Background(), testCreateVirtioBlkRequest)
			if !errors.Is(err, test.expectError) {
				t.Errorf("Expected error %v, got %v", test.expectError, err)
			}
			gotOut, _ := proto.Marshal(out)
			wantOut, _ := proto.Marshal(test.out)
			if !bytes.Equal(gotOut, wantOut) {
				t.Errorf("Expected out %v, got %v", &test.out, out)
			}
			if !qmpServer.WereExpectedCallsPerformed() {
				t.Errorf("Not all expected calls were performed")
			}
		})
	}
}

func TestDeleteVirtioBlk(t *testing.T) {
	tests := map[string]struct {
		expectDeleteVirtioBlk          bool
		expectDeleteVirtioBlkWithEvent bool
		expectDeleteVirtioBlkError     bool

		expectDeleteChardev      bool
		expectDeleteChardevError bool

		jsonRPC              server.JSONRPC
		expectError          error
		nonDefaultQmpAddress string
	}{
		"valid virtio-blk deletion": {
			expectDeleteVirtioBlkWithEvent: true,
			expectDeleteChardev:            true,
			jsonRPC:                        alwaysSuccessfulJSONRPC,
		},
		"qemu device delete failed": {
			expectDeleteVirtioBlkError: true,
			expectDeleteChardev:        true,
			jsonRPC:                    alwaysSuccessfulJSONRPC,
			expectError:                errDevicePartiallyDeleted,
		},
		"qemu device delete failed by timeout": {
			expectDeleteVirtioBlk: true,
			expectDeleteChardev:   true,
			jsonRPC:               alwaysSuccessfulJSONRPC,
			expectError:           errDevicePartiallyDeleted,
		},
		"qemu chardev delete failed": {
			expectDeleteVirtioBlkWithEvent: true,
			expectDeleteChardevError:       true,
			jsonRPC:                        alwaysSuccessfulJSONRPC,
			expectError:                    errDevicePartiallyDeleted,
		},
		"spdk failed to delete virtio-blk": {
			expectDeleteVirtioBlkWithEvent: true,
			expectDeleteChardev:            true,
			jsonRPC:                        alwaysFailingJSONRPC,
			expectError:                    errDevicePartiallyDeleted,
		},
		"all qemu and spdk calls failed": {
			expectDeleteVirtioBlkError: true,
			expectDeleteChardevError:   true,
			jsonRPC:                    alwaysFailingJSONRPC,
			expectError:                errDeviceNotDeleted,
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
			qmpServer := startMockQmpServer(t)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if test.nonDefaultQmpAddress != "" {
				qmpAddress = test.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir)
			kvmServer.timeout = qmplibTimeout

			if test.expectDeleteVirtioBlkWithEvent {
				qmpServer.ExpectDeleteVirtioBlkWithEvent(testVirtioBlkID)
			}
			if test.expectDeleteVirtioBlk {
				qmpServer.ExpectDeleteVirtioBlk(testVirtioBlkID)
			}
			if test.expectDeleteVirtioBlkError {
				qmpServer.ExpectDeleteVirtioBlk(testVirtioBlkID).WithErrorResponse()
			}
			if test.expectDeleteChardev {
				qmpServer.ExpectDeleteChardev(testVirtioBlkID)
			}
			if test.expectDeleteChardevError {
				qmpServer.ExpectDeleteChardev(testVirtioBlkID).WithErrorResponse()
			}

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
