// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"
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
	wantParams := models.NvmfSubsystemAddListenerParams{}
	wantParams.Nqn = "nqn.2014-08.org.nvmexpress:uuid:1630a3a6-5bac-4563-a1a6-d2b0257c282a"
	wantParams.ListenAddress.Trtype = "vfiouser"
	wantParams.ListenAddress.Traddr = filepath.Join(tmpDir, "nvme-1")

	vfiouserSubsysListener := NewVfiouserSubsystemListener(tmpDir)
	gotParams := vfiouserSubsysListener.Params(&pb.NVMeController{
		Spec: &pb.NVMeControllerSpec{
			Id: &pc.ObjectKey{
				Value: "nvme-1",
			},
		},
	}, "nqn.2014-08.org.nvmexpress:uuid:1630a3a6-5bac-4563-a1a6-d2b0257c282a")

	if !reflect.DeepEqual(wantParams, gotParams) {
		t.Errorf("Expect %v, received %v", wantParams, gotParams)
	}
}
