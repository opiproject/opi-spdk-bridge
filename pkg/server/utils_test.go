// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package server implements the server
package server

import (
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/protobuf/proto"
)

func TestProtoClone(t *testing.T) {
	tests := map[string]struct {
		in *pb.NvmeController
	}{
		"proto structure": {
			in: &pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					PcieId: &pb.PciEndpoint{},
				},
			},
		},
		"nil proto structure": {
			in: nil,
		},
	}
	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			copiedIn := ProtoClone(tt.in)

			if !proto.Equal(tt.in, copiedIn) {
				t.Errorf("Expect proto structure copy %v, received: %v", tt.in, copiedIn)
			}
			if tt.in != nil && tt.in.Spec.PcieId == copiedIn.Spec.PcieId {
				t.Errorf("Expect deep copy, not pointer copy")
			}
		})
	}
}
