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

	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

// CreateNvmeController creates an Nvme controller device and attaches it to QEMU instance
func (s *Server) CreateNvmeController(ctx context.Context, in *pb.CreateNvmeControllerRequest) (*pb.NvmeController, error) {
	if in.GetNvmeController().GetSpec().GetTrtype() != pb.NvmeTransportType_NVME_TRANSPORT_PCIE {
		return s.Server.CreateNvmeController(ctx, in)
	}

	if in.Parent == "" {
		return nil, errInvalidSubsystem
	}
	if in.GetNvmeController().GetSpec().GetPcieId() == nil {
		log.Println("Pci endpoint should be specified")
		return nil, errNoPcieEndpoint
	}
	location, err := s.locator.Calculate(in.GetNvmeController().GetSpec().GetPcieId())
	if err != nil {
		log.Println("Failed to calculate device location: ", err)
		return nil, errDeviceEndpoint
	}

	// Create request can miss Name field which is generated in spdk bridge.
	// Use subsystem instead, since it is required to exist
	dirName := utils.GetSubsystemIDFromNvmeName(in.Parent)
	if dirName == "" {
		log.Println("Failed to get subsystem id from:", in.Parent)
		return nil, errInvalidSubsystem
	}

	err = createControllerDir(s.ctrlrDir, dirName)
	if err != nil {
		log.Print(err)
		return nil, errFailedToCreateNvmeDir
	}

	out, err := s.Server.CreateNvmeController(ctx, in)
	if err != nil {
		log.Println("Error running cmd on opi-spdk bridge:", err)
		_ = deleteControllerDir(s.ctrlrDir, dirName)
		return out, err
	}
	name := out.Name

	mon, monErr := newMonitor(s.qmpAddress, s.protocol, s.timeout, s.pollDevicePresenceStep)
	if monErr != nil {
		log.Println("Couldn't create QEMU monitor")
		_, _ = s.Server.DeleteNvmeController(context.Background(), &pb.DeleteNvmeControllerRequest{Name: name})
		_ = deleteControllerDir(s.ctrlrDir, dirName)
		return nil, errMonitorCreation
	}
	defer mon.Disconnect()

	qemuDeviceID := toQemuID(name)
	if err := mon.AddNvmeControllerDevice(qemuDeviceID, controllerDirPath(s.ctrlrDir, dirName), location); err != nil {
		log.Println("Couldn't add Nvme controller:", err)
		_, _ = s.Server.DeleteNvmeController(context.Background(), &pb.DeleteNvmeControllerRequest{Name: name})
		_ = deleteControllerDir(s.ctrlrDir, dirName)
		return nil, errAddDeviceFailed
	}
	return out, nil
}

// DeleteNvmeController deletes an Nvme controller device and detaches it from QEMU instance
func (s *Server) DeleteNvmeController(ctx context.Context, in *pb.DeleteNvmeControllerRequest) (*emptypb.Empty, error) {
	controller, ok := s.Nvme.Controllers[in.GetName()]
	if !ok || controller.GetSpec().GetTrtype() != pb.NvmeTransportType_NVME_TRANSPORT_PCIE {
		return s.Server.DeleteNvmeController(ctx, in)
	}

	mon, monErr := newMonitor(s.qmpAddress, s.protocol, s.timeout, s.pollDevicePresenceStep)
	if monErr != nil {
		log.Println("Couldn't create QEMU monitor")
		return nil, errMonitorCreation
	}
	defer mon.Disconnect()

	dirName, findDirNameErr := s.findDirName(in.Name)
	if findDirNameErr != nil {
		log.Println("Failed to detect controller directory name:", findDirNameErr)
		return nil, findDirNameErr
	}

	qemuDeviceID := toQemuID(in.Name)
	delNvmeErr := mon.DeleteNvmeControllerDevice(qemuDeviceID)
	if delNvmeErr != nil {
		log.Printf("Couldn't delete Nvme controller: %v", delNvmeErr)
	}

	response, spdkErr := s.Server.DeleteNvmeController(ctx, in)
	if spdkErr != nil {
		log.Println("Error running underlying cmd on opi-spdk bridge:", spdkErr)
	}

	delDirErr := deleteControllerDir(s.ctrlrDir, dirName)
	if delDirErr != nil {
		log.Println("Failed to delete Nvme controller directory:", delDirErr)
	}

	var err error
	if delNvmeErr != nil && spdkErr != nil && delDirErr != nil {
		err = errDeviceNotDeleted
	} else if delNvmeErr != nil || spdkErr != nil || delDirErr != nil {
		err = errDevicePartiallyDeleted
	}
	return response, err
}

func (s *Server) findDirName(name string) (string, error) {
	ctrlr, ok := s.Server.Nvme.Controllers[name]
	if !ok {
		return "", errNoController
	}

	subsystemID := utils.GetSubsystemIDFromNvmeName(ctrlr.Name)
	if subsystemID == "" {
		return "", errInvalidSubsystem
	}

	return subsystemID, nil
}

func createControllerDir(ctrlrDir string, dirName string) error {
	if dirName == "" {
		return fmt.Errorf("dirName cannot be empty")
	}

	ctrlrDirPath := controllerDirPath(ctrlrDir, dirName)
	log.Printf("Creating dir for Nvme controller: %v", ctrlrDirPath)
	if os.Mkdir(ctrlrDirPath, 0600) != nil {
		return fmt.Errorf("cannot create controller directory %v", ctrlrDirPath)
	}
	return nil
}

func deleteControllerDir(ctrlrDir string, dirName string) error {
	ctrlrDirPath := controllerDirPath(ctrlrDir, dirName)
	log.Printf("Deleting dir for Nvme controller: %v", ctrlrDirPath)
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
