// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testNvmePathID   = "mytest"
	testNvmePathName = server.ResourceIDToVolumeName(testNvmePathID)
	testNvmePath     = pb.NvmePath{
		Trtype:            pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		Traddr:            "127.0.0.1",
		ControllerNameRef: testNvmeCtrlName,
		Fabrics: &pb.FabricsPath{
			Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
			Subnqn:  "nqn.2016-06.io.spdk:cnode1",
			Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
			Trsvcid: 4444,
		},
	}
)

func TestBackEnd_CreateNvmePath(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		id      string
		in      *pb.NvmePath
		out     *pb.NvmePath
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			id:      "CapitalLettersNotAllowed",
			in:      &testNvmePath,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			exist:   false,
		},
		"valid request with invalid marshal SPDK response": {
			id:      testNvmePathID,
			in:      &testNvmePath,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_attach_controller: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevNvmeAttachControllerResult"),
			exist:   false,
		},
		"valid request with empty SPDK response": {
			id:      testNvmePathID,
			in:      &testNvmePath,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_attach_controller: %v", "EOF"),
			exist:   false,
		},
		"valid request with ID mismatch SPDK response": {
			id:      testNvmePathID,
			in:      &testNvmePath,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[""]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response ID mismatch"),
			exist:   false,
		},
		"valid request with error code from SPDK response": {
			id:      testNvmePathID,
			in:      &testNvmePath,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[""]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response error: myopierr"),
			exist:   false,
		},
		"valid request with valid SPDK response": {
			id:      testNvmePathID,
			in:      &testNvmePath,
			out:     &testNvmePath,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":["mytest"]}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
		},
		"already exists": {
			id:      testNvmePathID,
			in:      &testNvmePath,
			out:     &testNvmePath,
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			exist:   true,
		},
		"no required field": {
			id:      testAioVolumeID,
			in:      nil,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: nvme_path",
			exist:   false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = server.ProtoClone(&testNvmeCtrl)
			if tt.exist {
				testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathName] = server.ProtoClone(&testNvmePath)
				testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathName].Name = testNvmePathName
			}
			if tt.out != nil {
				tt.out = server.ProtoClone(tt.out)
				tt.out.Name = testNvmePathName
			}

			request := &pb.CreateNvmePathRequest{NvmePath: tt.in, NvmePathId: tt.id}
			response, err := testEnv.client.CreateNvmePath(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
	pskTests := map[string]struct {
		createErr error
		writeErr  error
		spdk      []string
		errCode   codes.Code
		errMsg    string
	}{
		"tmp key file creation failed": {
			createErr: errors.New("stub error"),
			writeErr:  nil,
			spdk:      []string{},
			errCode:   codes.Internal,
			errMsg:    "failed to handle key",
		},
		"tmp key file write failed": {
			createErr: nil,
			writeErr:  errors.New("stub error"),
			spdk:      []string{},
			errCode:   codes.Internal,
			errMsg:    "failed to handle key",
		},
		"tmp key file removed after successful call": {
			createErr: nil,
			writeErr:  nil,
			spdk:      []string{`{"id":%d,"error":{"code":0,"message":""},"result":["mytest"]}`},
			errCode:   codes.OK,
			errMsg:    "",
		},
	}

	for name, tt := range pskTests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			const expectedKey = "NVMeTLSkey-1:01:MDAxMTIyMzM0NDU1NjY3Nzg4OTlhYWJiY2NkZGVlZmZwJEiQ:"
			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] =
				&pb.NvmeRemoteController{
					Multipath: pb.NvmeMultipath_NVME_MULTIPATH_MULTIPATH,
					Tcp: &pb.TcpController{
						Hdgst: false,
						Ddgst: false,
						Psk:   []byte(expectedKey),
					},
				}

			createdKeyFile := ""
			origCreateTempFile := testEnv.opiSpdkServer.psk.createTempFile
			testEnv.opiSpdkServer.psk.createTempFile =
				func(dir, pattern string) (*os.File, error) {
					if tt.createErr == nil {
						keyFile, _ := origCreateTempFile(t.TempDir(), pattern)
						createdKeyFile = keyFile.Name()
						return keyFile, nil
					}
					return nil, tt.createErr
				}
			origWriteKey := testEnv.opiSpdkServer.psk.writeKey
			testEnv.opiSpdkServer.psk.writeKey =
				func(keyFile string, key []byte, perm os.FileMode) error {
					if createdKeyFile != keyFile {
						t.Errorf("Expected key is written to: %v, instead: %v", createdKeyFile, keyFile)
					}
					if _, err := os.Stat(createdKeyFile); err != nil {
						t.Errorf("Expected temporary key file %v exists", createdKeyFile)
					}
					_ = origWriteKey(keyFile, key, perm)
					written, _ := os.ReadFile(filepath.Clean(keyFile))
					if string(written) != expectedKey {
						t.Errorf("Expected psk key: %v is written, received: %v", expectedKey, key)
					}
					return tt.writeErr
				}

			request := &pb.CreateNvmePathRequest{NvmePath: &testNvmePath, NvmePathId: "nvmetcppath0"}
			_, err := testEnv.client.CreateNvmePath(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}

			if _, err := os.Stat(createdKeyFile); err == nil {
				t.Errorf("Expect temporary key file %v is removed", createdKeyFile)
			}
		})
	}
}

func TestBackEnd_DeleteNvmePath(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid SPDK response": {
			in:      testNvmePathName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete Nvme Path: %s", testNvmePathID),
			missing: false,
		},
		"valid request with invalid marshal SPDK response": {
			in:      testNvmePathName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_detach_controller: %v", "json: cannot unmarshal array into Go value of type spdk.BdevNvmeDetachControllerResult"),
			missing: false,
		},
		"valid request with empty SPDK response": {
			in:      testNvmePathName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_detach_controller: %v", "EOF"),
			missing: false,
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNvmePathName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response ID mismatch"),
			missing: false,
		},
		"valid request with error code from SPDK response": {
			in:      testNvmePathName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			in:      testNvmePathName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			in:      server.ResourceIDToVolumeName("unknown-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			missing: false,
		},
		"unknown key with missing allowed": {
			in:      server.ResourceIDToVolumeName("unknown-id"),
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			missing: true,
		},
		"malformed name": {
			in:      server.ResourceIDToVolumeName("-ABC-DEF"),
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			missing: false,
		},
		"no required field": {
			in:      "",
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
			missing: false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathName] = server.ProtoClone(&testNvmePath)
			testEnv.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName] = server.ProtoClone(&testNvmeCtrl)

			request := &pb.DeleteNvmePathRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmePath(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}

			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}

func TestBackEnd_UpdateNvmePath(t *testing.T) {
	testNvmePathWithName := server.ProtoClone(&testNvmePath)
	testNvmePathWithName.Name = testNvmePathName
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(testNvmePathWithName)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmePath
		out     *pb.NvmePath
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"invalid fieldmask": {
			mask:    &fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			in:      testNvmePathWithName,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			missing: false,
		},
		// "delete fails": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
		// 	errCode: codes.InvalidArgument,
		// 	errMsg: fmt.Sprintf("Could not delete Null Dev: %s", testNvmePathID),
		//	missing: false,
		// },
		// "delete empty": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{""},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_detach_controller: %v", "EOF"),
		//	missing: false,
		// },
		// "delete ID mismatch": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response ID mismatch"),
		//	missing: false,
		// },
		// "delete exception": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response error: myopierr"),
		//	missing: false,
		// },
		// "delete ok create fails": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
		// 	errCode: codes.InvalidArgument,
		// 	errMsg: fmt.Sprintf("Could not create Null Dev: %v", "mytest"),
		//	missing: false,
		// },
		// "delete ok create empty": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_attach_controller: %v", "EOF"),
		//	missing: false,
		// },
		// "delete ok create ID mismatch": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response ID mismatch"),
		//	missing: false,
		// },
		// "delete ok create exception": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response error: myopierr"),
		//	missing: false,
		// },
		// "valid request with valid SPDK response": {
		// 	mask: nil,
		// 	in: &testNvmePath,
		// 	out: &testNvmePath,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
		// 	errCode: codes.OK,
		// 	errMsg: "",
		//	missing: false,
		// },
		"valid request with unknown key": {
			mask: nil,
			in: &pb.NvmePath{
				Name:              server.ResourceIDToVolumeName("unknown-id"),
				Trtype:            pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				Traddr:            "127.0.0.1",
				ControllerNameRef: "TBD",
				Fabrics: &pb.FabricsPath{
					Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
					Trsvcid: 4444,
					Subnqn:  testNvmePath.Fabrics.Subnqn,
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			missing: false,
		},
		"unknown key with missing allowed": {
			mask: nil,
			in: &pb.NvmePath{
				Name:              server.ResourceIDToVolumeName("unknown-id"),
				Trtype:            pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				Traddr:            "127.0.0.1",
				ControllerNameRef: "TBD",
				Fabrics: &pb.FabricsPath{
					Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
					Trsvcid: 4444,
					Subnqn:  testNvmePath.Fabrics.Subnqn,
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			missing: true,
		},
		"malformed name": {
			mask: nil,
			in: &pb.NvmePath{
				Name:              "-ABC-DEF",
				ControllerNameRef: "TBD",
				Trtype:            testNvmePath.Trtype,
				Traddr:            testNvmePath.Traddr,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			missing: false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathName] = server.ProtoClone(&testNvmePath)
			testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathName].Name = testNvmePathName

			request := &pb.UpdateNvmePathRequest{NvmePath: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateNvmePath(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestBackEnd_ListNvmePaths(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     []*pb.NvmePath
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		// "valid request with invalid SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
		// 	errCode: codes.InvalidArgument,
		// 	errMsg: fmt.Sprintf("Could not find any namespaces for NQN: %v", "nqn.2022-09.io.spdk:opi3"),
		// 	size: 0,
		// 	token: "",
		// },
		// "valid request with invalid marshal SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_get_controllers: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
		// 	size: 0,
		// 	token: "",
		// },
		// "valid request with empty SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{""},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_get_controllers: %v", "EOF"),
		// 	size: 0,
		// 	token: "",
		// },
		// "valid request with ID mismatch SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response ID mismatch"),
		// 	size: 0,
		// 	token: "",
		// },
		// "valid request with error code from SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response error: myopierr"),
		// 	size: 0,
		// 	token: "",
		// },
		// "valid request with valid SPDK response": {
		// 	in: testNvmePathID,
		// 	out: []*pb.NvmePath{
		// 		{
		// 			Name:    "Malloc0",
		// 			Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		// 			Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
		// 			Traddr:  "127.0.0.1",
		// 			Trsvcid: 4444,
		// 		},
		// 		{
		// 			Name:    "Malloc1",
		// 			Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		// 			Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
		// 			Traddr:  "127.0.0.1",
		// 			Trsvcid: 4444,
		// 		},
		// 	},
		// 	spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":[` +
		// 		`{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},` +
		// 		`{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}` +
		// 		`]}`},
		// 	errCode: codes.OK,
		// 	errMsg: "",
		// 	size: 0,
		// 	token: "",
		// },
		// "pagination overflow": {
		// 	in: testNvmePathID,
		// 	out: []*pb.NvmePath{
		// 		{
		// 			Name:    "Malloc0",
		// 			Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		// 			Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
		// 			Traddr:  "127.0.0.1",
		// 			Trsvcid: 4444,
		// 		},
		// 		{
		// 			Name:    "Malloc1",
		// 			Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		// 			Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
		// 			Traddr:  "127.0.0.1",
		// 			Trsvcid: 4444,
		// 		},
		// 	},
		// 	spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
		// 	errCode: codes.OK,
		// 	errMsg: "",
		// 	size: 1000,
		// 	token: "",
		// },
		// "pagination negative": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{},
		// 	errCode: codes.InvalidArgument,
		// 	errMsg: "negative PageSize is not allowed",
		// 	size: -10,
		// 	token: "",
		// },
		// "pagination error": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{},
		// 	errCode: codes.NotFound,
		// 	errMsg: fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
		// 	size: 0,
		// 	token: "unknown-pagination-token",
		// },
		// "pagination": {
		// 	in: testNvmePathID,
		// 	out: []*pb.NvmePath{
		// 		{
		// 			Name:    "Malloc0",
		// 			Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		// 			Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
		// 			Traddr:  "127.0.0.1",
		// 			Trsvcid: 4444,
		// 		},
		// 	},
		// 	spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
		// 	errCode: codes.OK,
		// 	errMsg: "",
		// 	size: 1,
		// 	token: "",
		// },
		// "pagination offset": {
		// 	in: testNvmePathID,
		// 	out: []*pb.NvmePath{
		// 		{
		// 			Name:    "Malloc1",
		// 			Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		// 			Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
		// 			Traddr:  "127.0.0.1",
		// 			Trsvcid: 4444,
		// 		},
		// 	},
		// 	spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
		// 	errCode: codes.OK,
		// 	errMsg: "",
		// 	size: 1,
		// 	token: "existing-pagination-token",
		// },
		"no required field": {
			in:      "",
			out:     []*pb.NvmePath{},
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: parent",
			size:    0,
			token:   "",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmePathsRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmePaths(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetNvmePaths(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNvmePaths())
			}

			// Empty NextPageToken indicates end of results list
			if tt.size != 1 && response.GetNextPageToken() != "" {
				t.Error("Expected end of results, receieved non-empty next page token", response.GetNextPageToken())
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestBackEnd_GetNvmePath(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.NvmePath
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		// "valid request with invalid SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
		// 	errCode: codes.InvalidArgument,
		// 	errMsg: fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		// },
		"valid request with invalid marshal SPDK response": {
			in:      testNvmePathID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_get_controllers: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevNvmeGetControllerResult"),
		},
		"valid request with empty SPDK response": {
			in:      testNvmePathID,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_get_controllers: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNvmePathID,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testNvmePathID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response error: myopierr"),
		},
		// "valid request with valid SPDK response": {
		// 	in: testNvmePathID,
		// 	out: &pb.NvmePath{
		// 		Name:    "Malloc1",
		// 		Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		// 		Adrfam:  pb.NvmeAddressFamily_NVME_ADRFAM_IPV4,
		// 		Traddr:  "127.0.0.1",
		// 		Trsvcid: 4444,
		// 	},
		// 	spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
		// 	errCode: codes.OK,
		// 	errMsg: "",
		// },
		"valid request with unknown key": {
			in:      "unknown-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-id"),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"no required field": {
			in:      "",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathID] = server.ProtoClone(&testNvmePath)

			request := &pb.GetNvmePathRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmePath(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestBackEnd_StatsNvmePath(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		// "valid request with invalid SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
		// 	errCode: codes.InvalidArgument,
		// 	errMsg: fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		// },
		// "valid request with invalid marshal SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevGetIostatResult"),
		// },
		// "valid request with empty SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{""},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
		// },
		// "valid request with ID mismatch SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
		// },
		// "valid request with error code from SPDK response": {
		// 	in: testNvmePathID,
		// 	out: nil,
		// 	spdk: []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
		// },
		// "valid request with valid SPDK response": {
		// 	in: testNvmePathID,
		// 	out: &pb.VolumeStats{
		// 		ReadBytesCount:    1,
		// 		ReadOpsCount:      2,
		// 		WriteBytesCount:   3,
		// 		WriteOpsCount:     4,
		// 		ReadLatencyTicks:  7,
		// 		WriteLatencyTicks: 8,
		// 	},
		// 	spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"ticks":18787040917434338,"bdevs":[{"name":"mytest","bytes_read":1,"num_read_ops":2,"bytes_written":3,"num_write_ops":4,"bytes_unmapped":0,"num_unmap_ops":0,"read_latency_ticks":7,"write_latency_ticks":8,"unmap_latency_ticks":0}]}}`},
		// 	errCode: codes.OK,
		// 	errMsg: "",
		// },
		"valid request with unknown key": {
			in:      "unknown-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-id"),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathID] = server.ProtoClone(&testNvmePath)
			testEnv.opiSpdkServer.Volumes.NvmePaths[testNvmePathID].Name = testNvmePathName

			request := &pb.StatsNvmePathRequest{Name: tt.in}
			response, err := testEnv.client.StatsNvmePath(testEnv.ctx, request)

			if !proto.Equal(response.GetStats(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetStats())
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}
