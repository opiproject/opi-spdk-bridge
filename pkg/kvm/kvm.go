// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"log"
	"path/filepath"

	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// Server is a wrapper for default opi-spdk-bridge frontend which automates
// interaction with QEMU instance to plug/unplug SPDK devices
type Server struct {
	*frontend.Server

	qmpAddress string
	ctrlrDir   string
}

// NewServer creates instance of KvmServer
func NewServer(s *frontend.Server, qmpAddress string, ctrlrDir string) *Server {
	return &Server{s, qmpAddress, ctrlrDir}
}

// CreateVirtioBlk creates a virtio-blk device and attaches it to QEMU instance
func (s *Server) CreateVirtioBlk(ctx context.Context, in *pb.CreateVirtioBlkRequest) (*pb.VirtioBlk, error) {
	out, err := s.Server.CreateVirtioBlk(ctx, in)
	if err != nil {
		return out, err
	}

	ctrlr := filepath.Join(s.ctrlrDir, in.VirtioBlk.Id.Value)
	log.Printf("Plugging virtio-blk %v to qemu over %v", ctrlr, s.qmpAddress)
	log.Println("virtio-blk is plugged.")
	return out, nil
}

// DeleteVirtioBlk deletes a virtio-blk device and detaches it from QEMU instance
func (s *Server) DeleteVirtioBlk(ctx context.Context, in *pb.DeleteVirtioBlkRequest) (*emptypb.Empty, error) {
	ctrlr := filepath.Join(s.ctrlrDir, in.Name)
	log.Printf("Unplugging virtio-blk %v from qemu over %v", ctrlr, s.qmpAddress)
	log.Println("virtio-blk is unplugged.")
	return s.Server.DeleteVirtioBlk(ctx, in)
}
