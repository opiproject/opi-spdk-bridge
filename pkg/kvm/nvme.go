// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
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
	ctrlrDirPath := controllerDirPath(c.ctrlrDir, ctrlr.Spec.Id.Value)
	result.Nqn = nqn
	result.ListenAddress.Trtype = "vfiouser"
	result.ListenAddress.Traddr = ctrlrDirPath

	return result
}

// CreateNVMeController creates an NVMe controller device and attaches it to QEMU instance
func (s *Server) CreateNVMeController(ctx context.Context, in *pb.CreateNVMeControllerRequest) (*pb.NVMeController, error) {
	id := in.NvMeController.Spec.Id.Value
	err := createControllerDir(s.ctrlrDir, id)
	if err != nil {
		log.Print(err)
		return nil, errFailedToCreateNvmeDir
	}

	out, err := s.Server.CreateNVMeController(ctx, in)
	if err != nil {
		log.Println("Error running cmd on opi-spdk bridge:", err)
		_ = deleteControllerDir(s.ctrlrDir, id)
		return out, err
	}

	mon, monErr := newMonitor(s.qmpAddress, s.protocol, s.timeout)
	if monErr != nil {
		log.Println("Couldn't create QEMU monitor")
		_, _ = s.Server.DeleteNVMeController(context.Background(), &pb.DeleteNVMeControllerRequest{Name: id})
		_ = deleteControllerDir(s.ctrlrDir, id)
		return nil, errMonitorCreation
	}
	defer mon.Disconnect()

	if err := mon.AddNvmeControllerDevice(id, controllerDirPath(s.ctrlrDir, id)); err != nil {
		log.Println("Couldn't add NVMe controller:", err)
		_, _ = s.Server.DeleteNVMeController(context.Background(), &pb.DeleteNVMeControllerRequest{Name: id})
		_ = deleteControllerDir(s.ctrlrDir, id)
		return nil, errAddDeviceFailed
	}
	return out, nil
}

func createControllerDir(ctrlrDir string, ctrlrID string) error {
	ctrlrDirPath := controllerDirPath(ctrlrDir, ctrlrID)
	log.Printf("Creating dir for %v NVMe controller: %v", ctrlrID, ctrlrDirPath)
	if os.Mkdir(ctrlrDirPath, 0600) != nil {
		return fmt.Errorf("cannot create controller directory %v", ctrlrDirPath)
	}
	return nil
}

func deleteControllerDir(ctrlrDir string, ctrlrID string) error {
	ctrlrDirPath := controllerDirPath(ctrlrDir, ctrlrID)
	log.Printf("Deleting dir for %v NVMe controller: %v", ctrlrID, ctrlrDirPath)
	if _, err := os.Stat(ctrlrDirPath); os.IsNotExist(err) {
		log.Printf("%v directory does not exist.", ctrlrDirPath)
		return nil
	}

	if os.Remove(ctrlrDirPath) != nil {
		return fmt.Errorf("cannot delete controller directory %v", ctrlrDirPath)
	}
	return nil
}

func controllerDirPath(ctrlrDir string, ctrlrID string) string {
	return filepath.Join(ctrlrDir, ctrlrID)
}
