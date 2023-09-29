// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implements the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"log"
	"net"
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

const (
	ipv4NvmeTCPProtocol = "ipv4"
	ipv6NvmeTCPProtocol = "ipv6"
)

// TODO: consider using https://pkg.go.dev/net#TCPAddr
type nvmeTCPTransport struct {
	listenAddr net.IP
	listenPort string
	protocol   string
}

// NewNvmeTCPTransport creates a new instance of nvmeTcpTransport
func NewNvmeTCPTransport(listenAddr string) NvmeTransport {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		log.Panicf("Invalid ip:port tuple: %v", listenAddr)
	}

	parsedAddr := net.ParseIP(host)
	if parsedAddr == nil {
		log.Panicf("Invalid ip address: %v", host)
	}

	var protocol string
	switch {
	case parsedAddr.To4() != nil:
		protocol = ipv4NvmeTCPProtocol
	case parsedAddr.To16() != nil:
		protocol = ipv6NvmeTCPProtocol
	default:
		log.Panicf("Not supported protocol for: %v", listenAddr)
	}

	return &nvmeTCPTransport{
		listenAddr: parsedAddr,
		listenPort: port,
		protocol:   protocol,
	}
}

func (c *nvmeTCPTransport) Params(_ *pb.NvmeController, nqn string) (spdk.NvmfSubsystemAddListenerParams, error) {
	result := spdk.NvmfSubsystemAddListenerParams{}
	result.Nqn = nqn
	result.SecureChannel = false
	result.ListenAddress.Trtype = "tcp"
	result.ListenAddress.Traddr = c.listenAddr.String()
	result.ListenAddress.Trsvcid = c.listenPort
	result.ListenAddress.Adrfam = c.protocol

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
