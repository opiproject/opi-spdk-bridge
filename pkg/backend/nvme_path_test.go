// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testNvmePathID   = "mytest"
	testNvmePathName = server.ResourceIDToVolumeName(testNvmePathID)
	testNvmePath     = pb.NVMfPath{
		Trtype:       pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		Adrfam:       pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
		Traddr:       "127.0.0.1",
		Trsvcid:      4444,
		Subnqn:       "nqn.2016-06.io.spdk:cnode1",
		Hostnqn:      "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
		ControllerId: &pc.ObjectKey{Value: controllerName},
	}
)

func TestBackEnd_CreateNVMfPath(t *testing.T) {
	tests := map[string]struct {
		id      string
		in      *pb.NVMfPath
		out     *pb.NVMfPath
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&testNvmePath,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
			false,
		},
		"valid request with invalid marshal SPDK response": {
			testNvmePathID,
			&testNvmePath,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevNvmeAttachControllerResult"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testNvmePathID,
			&testNvmePath,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNvmePathID,
			&testNvmePath,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[""]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testNvmePathID,
			&testNvmePath,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[""]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testNvmePathID,
			&testNvmePath,
			&testNvmePath,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":["mytest"]}`},
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			testNvmePathID,
			&testNvmePath,
			&testNvmePath,
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[controllerName] = &controller
			if tt.exist {
				testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathName] = &testNvmePath
			}
			if tt.out != nil {
				tt.out.Name = testNvmePathName
			}

			request := &pb.CreateNVMfPathRequest{NvMfPath: tt.in, NvMfPathId: tt.id}
			response, err := testEnv.client.CreateNVMfPath(testEnv.ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_DeleteNVMfPath(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testNvmePathID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Null Dev: %s", testNvmePathID),
			true,
			false,
		},
		"valid request with invalid marshal SPDK response": {
			testNvmePathID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json: cannot unmarshal array into Go value of type spdk.BdevNvmeDetachControllerResult"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testNvmePathID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNvmePathID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testNvmePathID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testNvmePathID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			fname1 := server.ResourceIDToVolumeName(tt.in)
			testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathName] = &testNvmePath
			testEnv.opiSpdkServer.Volumes.NvmeControllers[controllerName] = &controller

			request := &pb.DeleteNVMfPathRequest{Name: fname1, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNVMfPath(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}
