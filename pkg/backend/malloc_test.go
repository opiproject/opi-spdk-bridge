// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2024 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation
// Copyright (c) 2024 Xsight Labs Inc

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var (
	testMallocVolumeID   = "mytest"
	testMallocVolumeName = utils.ResourceIDToVolumeName(testMallocVolumeID)
	testMallocVolume     = pb.MallocVolume{
		BlockSize:   512,
		BlocksCount: 64,
	}
	testMallocVolumeWithName = pb.MallocVolume{
		Name:        testMallocVolumeName,
		BlockSize:   testMallocVolume.BlockSize,
		BlocksCount: testMallocVolume.BlocksCount,
	}
	testRPCHdr      = `"jsonrpc":"2.0","id":%d,"result"`
	testBdevMalloc0 = `{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}`
	testBdevMalloc1 = `{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}`
)

func TestBackEnd_CreateMallocVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		id      string
		in      *pb.MallocVolume
		out     *pb.MallocVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			id:      "CapitalLettersNotAllowed",
			in:      &testMallocVolume,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			exist:   false,
		},
		"valid request with invalid SPDK response": {
			id:      testMallocVolumeID,
			in:      &testMallocVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Malloc Dev: %v", testMallocVolumeID),
			exist:   false,
		},
		"valid request with empty SPDK response": {
			id:      testMallocVolumeID,
			in:      &testMallocVolume,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_create: %v", "EOF"),
			exist:   false,
		},
		"valid request with ID mismatch SPDK response": {
			id:      testMallocVolumeID,
			in:      &testMallocVolume,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_create: %v", "json response ID mismatch"),
			exist:   false,
		},
		"valid request with error code from SPDK response": {
			id:      testMallocVolumeID,
			in:      &testMallocVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_create: %v", "json response error: myopierr"),
			exist:   false,
		},
		"valid request with valid SPDK response": {
			id:      testMallocVolumeID,
			in:      &testMallocVolume,
			out:     &testMallocVolume,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
		},
		"already exists": {
			id:      testMallocVolumeID,
			in:      &testMallocVolume,
			out:     &testMallocVolume,
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
			errMsg:  "missing required field: malloc_volume",
			exist:   false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.Volumes.MallocVolumes[testMallocVolumeName] = utils.ProtoClone(&testMallocVolumeWithName)
			}
			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
				tt.out.Name = testMallocVolumeName
			}

			request := &pb.CreateMallocVolumeRequest{MallocVolume: tt.in, MallocVolumeId: tt.id}
			response, err := testEnv.client.CreateMallocVolume(testEnv.ctx, request)

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

func TestBackEnd_UpdateMallocVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.MallocVolume
		out     *pb.MallocVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"invalid fieldmask": {
			mask:    &fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			missing: false,
		},
		"delete fails": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete Malloc Dev: %s", testMallocVolumeID),
			missing: false,
		},
		"delete empty": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_delete: %v", "EOF"),
			missing: false,
		},
		"delete ID mismatch": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_delete: %v", "json response ID mismatch"),
			missing: false,
		},
		"delete exception": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_delete: %v", "json response error: myopierr"),
			missing: false,
		},
		"delete ok create fails": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Malloc Dev: %v", "mytest"),
			missing: false,
		},
		"delete ok create empty": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_create: %v", "EOF"),
			missing: false,
		},
		"delete ok create ID mismatch": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_create: %v", "json response ID mismatch"),
			missing: false,
		},
		"delete ok create exception": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_create: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			mask:    nil,
			in:      &testMallocVolumeWithName,
			out:     &testMallocVolumeWithName,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			mask: nil,
			in: &pb.MallocVolume{
				Name:        utils.ResourceIDToVolumeName("unknown-id"),
				BlockSize:   512,
				BlocksCount: 64,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			missing: false,
		},
		"unknown key with missing allowed": {
			mask: nil,
			in: &pb.MallocVolume{
				Name:        utils.ResourceIDToVolumeName("unknown-id"),
				BlockSize:   512,
				BlocksCount: 64,
			},
			out: &pb.MallocVolume{
				Name:        utils.ResourceIDToVolumeName("unknown-id"),
				BlockSize:   512,
				BlocksCount: 64,
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: true,
		},
		"malformed name": {
			mask: nil,
			in: &pb.MallocVolume{
				Name:        "-ABC-DEF",
				BlockSize:   testMallocVolume.BlockSize,
				BlocksCount: testAioVolume.BlocksCount,
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

			testEnv.opiSpdkServer.Volumes.MallocVolumes[testMallocVolumeName] = utils.ProtoClone(&testMallocVolumeWithName)

			request := &pb.UpdateMallocVolumeRequest{MallocVolume: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateMallocVolume(testEnv.ctx, request)

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

func TestBackEnd_ListMallocVolumes(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     []*pb.MallocVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with empty result SPDK response": {
			in:      testMallocVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"valid request with invalid marshal SPDK response": {
			in:      testMallocVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
			size:    0,
			token:   "",
		},
		"valid request with empty SPDK response": {
			in:      testMallocVolumeID,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			size:    0,
			token:   "",
		},
		"valid request with ID mismatch SPDK response": {
			in:      testMallocVolumeID,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			size:    0,
			token:   "",
		},
		"valid request with error code from SPDK response": {
			in:      testMallocVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			size:    0,
			token:   "",
		},
		"valid request with valid SPDK response": {
			in: testMallocVolumeID,
			out: []*pb.MallocVolume{
				{
					Name:        "Malloc0",
					Uuid:        "11d3902e-d9bb-49a7-bb27-cd7261ef3217",
					BlockSize:   512,
					BlocksCount: 131072,
				},
				{
					Name:        "Malloc1",
					Uuid:        "88112c76-8c49-4395-955a-0d695b1d2099",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk:    []string{`{` + testRPCHdr + `:[` + testBdevMalloc1 + `,` + testBdevMalloc0 + `]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"pagination overflow": {
			in: testMallocVolumeID,
			out: []*pb.MallocVolume{
				{
					Name:        "Malloc0",
					Uuid:        "11d3902e-d9bb-49a7-bb27-cd7261ef3217",
					BlockSize:   512,
					BlocksCount: 131072,
				},
				{
					Name:        "Malloc1",
					Uuid:        "88112c76-8c49-4395-955a-0d695b1d2099",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk:    []string{`{` + testRPCHdr + `:[` + testBdevMalloc0 + `,` + testBdevMalloc1 + `]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1000,
			token:   "",
		},
		"pagination negative": {
			in:      testMallocVolumeID,
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "negative PageSize is not allowed",
			size:    -10,
			token:   "",
		},
		"pagination error": {
			in:      testMallocVolumeID,
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			size:    0,
			token:   "unknown-pagination-token",
		},
		"pagination": {
			in: testMallocVolumeID,
			out: []*pb.MallocVolume{
				{
					Name:        "Malloc0",
					Uuid:        "11d3902e-d9bb-49a7-bb27-cd7261ef3217",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk:    []string{`{` + testRPCHdr + `:[` + testBdevMalloc0 + `,` + testBdevMalloc1 + `]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
		},
		"pagination offset": {
			in: testMallocVolumeID,
			out: []*pb.MallocVolume{
				{
					Name:        "Malloc1",
					Uuid:        "88112c76-8c49-4395-955a-0d695b1d2099",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk:    []string{`{` + testRPCHdr + `:[` + testBdevMalloc0 + `,` + testBdevMalloc1 + `]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "existing-pagination-token",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListMallocVolumesRequest{PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListMallocVolumes(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetMallocVolumes(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetMallocVolumes())
			}

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

func TestBackEnd_GetMallocVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.MallocVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
		},
		"valid request with empty SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testMallocVolumeName,
			out: &pb.MallocVolume{
				Name:        "Malloc1",
				Uuid:        "88112c76-8c49-4395-955a-0d695b1d2099",
				BlockSize:   512,
				BlocksCount: 131072,
			},
			spdk:    []string{`{` + testRPCHdr + `:[` + testBdevMalloc1 + `]}`},
			errCode: codes.OK,
			errMsg:  "",
		},
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

			testEnv.opiSpdkServer.Volumes.MallocVolumes[testMallocVolumeName] = utils.ProtoClone(&testMallocVolumeWithName)

			request := &pb.GetMallocVolumeRequest{Name: tt.in}
			response, err := testEnv.client.GetMallocVolume(testEnv.ctx, request)

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

func TestBackEnd_StatsMallocVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevGetIostatResult"),
		},
		"valid request with empty SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testMallocVolumeName,
			out: &pb.VolumeStats{
				ReadBytesCount:    1,
				ReadOpsCount:      2,
				WriteBytesCount:   3,
				WriteOpsCount:     4,
				ReadLatencyTicks:  7,
				WriteLatencyTicks: 8,
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"ticks":18787040917434338,"bdevs":[{"name":"mytest","bytes_read":1,"num_read_ops":2,"bytes_written":3,"num_write_ops":4,"bytes_unmapped":0,"num_unmap_ops":0,"read_latency_ticks":7,"write_latency_ticks":8,"unmap_latency_ticks":0}]}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
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

			testEnv.opiSpdkServer.Volumes.MallocVolumes[testMallocVolumeName] = utils.ProtoClone(&testMallocVolumeWithName)

			request := &pb.StatsMallocVolumeRequest{Name: tt.in}
			response, err := testEnv.client.StatsMallocVolume(testEnv.ctx, request)

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

func TestBackEnd_DeleteMallocVolume(t *testing.T) {
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
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete Malloc Dev: %s", testMallocVolumeID),
			missing: false,
		},
		"valid request with empty SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_delete: %v", "EOF"),
			missing: false,
		},
		"valid request with ID mismatch SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_delete: %v", "json response ID mismatch"),
			missing: false,
		},
		"valid request with error code from SPDK response": {
			in:      testMallocVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_malloc_delete: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			in:      testMallocVolumeName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			in:      utils.ResourceIDToVolumeName("unknown-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
			missing: false,
		},
		"unknown key with missing allowed": {
			in:      utils.ResourceIDToVolumeName("unknown-id"),
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			missing: true,
		},
		"malformed name": {
			in:      utils.ResourceIDToVolumeName("-ABC-DEF"),
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

			testEnv.opiSpdkServer.Volumes.MallocVolumes[testMallocVolumeName] = utils.ProtoClone(&testMallocVolumeWithName)

			request := &pb.DeleteMallocVolumeRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteMallocVolume(testEnv.ctx, request)

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
