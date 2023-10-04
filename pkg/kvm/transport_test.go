// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
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
					t.Errorf("NewNvmeVfiouserTransport() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			gotTransport := NewNvmeVfiouserTransport(tt.ctrlrDir)
			wantTransport := &nvmeVfiouserTransport{
				ctrlrDir: tt.ctrlrDir,
			}

			if !reflect.DeepEqual(gotTransport, wantTransport) {
				t.Errorf("Received transport %v not equal to expected one %v", gotTransport, wantTransport)
			}
		})
	}
}

func TestNewNvmeVfiouserTransportParams(t *testing.T) {
	tmpDir := t.TempDir()
	tests := map[string]struct {
		pf         int32
		vf         int32
		port       int32
		wantErr    bool
		wantParams spdk.NvmfSubsystemAddListenerParams
	}{
		"not allowed vf": {
			pf:         0,
			vf:         1,
			port:       0,
			wantErr:    true,
			wantParams: spdk.NvmfSubsystemAddListenerParams{},
		},
		"not allowed port": {
			pf:         0,
			vf:         0,
			port:       2,
			wantErr:    true,
			wantParams: spdk.NvmfSubsystemAddListenerParams{},
		},
		"successful params": {
			pf:      3,
			vf:      0,
			port:    0,
			wantErr: false,
			wantParams: spdk.NvmfSubsystemAddListenerParams{
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
			vfiouserTransport := NewNvmeVfiouserTransport(tmpDir)
			gotParams, err := vfiouserTransport.Params(&pb.NvmeController{
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
					Nqn: "nqn.2014-08.org.nvmexpress:uuid:1630a3a6-5bac-4563-a1a6-d2b0257c282a",
				},
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("Expect error: %v, received: %v", nil, err)
			}
			if !reflect.DeepEqual(tt.wantParams, gotParams) {
				t.Errorf("Expect %v, received %v", tt.wantParams, gotParams)
			}
		})
	}
}
