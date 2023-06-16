// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testNamespaceID   = "namespace-test"
	testNamespaceName = server.ResourceIDToVolumeName(testNamespaceID)
	testNamespace     = pb.NvmeNamespace{
		Spec: &pb.NvmeNamespaceSpec{
			HostNsid:    22,
			SubsystemId: &pc.ObjectKey{Value: testSubsystemName},
		},
		Status: &pb.NvmeNamespaceStatus{
			PciState:     2,
			PciOperState: 1,
		},
	}
)

func TestFrontEnd_CreateNvmeNamespace(t *testing.T) {
	spec := &pb.NvmeNamespaceSpec{
		SubsystemId: &pc.ObjectKey{Value: testSubsystemName},
		HostNsid:    0,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	namespaceSpec := &pb.NvmeNamespaceSpec{
		SubsystemId: &pc.ObjectKey{Value: testSubsystemName},
		HostNsid:    22,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	tests := map[string]struct {
		id      string
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
			false,
		},
		"valid request with invalid SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":-1}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NS: %v", testNamespaceName),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_ns: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":-1}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_ns: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":-1}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_add_ns: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Spec: namespaceSpec,
			},
			&pb.NvmeNamespace{
				Spec: namespaceSpec,
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":22}`},
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Spec: spec,
			},
			&testNamespace,
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

			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController
			if tt.exist {
				testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = &testNamespace
			}
			if tt.out != nil {
				tt.out.Name = testNamespaceName
			}

			request := &pb.CreateNvmeNamespaceRequest{NvmeNamespace: tt.in, NvmeNamespaceId: tt.id}
			response, err := testEnv.client.CreateNvmeNamespace(testEnv.ctx, request)
			if response != nil {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
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

func TestFrontEnd_DeleteNvmeNamespace(t *testing.T) {
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
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NS: %v", testNamespaceName),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testNamespaceName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_ns: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_ns: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_subsystem_remove_ns: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testNamespaceName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-namespace-id"),
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-namespace-id")),
			false,
			false,
		},
		"unknown key with missing allowed": {
			server.ResourceIDToVolumeName("unknown-id"),
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
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.DeleteNvmeNamespaceRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeNamespace(testEnv.ctx, request)
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

func TestFrontEnd_UpdateNvmeNamespace(t *testing.T) {
	spec := &pb.NvmeNamespaceSpec{
		SubsystemId: &pc.ObjectKey{Value: testSubsystemName},
		HostNsid:    22,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
			false,
		},
		"valid request without SPDK": {
			nil,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{""},
			codes.OK,
			"",
			false,
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeNamespace{
				Name: server.ResourceIDToVolumeName("unknown-id"),
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
			false,
		},
		"unknown key with missing allowed": {
			nil,
			&pb.NvmeNamespace{
				Name: server.ResourceIDToVolumeName("unknown-id"),
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.UpdateNvmeNamespaceRequest{NvmeNamespace: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateNvmeNamespace(testEnv.ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out.Spec)
				mResponse, _ := proto.Marshal(response.Spec)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
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

func TestFrontEnd_ListNvmeNamespaces(t *testing.T) {
	testNamespaces := []pb.NvmeNamespace{
		{
			Spec: &pb.NvmeNamespaceSpec{
				HostNsid: 11,
			},
		},
		{
			Spec: &pb.NvmeNamespaceSpec{
				HostNsid: 12,
			},
		},
		{
			Spec: &pb.NvmeNamespaceSpec{
				HostNsid: 13,
			},
		},
	}
	tests := map[string]struct {
		in      string
		out     []*pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find any namespaces for NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json: cannot unmarshal bool into Go value of type []spdk.NvmfGetSubsystemsResult"),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			[]*pb.NvmeNamespace{
				&testNamespaces[0],
				&testNamespaces[1],
				&testNamespaces[2],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},` +
				`{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[` +
				`{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},` +
				`{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},` +
				`{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}` +
				`]}]}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
		"pagination overflow": {
			testSubsystemName,
			[]*pb.NvmeNamespace{
				&testNamespaces[0],
				&testNamespaces[1],
				&testNamespaces[2],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
			1000,
			"",
		},
		"pagination negative": {
			testSubsystemName,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			testSubsystemName,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			testSubsystemName,
			[]*pb.NvmeNamespace{
				&testNamespaces[0],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination offset": {
			testSubsystemName,
			[]*pb.NvmeNamespace{
				&testNamespaces[1],
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":11,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":12,"bdev_name":"Malloc1","name":"Malloc1","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"},{"nsid":13,"bdev_name":"Malloc2","name":"Malloc2","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find subsystem %v", "unknown-namespace-id"),
			false,
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces[server.ResourceIDToVolumeName("ns0")] = &testNamespaces[0]
			testEnv.opiSpdkServer.Nvme.Namespaces[server.ResourceIDToVolumeName("ns1")] = &testNamespaces[1]
			testEnv.opiSpdkServer.Nvme.Namespaces[server.ResourceIDToVolumeName("ns2")] = &testNamespaces[2]
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeNamespacesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeNamespaces(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvmeNamespaces, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvmeNamespaces)
				}
				// Empty NextPageToken indicates end of results list
				if tt.size != 1 && response.NextPageToken != "" {
					t.Error("Expected end of results, receieved non-empty next page token", response.NextPageToken)
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

func TestFrontEnd_GetNvmeNamespace(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json: cannot unmarshal bool into Go value of type []spdk.NvmfGetSubsystemsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testNamespaceName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("nvmf_get_subsystems: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testNamespaceName,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: &pb.NvmeNamespaceSpec{
					HostNsid: 22,
				},
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"nqn":"nqn.2014-08.org.nvmexpress.discovery","subtype":"Discovery","listen_addresses":[],"allow_any_host":true,"hosts":[]},{"nqn":"nqn.2022-09.io.spdk:opi3","subtype":"Nvme","listen_addresses":[{"transport":"TCP","trtype":"TCP","adrfam":"IPv4","traddr":"192.168.80.2","trsvcid":"4444"}],"allow_any_host":false,"hosts":[{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c"}],"serial_number":"SPDK00000000000001","model_number":"SPDK_Controller1","max_namespaces":32,"min_cntlid":1,"max_cntlid":65519,"namespaces":[{"nsid":22,"bdev_name":"Malloc0","name":"Malloc0","nguid":"611C13802D994E1DAB121F38A9887929","uuid":"611c1380-2d99-4e1d-ab12-1f38a9887929"}]}]}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()
			testEnv.opiSpdkServer.Nvme.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Nvme.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.GetNvmeNamespaceRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeNamespace(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Spec, tt.out.Spec) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
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

func TestFrontEnd_NvmeNamespaceStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with valid SPDK response": {
			testNamespaceName,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{""},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-id"),
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Nvme.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.NvmeNamespaceStatsRequest{NamespaceId: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeNamespaceStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
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
