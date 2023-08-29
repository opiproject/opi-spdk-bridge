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

func TestEqualProtoSlices(t *testing.T) {
	tests := map[string]struct {
		x     []*pb.NvmeController
		y     []*pb.NvmeController
		equal bool
	}{
		"nils": {
			x:     nil,
			y:     nil,
			equal: true,
		},
		"nil, empty array": {
			x:     nil,
			y:     []*pb.NvmeController{},
			equal: true,
		},
		"nil, non empty array": {
			x:     nil,
			y:     []*pb.NvmeController{{Name: "0"}},
			equal: false,
		},
		"both non empty arrays": {
			x:     []*pb.NvmeController{{Name: "0"}, {Name: "1"}},
			y:     []*pb.NvmeController{{Name: "0"}, {Name: "1"}},
			equal: true,
		},
		"non empty but different arrays": {
			x:     []*pb.NvmeController{{Name: "0"}, {Name: "1"}},
			y:     []*pb.NvmeController{{Name: "0"}, {Name: "2"}},
			equal: false,
		},
		"non empty arrays with different sizes": {
			x:     []*pb.NvmeController{{Name: "0"}, {Name: "1"}},
			y:     []*pb.NvmeController{{Name: "0"}},
			equal: false,
		},
	}
	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			if EqualProtoSlices(tt.x, tt.y) != tt.equal {
				t.Errorf("Expect x: %v and y: %v are equal: %v", tt.x, tt.y, tt.equal)
			}
		})
	}
}

type testChangeProtoObjReporter struct {
	reported bool
}

func (r *testChangeProtoObjReporter) Fatalf(string, ...any) {
	r.reported = true
}

func TestCheckTestProtoObjectsNotChanged(t *testing.T) {
	tests := map[string]struct {
		msgs   []proto.Message
		change bool
	}{
		"no change for single object arg": {
			msgs: []proto.Message{
				&pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: 5}},
			},
			change: false,
		},
		"change for single object arg": {
			msgs: []proto.Message{
				&pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: 5}},
			},
			change: true,
		},
		"no change for multiple object args": {
			msgs: []proto.Message{
				&pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: 5}},
				&pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: 7}},
			},
			change: false,
		},
		"change for multiple object args": {
			msgs: []proto.Message{
				&pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: 5}},
				&pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: 7}},
			},
			change: true,
		},
	}
	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			r := &testChangeProtoObjReporter{}

			testCleanup := CheckTestProtoObjectsNotChanged(tt.msgs...)(r, "test")
			if tt.change {
				tt.msgs[len(tt.msgs)-1].(*pb.NvmeNamespace).Spec.SubsystemNameRef = "somevolume"
			}
			testCleanup()

			if tt.change != r.reported {
				t.Errorf("Expect error is reported: %v, received: %v", tt.change, r.reported)
			}
		})
	}
}
