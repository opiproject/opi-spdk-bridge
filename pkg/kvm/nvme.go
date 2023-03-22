// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"log"
	"os"
	"path/filepath"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"
)

type vfiouserSubsystemListener struct {
	ctrlrDir string
}

// NewVfiouserSubsystemListener creates a new instance of vfiouserSubsystemListener
func NewVfiouserSubsystemListener(ctrlrDir string) frontend.SubsystemListener {
	if ctrlrDir == "" {
		log.Panicf("ctrlrDir cannot be empty")
	}

	dir, err := os.Stat(ctrlrDir)
	if err != nil {
		log.Panicf("%v path cannot be evaluated", ctrlrDir)
	}
	if !dir.IsDir() {
		log.Panicf("%v is not a directory", ctrlrDir)
	}

	return &vfiouserSubsystemListener{
		ctrlrDir: ctrlrDir,
	}
}

func (c *vfiouserSubsystemListener) Params(ctrlr *pb.NVMeController, nqn string) models.NvmfSubsystemAddListenerParams {
	result := models.NvmfSubsystemAddListenerParams{}
	ctrlrDirPath := controllerDirPath(c.ctrlrDir, ctrlr.Spec.Id.Value)
	result.Nqn = nqn
	result.ListenAddress.Trtype = "vfiouser"
	result.ListenAddress.Traddr = ctrlrDirPath

	return result
}

func controllerDirPath(ctrlrDir string, ctrlrID string) string {
	return filepath.Join(ctrlrDir, ctrlrID)
}
