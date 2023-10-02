// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"log"
	"path"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// NvmeTransport interface is used to provide SPDK call params to create/delete
// Nvme controllers depending on used transport type.
type NvmeTransport interface {
	Params(ctrlr *pb.NvmeController, nqn string) (spdk.NvmfSubsystemAddListenerParams, error)
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

func (c *nvmeTCPTransport) Params(_ *pb.NvmeController, nqn string) (spdk.NvmfSubsystemAddListenerParams, error) {
	result := spdk.NvmfSubsystemAddListenerParams{}
	result.Nqn = nqn
	result.SecureChannel = false
	result.ListenAddress.Trtype = "tcp"
	result.ListenAddress.Traddr = ""
	result.ListenAddress.Trsvcid = ""
	result.ListenAddress.Adrfam = ""

	return result, nil
}

type vhostUserBlkTransport struct{}

// NewVhostUserBlkTransport creates objects to handle vhost user blk transport
// specifics
func NewVhostUserBlkTransport() VirtioBlkTransport {
	return &vhostUserBlkTransport{}
}

func (v vhostUserBlkTransport) CreateParams(virtioBlk *pb.VirtioBlk) (any, error) {
	v.verifyTransportSpecificParams(virtioBlk)

	resourceID := path.Base(virtioBlk.Name)
	return spdk.VhostCreateBlkControllerParams{
		Ctrlr:   resourceID,
		DevName: virtioBlk.VolumeNameRef,
	}, nil
}

func (v vhostUserBlkTransport) DeleteParams(virtioBlk *pb.VirtioBlk) (any, error) {
	v.verifyTransportSpecificParams(virtioBlk)

	resourceID := path.Base(virtioBlk.Name)
	return spdk.VhostDeleteControllerParams{
		Ctrlr: resourceID,
	}, nil
}

func (v vhostUserBlkTransport) verifyTransportSpecificParams(virtioBlk *pb.VirtioBlk) {
	pcieID := virtioBlk.PcieId
	if pcieID.PortId.Value != 0 {
		log.Printf("WARNING: only port 0 is supported for vhost user. Will be replaced with an error")
	}

	if pcieID.VirtualFunction.Value != 0 {
		log.Println("WARNING: virtual functions are not supported for vhost user. Will be replaced with an error")
	}
}
