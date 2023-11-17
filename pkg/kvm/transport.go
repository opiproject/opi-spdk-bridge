// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type nvmeVfiouserTransport struct {
	ctrlrDir string
	rpc      spdk.JSONRPC
}

// NewNvmeVfiouserTransport creates a new instance of nvmeVfiouserTransport
func NewNvmeVfiouserTransport(ctrlrDir string, rpc spdk.JSONRPC) frontend.NvmeTransport {
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

	if rpc == nil {
		log.Panicf("rpc cannot be nil")
	}

	return &nvmeVfiouserTransport{
		ctrlrDir: ctrlrDir,
		rpc:      rpc,
	}
}

func (c *nvmeVfiouserTransport) CreateController(
	ctx context.Context,
	ctrlr *pb.NvmeController,
	subsys *pb.NvmeSubsystem,
) error {
	pcieID := ctrlr.GetSpec().GetPcieId()
	if pcieID.PortId.Value != 0 {
		return status.Error(codes.InvalidArgument, "only port 0 is supported for vfiouser")
	}

	if pcieID.VirtualFunction.Value != 0 {
		return status.Error(codes.InvalidArgument, "virtual functions are not supported for vfiouser")
	}

	if subsys.Spec.Hostnqn != "" {
		return status.Error(codes.InvalidArgument, "hostnqn for subsystem is not supported for vfiouser")
	}

	params := c.params(ctrlr, subsys)
	var result spdk.NvmfSubsystemAddListenerResult
	err := c.rpc.Call(ctx, "nvmf_subsystem_add_listener", &params, &result)
	if err != nil {
		return err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not create CTRL: %s", ctrlr.Name)
		return status.Errorf(codes.InvalidArgument, msg)
	}

	return nil
}

func (c *nvmeVfiouserTransport) DeleteController(
	ctx context.Context,
	ctrlr *pb.NvmeController,
	subsys *pb.NvmeSubsystem,
) error {
	params := c.params(ctrlr, subsys)
	var result spdk.NvmfSubsystemAddListenerResult
	err := c.rpc.Call(ctx, "nvmf_subsystem_remove_listener", &params, &result)
	if err != nil {
		return err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete CTRL: %s", ctrlr.Name)
		return status.Errorf(codes.InvalidArgument, msg)
	}

	return nil
}

func (c *nvmeVfiouserTransport) params(
	ctrlr *pb.NvmeController,
	subsys *pb.NvmeSubsystem,
) spdk.NvmfSubsystemAddListenerParams {
	params := spdk.NvmfSubsystemAddListenerParams{}
	ctrlrDirPath := controllerDirPath(c.ctrlrDir, utils.GetSubsystemIDFromNvmeName(ctrlr.Name))
	params.Nqn = subsys.Spec.Nqn
	params.ListenAddress.Trtype = "vfiouser"
	params.ListenAddress.Traddr = ctrlrDirPath

	return params
}
