// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation
// Copyright (C) 2024 Dell Inc, or its subsidiaries.

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/philippgille/gokv/gomap"
	"github.com/spdk/spdk/go/rpc/client"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	testNvmeControllerID   = "nvme-43"
	testNvmeControllerName = utils.ResourceIDToControllerName(testSubsystemID, "nvme-43")
	testSubsystemID        = "subsystem0"
	testSubsystemName      = utils.ResourceIDToSubsystemName("subsystem0")
	testSubsystem          = pb.NvmeSubsystem{
		Name: testSubsystemName,
		Spec: &pb.NvmeSubsystemSpec{
			Nqn: "nqn.2022-09.io.spdk:opi2",
		},
	}
	testCreateNvmeControllerRequest = &pb.CreateNvmeControllerRequest{
		Parent:           testSubsystemName,
		NvmeControllerId: testNvmeControllerID,
		NvmeController: &pb.NvmeController{
			Spec: &pb.NvmeControllerSpec{
				Endpoint: &pb.NvmeControllerSpec_PcieId{
					PcieId: &pb.PciEndpoint{
						PhysicalFunction: wrapperspb.Int32(43),
						VirtualFunction:  wrapperspb.Int32(0),
						PortId:           wrapperspb.Int32(0),
					},
				},
				Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE,
				NvmeControllerId: proto.Int32(43),
			},
			Status: &pb.NvmeControllerStatus{
				Active: true,
			},
		}}
	testDeleteNvmeControllerRequest = &pb.DeleteNvmeControllerRequest{Name: testNvmeControllerName}
)

func dirExists(dirname string) bool {
	fi, err := os.Stat(dirname)
	return err == nil && fi.IsDir()
}

func TestCreateNvmeController(t *testing.T) {
	expectNotNilOut := utils.ProtoClone(testCreateNvmeControllerRequest.NvmeController)
	expectNotNilOut.Spec.NvmeControllerId = proto.Int32(-1)
	expectNotNilOut.Name = testNvmeControllerName
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(expectNotNilOut)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		jsonRPC                       client.IClient
		nonDefaultQmpAddress          string
		ctrlrDirExistsBeforeOperation bool
		ctrlrDirExistsAfterOperation  bool
		buses                         []string

		in      *pb.CreateNvmeControllerRequest
		out     *pb.NvmeController
		errCode codes.Code
		errMsg  string

		mockQmpCalls *mockQmpCalls
	}{
		"valid Nvme controller creation": {
			in:                            testCreateNvmeControllerRequest,
			out:                           expectNotNilOut,
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  true,
			errCode:                       codes.OK,
			errMsg:                        "",
			mockQmpCalls: newMockQmpCalls().
				ExpectAddNvmeController(testNvmeControllerID, testSubsystemID).
				ExpectQueryPci(testNvmeControllerID),
		},
		"spdk failed to create Nvme controller": {
			in:                            testCreateNvmeControllerRequest,
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			errCode:                       codes.Unknown,
			errMsg:                        "nvmf_subsystem_add_listener: stub error",
		},
		"qemu Nvme controller add failed": {
			in:                            testCreateNvmeControllerRequest,
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			errCode:                       status.Convert(errAddDeviceFailed).Code(),
			errMsg:                        status.Convert(errAddDeviceFailed).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectAddNvmeController(testNvmeControllerID, testSubsystemID).WithErrorResponse(),
		},
		"failed to create monitor": {
			in:                            testCreateNvmeControllerRequest,
			nonDefaultQmpAddress:          "/dev/null",
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			errCode:                       status.Convert(errMonitorCreation).Code(),
			errMsg:                        status.Convert(errMonitorCreation).Message(),
		},
		"Ctrlr dir already exists": {
			in:                            testCreateNvmeControllerRequest,
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			errCode:                       status.Convert(errFailedToCreateNvmeDir).Code(),
			errMsg:                        status.Convert(errFailedToCreateNvmeDir).Message(),
		},
		"empty subsystem in request": {
			in: &pb.CreateNvmeControllerRequest{
				Parent: "",
				NvmeController: &pb.NvmeController{
					Spec: &pb.NvmeControllerSpec{
						Endpoint: &pb.NvmeControllerSpec_PcieId{
							PcieId: &pb.PciEndpoint{
								PhysicalFunction: wrapperspb.Int32(1),
								VirtualFunction:  wrapperspb.Int32(0),
								PortId:           wrapperspb.Int32(0),
							},
						},
						NvmeControllerId: proto.Int32(43),
						Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				}, NvmeControllerId: testNvmeControllerID},
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			errCode:                       status.Convert(errInvalidSubsystem).Code(),
			errMsg:                        status.Convert(errInvalidSubsystem).Message(),
		},
		"valid Nvme creation with on first bus location": {
			in: &pb.CreateNvmeControllerRequest{
				Parent: testSubsystemName,
				NvmeController: &pb.NvmeController{
					Spec: &pb.NvmeControllerSpec{
						Endpoint: &pb.NvmeControllerSpec_PcieId{
							PcieId: &pb.PciEndpoint{
								PhysicalFunction: wrapperspb.Int32(1),
								VirtualFunction:  wrapperspb.Int32(0),
								PortId:           wrapperspb.Int32(0)}},
						NvmeControllerId: proto.Int32(43),
						Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				}, NvmeControllerId: testNvmeControllerID},
			out: &pb.NvmeController{
				Name: testNvmeControllerName,
				Spec: &pb.NvmeControllerSpec{
					Endpoint: &pb.NvmeControllerSpec_PcieId{
						PcieId: &pb.PciEndpoint{
							PhysicalFunction: wrapperspb.Int32(1),
							VirtualFunction:  wrapperspb.Int32(0),
							PortId:           wrapperspb.Int32(0)}},
					NvmeControllerId: proto.Int32(-1),
					Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE,
				},
				Status: &pb.NvmeControllerStatus{
					Active: true,
				},
			},
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  true,
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			buses:                         []string{"pci.opi.0", "pci.opi.1"},
			mockQmpCalls: newMockQmpCalls().
				ExpectAddNvmeControllerWithAddress(testNvmeControllerID, testSubsystemID, "pci.opi.0", 1).
				ExpectQueryPci(testNvmeControllerID),
		},
		"valid Nvme creation with on second bus location": {
			in:                            testCreateNvmeControllerRequest,
			out:                           expectNotNilOut,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  true,
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			buses:                         []string{"pci.opi.0", "pci.opi.1"},
			mockQmpCalls: newMockQmpCalls().
				ExpectAddNvmeControllerWithAddress(testNvmeControllerID, testSubsystemID, "pci.opi.1", 11).
				ExpectQueryPci(testNvmeControllerID),
		},
		"Nvme creation with physical function goes out of buses": {
			in:      testCreateNvmeControllerRequest,
			out:     nil,
			errCode: status.Convert(errDeviceEndpoint).Code(),
			errMsg:  status.Convert(errDeviceEndpoint).Message(),
			jsonRPC: alwaysSuccessfulJSONRPC,
			buses:   []string{"pci.opi.0"},
		},
		"negative physical function": {
			in: &pb.CreateNvmeControllerRequest{
				Parent: testSubsystemName,
				NvmeController: &pb.NvmeController{
					Spec: &pb.NvmeControllerSpec{
						Endpoint: &pb.NvmeControllerSpec_PcieId{
							PcieId: &pb.PciEndpoint{
								PhysicalFunction: wrapperspb.Int32(-1),
								VirtualFunction:  wrapperspb.Int32(0),
								PortId:           wrapperspb.Int32(0),
							},
						},
						NvmeControllerId: proto.Int32(43),
						Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				}, NvmeControllerId: testNvmeControllerID},
			out:     nil,
			errCode: status.Convert(errDeviceEndpoint).Code(),
			errMsg:  status.Convert(errDeviceEndpoint).Message(),
			jsonRPC: alwaysSuccessfulJSONRPC,
			buses:   []string{"pci.opi.0"},
		},
		"nil path": {
			in: &pb.CreateNvmeControllerRequest{
				Parent: testSubsystemName,
				NvmeController: &pb.NvmeController{
					Spec: &pb.NvmeControllerSpec{
						Endpoint:         nil,
						NvmeControllerId: proto.Int32(43),
						Trtype:           pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE,
					},
					Status: &pb.NvmeControllerStatus{
						Active: true,
					},
				}, NvmeControllerId: testNvmeControllerID},
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
			qmpServer := startMockQmpServer(t, tt.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if tt.nonDefaultQmpAddress != "" {
				qmpAddress = tt.nonDefaultQmpAddress
			}
			opiSpdkServer := frontend.NewCustomizedServer(tt.jsonRPC, store,
				map[pb.NvmeTransportType]frontend.NvmeTransport{
					pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE: NewNvmeVfiouserTransport(qmpServer.testDir, tt.jsonRPC),
				}, frontend.NewVhostUserBlkTransport())
			opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir, tt.buses)
			kvmServer.timeout = qmplibTimeout
			testCtrlrDir := controllerDirPath(qmpServer.testDir, testSubsystemID)
			if tt.ctrlrDirExistsBeforeOperation &&
				os.Mkdir(testCtrlrDir, os.ModePerm) != nil {
				log.Panicf("Couldn't create ctrlr dir for test")
			}
			request := utils.ProtoClone(tt.in)

			out, err := kvmServer.CreateNvmeController(context.Background(), request)

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
			ctrlrDirExists := dirExists(testCtrlrDir)
			if tt.ctrlrDirExistsAfterOperation != ctrlrDirExists {
				t.Errorf("Expect controller dir exists %v, got %v", tt.ctrlrDirExistsAfterOperation, ctrlrDirExists)
			}
		})
	}
}

func TestDeleteNvmeController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		jsonRPC              client.IClient
		nonDefaultQmpAddress string

		ctrlrDirExistsBeforeOperation bool
		ctrlrDirExistsAfterOperation  bool
		nonEmptyCtrlrDirAfterSpdkCall bool
		noController                  bool
		errCode                       codes.Code
		errMsg                        string

		mockQmpCalls *mockQmpCalls
	}{
		"valid Nvme controller deletion": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			errCode:                       codes.OK,
			errMsg:                        "",
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).
				ExpectNoDeviceQueryPci(),
		},
		"qemu Nvme controller delete failed": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			errCode:                       status.Convert(errDevicePartiallyDeleted).Code(),
			errMsg:                        status.Convert(errDevicePartiallyDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).WithErrorResponse(),
		},
		"spdk failed to delete Nvme controller": {
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			errCode:                       status.Convert(errDevicePartiallyDeleted).Code(),
			errMsg:                        status.Convert(errDevicePartiallyDeleted).Message(),
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
			errCode:                       status.Convert(errMonitorCreation).Code(),
			errMsg:                        status.Convert(errMonitorCreation).Message(),
		},
		"ctrlr dir is not empty after SPDK call": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			nonEmptyCtrlrDirAfterSpdkCall: true,
			errCode:                       status.Convert(errDevicePartiallyDeleted).Code(),
			errMsg:                        status.Convert(errDevicePartiallyDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).
				ExpectNoDeviceQueryPci(),
		},
		"ctrlr dir does not exist": {
			jsonRPC:                       alwaysSuccessfulJSONRPC,
			ctrlrDirExistsBeforeOperation: false,
			ctrlrDirExistsAfterOperation:  false,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			errCode:                       codes.OK,
			errMsg:                        "",
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).
				ExpectNoDeviceQueryPci(),
		},
		"all communication operations failed": {
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			nonEmptyCtrlrDirAfterSpdkCall: true,
			errCode:                       status.Convert(errDeviceNotDeleted).Code(),
			errMsg:                        status.Convert(errDeviceNotDeleted).Message(),
			mockQmpCalls: newMockQmpCalls().
				ExpectDeleteNvmeController(testNvmeControllerID).WithErrorResponse(),
		},
		"no controller found": {
			jsonRPC:                       alwaysFailingJSONRPC,
			ctrlrDirExistsBeforeOperation: true,
			ctrlrDirExistsAfterOperation:  true,
			nonEmptyCtrlrDirAfterSpdkCall: false,
			noController:                  true,
			errCode:                       codes.NotFound,
			errMsg:                        fmt.Sprintf("unable to find key %s", testNvmeControllerName),
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			options := gomap.DefaultOptions
			options.Codec = utils.ProtoCodec{}
			store := gomap.NewStore(options)
			qmpServer := startMockQmpServer(t, tt.mockQmpCalls)
			defer qmpServer.Stop()
			qmpAddress := qmpServer.socketPath
			if tt.nonDefaultQmpAddress != "" {
				qmpAddress = tt.nonDefaultQmpAddress
			}
			opiSpdkServer := frontend.NewCustomizedServer(tt.jsonRPC, store,
				map[pb.NvmeTransportType]frontend.NvmeTransport{
					pb.NvmeTransportType_NVME_TRANSPORT_TYPE_PCIE: NewNvmeVfiouserTransport(qmpServer.testDir, tt.jsonRPC),
				}, frontend.NewVhostUserBlkTransport())
			opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			if !tt.noController {
				opiSpdkServer.Nvme.Controllers[testNvmeControllerName] =
					utils.ProtoClone(testCreateNvmeControllerRequest.NvmeController)
				opiSpdkServer.Nvme.Controllers[testNvmeControllerName].Name = testNvmeControllerName
			}
			kvmServer := NewServer(opiSpdkServer, qmpAddress, qmpServer.testDir, nil)
			kvmServer.timeout = qmplibTimeout
			testCtrlrDir := controllerDirPath(qmpServer.testDir, testSubsystemID)
			if tt.ctrlrDirExistsBeforeOperation {
				if err := os.Mkdir(testCtrlrDir, os.ModePerm); err != nil {
					log.Panic(err)
				}

				if tt.nonEmptyCtrlrDirAfterSpdkCall {
					if err := os.Mkdir(filepath.Join(testCtrlrDir, "ctrlr"), os.ModeDir); err != nil {
						log.Panic(err)
					}
				}
			}
			request := utils.ProtoClone(testDeleteNvmeControllerRequest)

			_, err := kvmServer.DeleteNvmeController(context.Background(), request)

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
			ctrlrDirExists := dirExists(testCtrlrDir)
			if ctrlrDirExists != tt.ctrlrDirExistsAfterOperation {
				t.Errorf("Expect controller dir exists %v, got %v",
					tt.ctrlrDirExistsAfterOperation, ctrlrDirExists)
			}
		})
	}
}
