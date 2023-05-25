// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"google.golang.org/protobuf/proto"
)

var (
	testNvmeControllerID = "nvme-43"
	testSubsystemID      = "subsystem0"
	testSubsystem        = pb.NVMeSubsystem{
		Spec: &pb.NVMeSubsystemSpec{
			Name: testSubsystemID,
			Nqn:  "nqn.2022-09.io.spdk:opi2",
		},
	}
	testCreateNvmeControllerRequest = &pb.CreateNVMeControllerRequest{NvMeControllerId: testNvmeControllerID, NvMeController: &pb.NVMeController{
		Spec: &pb.NVMeControllerSpec{
			Name:             testNvmeControllerID,
			SubsystemId:      &pc.ObjectKey{Value: testSubsystem.Spec.Name},
			PcieId:           &pb.PciEndpoint{PhysicalFunction: 0, VirtualFunction: 5},
			NvmeControllerId: 43,
		},
		Status: &pb.NVMeControllerStatus{
			Active: true,
		},
	}}
	testDeleteNvmeControllerRequest = &pb.DeleteNVMeControllerRequest{Name: testNvmeControllerID}
)

func TestNewVfiouserSubsystemListener(t *testing.T) {
	tests := map[string]struct {
		ctrlrDir  string
		wantPanic bool
	}{
		"valid controller dir": {
			ctrlrDir:  ".",
			wantPanic: false,
		},
		"empty string for controller dir": {
			ctrlrDir:  "",
			wantPanic: true,
		},
		"non existing path": {
			ctrlrDir:  "this/is/some/non/existing/path",
			wantPanic: true,
		},
		"ctrlrDir points to non-directory": {
			ctrlrDir:  "/dev/null",
			wantPanic: true,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("NewVfiouserSubsystemListener() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			gotSubsysListener := NewVfiouserSubsystemListener(tt.ctrlrDir)
			wantSubsysListener := &vfiouserSubsystemListener{
				ctrlrDir: tt.ctrlrDir,
			}

			if !reflect.DeepEqual(gotSubsysListener, wantSubsysListener) {
				t.Errorf("Received subsystem listern %v not equal to expected one %v", gotSubsysListener, wantSubsysListener)
			}
		})
	}
}

func TestNewVfiouserSubsystemListenerParams(t *testing.T) {
	tmpDir := os.TempDir()
	wantParams := spdk.NvmfSubsystemAddListenerParams{}
	wantParams.Nqn = "nqn.2014-08.org.nvmexpress:uuid:1630a3a6-5bac-4563-a1a6-d2b0257c282a"
	wantParams.ListenAddress.Trtype = "vfiouser"
	wantParams.ListenAddress.Traddr = filepath.Join(tmpDir, "nvme-1")

	vfiouserSubsysListener := NewVfiouserSubsystemListener(tmpDir)
	gotParams := vfiouserSubsysListener.Params(&pb.NVMeController{
		Spec: &pb.NVMeControllerSpec{
			SubsystemId: &pc.ObjectKey{Value: "nvme-1"},
		},
	}, "nqn.2014-08.org.nvmexpress:uuid:1630a3a6-5bac-4563-a1a6-d2b0257c282a")

	if !reflect.DeepEqual(wantParams, gotParams) {
		t.Errorf("Expect %v, received %v", wantParams, gotParams)
	}
}

func dirExists(dirname string) bool {
	fi, err := os.Stat(dirname)
	return err == nil && fi.IsDir()
}

func TestCreateNvmeController(t *testing.T) {
	expectNotNilOut := proto.Clone(testCreateNvmeControllerRequest.NvMeController).(*pb.NVMeController)
	expectNotNilOut.Spec.NvmeControllerId = -1

	tests := map[string]struct {
		jsonRPC                       spdk.JSONRPC
		nonDefaultQmpAddress          string
		ctrlrDirExistsBeforeOperation bool
		ctrlrDirExistsAfterOperation  bool
		emptySubsystem                bool

		out         *pb.NVMeController
		expectError error

		mockQmpCalls *mockQmpCalls
	}{
		"valid NVMe controller creation": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  true,
			out:                           expectNotNilOut,
			expectError:                   nil,
			mockQmpCalls: newMockQmpCalls().
				ExpectAddNvmeController(testNvmeControllerID, testSubsystemID).
				ExpectQueryPci(testNvmeControllerID),
		},
		"spdk failed to create NVMe controller": {
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			expectError:                   errStub,
		},
		"qemu NVMe controller add failed": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			expectError:                   errAddDeviceFailed,
			mockQmpCalls: newMockQmpCalls().
				ExpectAddNvmeController(testNvmeControllerID, testSubsystemID).WithErrorResponse(),
		},
		"failed to create monitor": {
			nonDefaultQmpAddress:          "/dev/null",
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			expectError:                   errMonitorCreation,
		},
		"Ctrlr dir already exists": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			expectError:                   errFailedToCreateNvmeDir,
		},
		"empty subsystem in request": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			emptySubsystem:                true,
			expectError:                   errInvalidSubsystem,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opiSpdkServer := frontend.NewServer(test.jsonRPC)
			opiSpdkServer.Nvme.Subsystems[testSubsystem.Spec.Name] = &testSubsystem
			qmpServer := startMockQmpServer(t, test.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if test.nonDefaultQmpAddress != "" {
				qmpAddress = test.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir)
			kvmServer.timeout = qmplibTimeout
			testCtrlrDir := controllerDirPath(qmpServer.testDir, testSubsystemID)
			if test.ctrlrDirExistsBeforeOperation &&
				os.Mkdir(testCtrlrDir, os.ModePerm) != nil {
				log.Panicf("Couldn't create ctrlr dir for test")
			}

			request := proto.Clone(testCreateNvmeControllerRequest).(*pb.CreateNVMeControllerRequest)
			if test.emptySubsystem {
				request.NvMeController.Spec.SubsystemId.Value = ""
			}

			out, err := kvmServer.CreateNVMeController(context.Background(), request)
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
			ctrlrDirExists := dirExists(testCtrlrDir)
			if test.ctrlrDirExistsAfterOperation != ctrlrDirExists {
				t.Errorf("Expect controller dir exists %v, got %v", test.ctrlrDirExistsAfterOperation, ctrlrDirExists)
			}
		})
	}
}

func TestDeleteNvmeController(t *testing.T) {
	tests := map[string]struct {
		jsonRPC              spdk.JSONRPC
		nonDefaultQmpAddress string

		ctrlrDirExistsBeforeOperation bool
		ctrlrDirExistsAfterOperation  bool
		nonEmptyCtrlrDirAfterSpdkCall bool
		noController                  bool
		expectError                   error

		mockQmpCalls *mockQmpCalls
	}{
		"valid NVMe controller deletion": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			expectError:                   nil,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).
				ExpectNoDeviceQueryPci(),
		},
		"qemu NVMe controller delete failed": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			expectError:                   errDevicePartiallyDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).WithErrorResponse(),
		},
		"spdk failed to delete NVMe controller": {
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			expectError:                   errDevicePartiallyDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).
				ExpectNoDeviceQueryPci(),
		},
		"failed to create monitor": {
			nonDefaultQmpAddress:          "/dev/null",
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			expectError:                   errMonitorCreation,
		},
		"ctrlr dir is not empty after SPDK call": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			nonEmptyCtrlrDirAfterSpdkCall: true,
			expectError:                   errDevicePartiallyDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).
				ExpectNoDeviceQueryPci(),
		},
		"ctrlr dir does not exist": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			expectError:                   nil,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).
				ExpectNoDeviceQueryPci(),
		},
		"all communication operations failed": {
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			nonEmptyCtrlrDirAfterSpdkCall: true,
			expectError:                   errDeviceNotDeleted,
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).WithErrorResponse(),
		},
		"no controller found": {
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			noController:                  true,
			expectError:                   errNoController,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opiSpdkServer := frontend.NewServer(test.jsonRPC)
			opiSpdkServer.Nvme.Subsystems[testSubsystem.Spec.Name] = &testSubsystem
			if !test.noController {
				opiSpdkServer.Nvme.Controllers[testCreateNvmeControllerRequest.NvMeController.Spec.Name] =
					testCreateNvmeControllerRequest.NvMeController
			}
			qmpServer := startMockQmpServer(t, test.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if test.nonDefaultQmpAddress != "" {
				qmpAddress = test.nonDefaultQmpAddress
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir)
			kvmServer.timeout = qmplibTimeout
			testCtrlrDir := controllerDirPath(qmpServer.testDir, testSubsystemID)
			if test.ctrlrDirExistsBeforeOperation {
				if err := os.Mkdir(testCtrlrDir, os.ModePerm); err != nil {
					log.Panic(err)
				}

				if test.nonEmptyCtrlrDirAfterSpdkCall {
					if err := os.Mkdir(filepath.Join(testCtrlrDir, "ctrlr"), os.ModeDir); err != nil {
						log.Panic(err)
					}
				}
			}

			_, err := kvmServer.DeleteNVMeController(context.Background(), testDeleteNvmeControllerRequest)
			if !errors.Is(err, test.expectError) {
				t.Errorf("Expected error %v, got %v", test.expectError, err)
			}
			if !qmpServer.WereExpectedCallsPerformed() {
				t.Errorf("Not all expected calls were performed")
			}
			ctrlrDirExists := dirExists(testCtrlrDir)
			if ctrlrDirExists != test.ctrlrDirExistsAfterOperation {
				t.Errorf("Expect controller dir exists %v, got %v",
					test.ctrlrDirExistsAfterOperation, ctrlrDirExists)
			}
		})
	}
}
