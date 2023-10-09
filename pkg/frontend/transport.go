// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"errors"
	"path"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

// NvmeTransport interface is used to provide SPDK call params to create/delete
// Nvme controllers depending on used transport type.
type NvmeTransport interface {
	Params(ctrlr *pb.NvmeController, subsys *pb.NvmeSubsystem) (spdk.NvmfSubsystemAddListenerParams, error)
}

// VirtioBlkTransport interface is used to provide SPDK call params to create/delete
// virtio-blk controllers depending on used transport type.
type VirtioBlkTransport interface {
	CreateParams(virtioBlk *pb.VirtioBlk) (any, error)
	DeleteParams(virtioBlk *pb.VirtioBlk) (any, error)
}

type nvmeTCPTransport struct{}

// NewNvmeTCPTransport creates a new instance of nvmeTcpTransport
func NewNvmeTCPTransport() NvmeTransport {
	return &nvmeTCPTransport{}
}

func (c *nvmeTCPTransport) Params(ctrlr *pb.NvmeController, subsys *pb.NvmeSubsystem) (spdk.NvmfSubsystemAddListenerParams, error) {
	result := spdk.NvmfSubsystemAddListenerParams{}
	result.Nqn = subsys.Spec.Nqn
	result.SecureChannel = len(subsys.Spec.Psk) > 0
	result.ListenAddress.Trtype = "tcp"
	result.ListenAddress.Traddr = ctrlr.GetSpec().GetFabricsId().GetTraddr()
	result.ListenAddress.Trsvcid = ctrlr.GetSpec().GetFabricsId().GetTrsvcid()
	result.ListenAddress.Adrfam = utils.OpiAdressFamilyToSpdk(
		ctrlr.GetSpec().GetFabricsId().GetAdrfam(),
	)

	return result, nil
}

type vhostUserBlkTransport struct{}

// NewVhostUserBlkTransport creates objects to handle vhost user blk transport
// specifics
func NewVhostUserBlkTransport() VirtioBlkTransport {
	return &vhostUserBlkTransport{}
}

func (v vhostUserBlkTransport) CreateParams(virtioBlk *pb.VirtioBlk) (any, error) {
	if err := v.verifyTransportSpecificParams(virtioBlk); err != nil {
		return nil, err
	}

	resourceID := path.Base(virtioBlk.Name)
	return spdk.VhostCreateBlkControllerParams{
		Ctrlr:   resourceID,
		DevName: virtioBlk.VolumeNameRef,
	}, nil
}

func (v vhostUserBlkTransport) DeleteParams(virtioBlk *pb.VirtioBlk) (any, error) {
	if err := v.verifyTransportSpecificParams(virtioBlk); err != nil {
		return nil, err
	}

	resourceID := path.Base(virtioBlk.Name)
	return spdk.VhostDeleteControllerParams{
		Ctrlr: resourceID,
	}, nil
}

func (v vhostUserBlkTransport) verifyTransportSpecificParams(virtioBlk *pb.VirtioBlk) error {
	pcieID := virtioBlk.PcieId
	if pcieID.PortId.Value != 0 {
		return errors.New("only port 0 is supported for vhost-user-blk")
	}

	if pcieID.VirtualFunction.Value != 0 {
		return errors.New("virtual functions are not supported for vhost-user-blk")
	}

	return nil
}
