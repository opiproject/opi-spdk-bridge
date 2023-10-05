// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var (
	testSubsystemID   = "subsystem-test"
	testSubsystemName = utils.ResourceIDToSubsystemName(testSubsystemID)
	testSubsystem     = pb.NvmeSubsystem{
		Spec: &pb.NvmeSubsystemSpec{
			Nqn: "nqn.2022-09.io.spdk:opi3",
		},
	}
)

func TestFrontEnd_CreateNvmeSubsystem(t *testing.T) {
	spec := &pb.NvmeSubsystemSpec{
		Nqn:          "nqn.2022-09.io.spdk:opi3",
		SerialNumber: "OpiSerialNumber",
		ModelNumber:  "OpiModelNumber",
	}
	specHostNqn := utils.ProtoClone(spec)
	specHostNqn.Hostnqn = "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"

	specHostNqnWithPsk := utils.ProtoClone(specHostNqn)
	specHostNqnWithPsk.Psk = []byte("NVMeTLSkey-1:01:MDAxMTIyMzM0NDU1NjY3Nzg4OTlhYWJiY2NkZGVlZmZwJEiQ:")
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(spec, specHostNqn, specHostNqnWithPsk)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		id                     string
		in                     *pb.NvmeSubsystem
		out                    *pb.NvmeSubsystem
		spdk                   []string
		errCode                codes.Code
		errMsg                 string
		exist                  bool
		stubKeyToTemporaryFile func(tmpDir string, pskKey []byte) (string, error)
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
			nil,
		},
		"valid request with invalid SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			false,
			nil,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_create_subsystem: %v", "EOF"),
			false,
			nil,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_create_subsystem: %v", "json response ID mismatch"),
			false,
			nil,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_create_subsystem: %v", "json response error: myopierr"),
			false,
			nil,
		},
		"valid request with error code from SPDK version response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("spdk_get_version: %v", "json response error: myopierr"),
			false,
			nil,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			&pb.NvmeSubsystem{
				Spec: spec,
				Status: &pb.NvmeSubsystemStatus{
					FirmwareRevision: "SPDK v20.10",
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"jsonrpc":"2.0","id":%d,"result":{"version":"SPDK v20.10","fields":{"major":20,"minor":10,"patch":0,"suffix":""}}}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			false,
			nil,
		},
		"valid request with valid SPDK response with HostNQN": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: specHostNqn,
			},
			&pb.NvmeSubsystem{
				Spec: specHostNqn,
				Status: &pb.NvmeSubsystemStatus{
					FirmwareRevision: "SPDK v20.10",
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"jsonrpc":"2.0","id":%d,"result":{"version":"SPDK v20.10","fields":{"major":20,"minor":10,"patch":0,"suffix":""}}}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			false,
			nil,
		},
		"valid request with valid SPDK response with HostNQN and PSK": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: specHostNqnWithPsk,
			},
			&pb.NvmeSubsystem{
				Spec: specHostNqnWithPsk,
				Status: &pb.NvmeSubsystemStatus{
					FirmwareRevision: "SPDK v20.10",
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"jsonrpc":"2.0","id":%d,"result":{"version":"SPDK v20.10","fields":{"major":20,"minor":10,"patch":0,"suffix":""}}}`},
			codes.OK,
			"",
			false,
			nil,
		},
		"valid request with valid SPDK response with HostNQN and PSK error": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: specHostNqnWithPsk,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			codes.Internal,
			"some psk file error",
			false,
			func(_ string, _ []byte) (string, error) {
				return "", status.Errorf(codes.Internal, "some psk file error")
			},
		},
		"already exists": {
			testSubsystemID,
			&pb.NvmeSubsystem{
				Spec: spec,
			},
			&testSubsystem,
			[]string{},
			codes.OK,
			"",
			true,
			nil,
		},
		"no required field": {
			testControllerID,
			nil,
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: nvme_subsystem",
			false,
			nil,
		},
		"too long nqn field": {
			id: testControllerID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          strings.Repeat("a", 224),
					SerialNumber: strings.Repeat("b", 20),
					ModelNumber:  strings.Repeat("c", 40),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Nqn value (%s) is too long, have to be between 1 and %d", strings.Repeat("a", 224), 223),
			exist:   false,
		},
		"too long model field": {
			id: testControllerID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          strings.Repeat("a", 223),
					SerialNumber: strings.Repeat("b", 20),
					ModelNumber:  strings.Repeat("c", 41),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("ModelNumber value (%s) is too long, have to be between 1 and %d", strings.Repeat("c", 41), 40),
			exist:   false,
		},
		"too long serial field": {
			id: testControllerID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          strings.Repeat("a", 223),
					SerialNumber: strings.Repeat("b", 21),
					ModelNumber:  strings.Repeat("c", 40),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("SerialNumber value (%s) is too long, have to be between 1 and %d", strings.Repeat("b", 21), 20),
			exist:   false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			writtenPskKey := []byte{}
			origWriteKey := testEnv.opiSpdkServer.keyToTemporaryFile
			testEnv.opiSpdkServer.keyToTemporaryFile = func(tmpDir string, pskKey []byte) (string, error) {
				writtenPskKey = pskKey
				if tt.stubKeyToTemporaryFile != nil {
					return tt.stubKeyToTemporaryFile(tmpDir, pskKey)
				}
				return origWriteKey(tmpDir, pskKey)
			}

			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = utils.ProtoClone(&testController)
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespace)
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)
				testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName].Name = testSubsystemName
			}
			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
				tt.out.Name = testSubsystemName
			}

			request := &pb.CreateNvmeSubsystemRequest{NvmeSubsystem: tt.in, NvmeSubsystemId: tt.id}
			response, err := testEnv.client.CreateNvmeSubsystem(testEnv.ctx, request)

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

			if entries, err := os.ReadDir(t.TempDir()); err != nil || len(entries) > 0 {
				t.Error("expected no tmp files exist")
			}

			if !bytes.Equal(tt.in.GetSpec().GetPsk(), writtenPskKey) {
				t.Error("expected psk key", string(tt.in.GetSpec().GetPsk()), "received", string(writtenPskKey))
			}
		})
	}
}

func TestFrontEnd_DeleteNvmeSubsystem(t *testing.T) {
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
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			false,
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_delete_subsystem: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			utils.ResourceIDToVolumeName("unknown-subsystem-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-subsystem-id")),
			false,
		},
		"unknown key with missing allowed": {
			utils.ResourceIDToVolumeName("unknown-id"),
			&emptypb.Empty{},
			[]string{},
			codes.OK,
			"",
			true,
		},
		"malformed name": {
			"-ABC-DEF",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
		"no required field": {
			"",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			"missing required field: name",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)

			request := &pb.DeleteNvmeSubsystemRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeSubsystem(testEnv.ctx, request)

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

func TestFrontEnd_UpdateNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: testSubsystem.Spec,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
		},
		"unimplemented method": {
			nil,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: testSubsystem.Spec,
			},
			nil,
			[]string{},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNvmeSubsystem"),
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeSubsystem{
				Name: utils.ResourceIDToVolumeName("unknown-id"),
				Spec: &pb.NvmeSubsystemSpec{
					Nqn: "nqn.2022-09.io.spdk:opi3",
				},
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			false,
		},
		"unknown key with missing allowed": {
			nil,
			&pb.NvmeSubsystem{
				Name: utils.ResourceIDToVolumeName("unknown-id"),
				Spec: &pb.NvmeSubsystemSpec{
					Nqn: "nqn.2022-09.io.spdk:opi3",
				},
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			true,
		},
		"malformed name": {
			nil,
			&pb.NvmeSubsystem{Name: "-ABC-DEF", Spec: testSubsystem.Spec},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)

			request := &pb.UpdateNvmeSubsystemRequest{NvmeSubsystem: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateNvmeSubsystem(testEnv.ctx, request)

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

func TestFrontEnd_ListNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	testParent := "todo"
	tests := map[string]struct {
		in      string
		out     []*pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with empty result SPDK response": {
			testParent,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.OK,
			"",
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testParent,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testParent,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testParent,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			0,
			"",
		},
		"valid request with valid SPDK response": {
			testParent,
			[]*pb.NvmeSubsystem{
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi1",
						SerialNumber: "OpiSerialNumber1",
						ModelNumber:  "OpiModelNumber1",
					},
				},
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi2",
						SerialNumber: "OpiSerialNumber2",
						ModelNumber:  "OpiModelNumber2",
					},
				},
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi3",
						SerialNumber: "OpiSerialNumber3",
						ModelNumber:  "OpiModelNumber3",
					},
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[` +
				`{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"},` +
				`{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},` +
				`{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"}` +
				`]}`},
			codes.OK,
			"",
			0,
			"",
		},
		"pagination negative": {
			testParent,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			-10,
			"",
		},
		"pagination error": {
			testParent,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			testParent,
			[]*pb.NvmeSubsystem{
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi1",
						SerialNumber: "OpiSerialNumber1",
						ModelNumber:  "OpiModelNumber1",
					},
				},
			},
			// {'jsonrpc': '2.0', 'id': 1, 'result': [{'nqn': 'nqn.2020-12.mlnx.snap', 'serial_number': 'Mellanox_Nvme_SNAP', 'model_number': 'Mellanox Nvme SNAP Controller', 'controllers': [{'name': 'NvmeEmu0pf1', 'cntlid': 0, 'pci_bdf': 'ca:00.3', 'pci_index': 1}]}]}
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"},{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"}]}`},
			codes.OK,
			"",
			1,
			"",
		},
		"pagination offset": {
			testParent,
			[]*pb.NvmeSubsystem{
				{
					Spec: &pb.NvmeSubsystemSpec{
						Nqn:          "nqn.2022-09.io.spdk:opi2",
						SerialNumber: "OpiSerialNumber2",
						ModelNumber:  "OpiModelNumber2",
					},
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"},{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"}]}`},
			codes.OK,
			"",
			1,
			"existing-pagination-token",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeSubsystemsRequest{PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeSubsystems(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetNvmeSubsystems(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNvmeSubsystems())
			}

			// Empty NextPageToken indicates end of results list
			if tt.size != 1 && response.GetNextPageToken() != "" {
				t.Error("Expected end of results, received non-empty next page token", response.GetNextPageToken())
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

func TestFrontEnd_GetNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find NQN: %v", "nqn.2022-09.io.spdk:opi3"),
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&pb.NvmeSubsystem{
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          "nqn.2022-09.io.spdk:opi3",
					SerialNumber: "OpiSerialNumber3",
					ModelNumber:  "OpiModelNumber3",
				},
				Status: &pb.NvmeSubsystemStatus{
					FirmwareRevision: "TBD",
				},
			},
			// {'jsonrpc': '2.0', 'id': 1, 'result': [{'nqn': 'nqn.2020-12.mlnx.snap', 'serial_number': 'Mellanox_Nvme_SNAP', 'model_number': 'Mellanox Nvme SNAP Controller', 'controllers': [{'name': 'NvmeEmu0pf1', 'cntlid': 0, 'pci_bdf': 'ca:00.3', 'pci_index': 1}]}]}
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[{"nqn": "nqn.2022-09.io.spdk:opi1", "serial_number": "OpiSerialNumber1", "model_number": "OpiModelNumber1"},{"nqn": "nqn.2022-09.io.spdk:opi2", "serial_number": "OpiSerialNumber2", "model_number": "OpiModelNumber2"},{"nqn": "nqn.2022-09.io.spdk:opi3", "serial_number": "OpiSerialNumber3", "model_number": "OpiModelNumber3"}]}`},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
		},
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"no required field": {
			"",
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: name",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)

			request := &pb.GetNvmeSubsystemRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeSubsystem(testEnv.ctx, request)

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

func TestFrontEnd_StatsNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid marshal SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json: cannot unmarshal array into Go value of type spdk.NvmfGetSubsystemStatsResult"),
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_stats: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"poll_groups":[{"name":"nvmf_tgt_poll_group_0","admin_qpairs":0,"io_qpairs":0,"current_admin_qpairs":0,"current_io_qpairs":0,"pending_bdev_io":0,"transports":[{"trtype":"TCP"},{"trtype":"VFIOUSER"}]}]}}`},
			codes.OK,
			"",
		},
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystem)

			request := &pb.StatsNvmeSubsystemRequest{Name: tt.in}
			response, err := testEnv.client.StatsNvmeSubsystem(testEnv.ctx, request)

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
