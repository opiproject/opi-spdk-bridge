// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"fmt"
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
	ctrlrDirPath := c.controllerDirPath(ctrlr)
	result.Nqn = nqn
	result.ListenAddress.Trtype = "vfiouser"
	result.ListenAddress.Traddr = ctrlrDirPath

	return result
}

func (c *vfiouserSubsystemListener) PreAdd(ctrlr *pb.NVMeController) error {
	ctrlrDirPath := c.controllerDirPath(ctrlr)
	log.Printf("Creating dir for %v NVMe controller: %v", ctrlr.Spec.Id.Value, ctrlrDirPath)
	if os.Mkdir(ctrlrDirPath, 0600) != nil {
		return fmt.Errorf("cannot create controller directory %v", ctrlrDirPath)
	}
	return nil
}

func (c *vfiouserSubsystemListener) PostRemove(ctrlr *pb.NVMeController) error {
	ctrlrDirPath := c.controllerDirPath(ctrlr)
	log.Printf("Deleting dir for %v NVMe controller: %v", ctrlr.Spec.Id.Value, ctrlrDirPath)
	if _, err := os.Stat(ctrlrDirPath); os.IsNotExist(err) {
		log.Printf("%v directory does not exist.", ctrlrDirPath)
		return nil
	}

	if os.Remove(ctrlrDirPath) != nil {
		return fmt.Errorf("cannot delete controller directory %v", ctrlrDirPath)
	}
	return nil
}

func (c *vfiouserSubsystemListener) controllerDirPath(ctrlr *pb.NVMeController) string {
	return filepath.Join(c.ctrlrDir, ctrlr.Spec.Id.Value)
}
