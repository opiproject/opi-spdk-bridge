// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestNewNvmeVfiouserTransport(t *testing.T) {
	tests := map[string]struct {
		ctrlrDir  string
		rpc       spdk.JSONRPC
		wantPanic bool
	}{
		"valid controller dir": {
			ctrlrDir:  ".",
			rpc:       &stubJSONRRPC{},
			wantPanic: false,
		},
		"empty string for controller dir": {
			ctrlrDir:  "",
			rpc:       &stubJSONRRPC{},
			wantPanic: true,
		},
		"non existing path": {
			ctrlrDir:  "this/is/some/non/existing/path",
			rpc:       &stubJSONRRPC{},
			wantPanic: true,
		},
		"ctrlrDir points to non-directory": {
			ctrlrDir:  "/dev/null",
			rpc:       &stubJSONRRPC{},
			wantPanic: true,
		},
		"nil json rpc": {
			ctrlrDir:  ".",
			rpc:       nil,
			wantPanic: true,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("NewNvmeVfiouserTransport() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			gotTransport := NewNvmeVfiouserTransport(tt.ctrlrDir, tt.rpc)
			wantTransport := &nvmeVfiouserTransport{
				ctrlrDir: tt.ctrlrDir,
				rpc:      tt.rpc,
			}

			if !reflect.DeepEqual(gotTransport, wantTransport) {
				t.Errorf("Received transport %v not equal to expected one %v", gotTransport, wantTransport)
			}
		})
	}
}

func TestNvmeVfiouserTransportCreateController(t *testing.T) {
	tmpDir := t.TempDir()
	tests := map[string]struct {
		pf         int32
		vf         int32
		port       int32
		hostnqn    string
		wantErr    bool
		wantParams any
	}{
		"not allowed vf": {
			pf:         0,
			vf:         1,
			port:       0,
			hostnqn:    "",
			wantErr:    true,
			wantParams: nil,
		},
		"not allowed port": {
			pf:         0,
			vf:         0,
			port:       2,
			hostnqn:    "",
			wantErr:    true,
			wantParams: nil,
		},
		"not allowed hostnqn in subsystem": {
			pf:         0,
			vf:         0,
			port:       0,
			hostnqn:    "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
			wantErr:    true,
			wantParams: nil,
		},
		"successful params": {
			pf:      3,
			vf:      0,
			port:    0,
			hostnqn: "",
			wantErr: false,
			wantParams: &spdk.NvmfSubsystemAddListenerParams{
				Nqn: "nqn.2014-08.org.nvmexpress:uuid:1630a3a6-5bac-4563-a1a6-d2b0257c282a",
				ListenAddress: struct {
					Trtype  string "json:\"trtype\""
					Traddr  string "json:\"traddr\""
					Trsvcid string "json:\"trsvcid,omitempty\""
					Adrfam  string "json:\"adrfam,omitempty\""
				}{
					Trtype:  "vfiouser",
					Traddr:  filepath.Join(tmpDir, "subsys0"),
					Trsvcid: "",
					Adrfam:  "",
				},
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			rpc := &stubJSONRRPC{}
			vfiouserTransport := NewNvmeVfiouserTransport(tmpDir, rpc)
			err := vfiouserTransport.CreateController(
				context.Background(),
				&pb.NvmeController{
					Name: utils.ResourceIDToControllerName("subsys0", "nvme-1"),
					Spec: &pb.NvmeControllerSpec{
						Endpoint: &pb.NvmeControllerSpec_PcieId{
							PcieId: &pb.PciEndpoint{
								PortId:           wrapperspb.Int32(tt.port),
								PhysicalFunction: wrapperspb.Int32(tt.pf),
								VirtualFunction:  wrapperspb.Int32(tt.vf),
							},
						},
						Trtype: pb.NvmeTransportType_NVME_TRANSPORT_PCIE,
					},
				}, &pb.NvmeSubsystem{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:     "nqn.2014-08.org.nvmexpress:uuid:1630a3a6-5bac-4563-a1a6-d2b0257c282a",
						Hostnqn: tt.hostnqn,
					},
				})

			if (err != nil) != tt.wantErr {
				t.Errorf("Expect error: %v, received: %v", nil, err)
			}
			if !reflect.DeepEqual(tt.wantParams, rpc.arg) {
				t.Errorf("Expect %v, received %v", tt.wantParams, rpc.arg)
			}
		})
	}
}
