// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

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

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

var (
	testNullVolumeID   = "mytest"
	testNullVolumeName = utils.ResourceIDToVolumeName(testNullVolumeID)
	testNullVolume     = pb.NullVolume{
		BlockSize:   512,
		BlocksCount: 64,
	}
)

func TestBackEnd_CreateNullVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		id      string
		in      *pb.NullVolume
		out     *pb.NullVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			id:      "CapitalLettersNotAllowed",
			in:      &testNullVolume,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			exist:   false,
		},
		"valid request with invalid SPDK response": {
			id:      testNullVolumeID,
			in:      &testNullVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Null Dev: %v", testNullVolumeID),
			exist:   false,
		},
		"valid request with empty SPDK response": {
			id:      testNullVolumeID,
			in:      &testNullVolume,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_create: %v", "EOF"),
			exist:   false,
		},
		"valid request with ID mismatch SPDK response": {
			id:      testNullVolumeID,
			in:      &testNullVolume,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_create: %v", "json response ID mismatch"),
			exist:   false,
		},
		"valid request with error code from SPDK response": {
			id:      testNullVolumeID,
			in:      &testNullVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_create: %v", "json response error: myopierr"),
			exist:   false,
		},
		"valid request with valid SPDK response": {
			id:      testNullVolumeID,
			in:      &testNullVolume,
			out:     &testNullVolume,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
		},
		"already exists": {
			id:      testNullVolumeID,
			in:      &testNullVolume,
			out:     &testNullVolume,
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
			errMsg:  "missing required field: null_volume",
			exist:   false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeName] = utils.ProtoClone(&testNullVolume)
				testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeName].Name = testNullVolumeName
			}
			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
				tt.out.Name = testNullVolumeName
			}

			request := &pb.CreateNullVolumeRequest{NullVolume: tt.in, NullVolumeId: tt.id}
			response, err := testEnv.client.CreateNullVolume(testEnv.ctx, request)

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

func TestBackEnd_UpdateNullVolume(t *testing.T) {
	testNullVolumeWithName := utils.ProtoClone(&testNullVolume)
	testNullVolumeWithName.Name = testNullVolumeName
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(testNullVolumeWithName)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NullVolume
		out     *pb.NullVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"invalid fieldmask": {
			mask:    &fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			missing: false,
		},
		"delete fails": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete Null Dev: %s", testNullVolumeID),
			missing: false,
		},
		"delete empty": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_delete: %v", "EOF"),
			missing: false,
		},
		"delete ID mismatch": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_delete: %v", "json response ID mismatch"),
			missing: false,
		},
		"delete exception": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_delete: %v", "json response error: myopierr"),
			missing: false,
		},
		"delete ok create fails": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Null Dev: %v", "mytest"),
			missing: false,
		},
		"delete ok create empty": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_create: %v", "EOF"),
			missing: false,
		},
		"delete ok create ID mismatch": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_create: %v", "json response ID mismatch"),
			missing: false,
		},
		"delete ok create exception": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_create: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			mask:    nil,
			in:      testNullVolumeWithName,
			out:     testNullVolumeWithName,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			mask: nil,
			in: &pb.NullVolume{
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
			in: &pb.NullVolume{
				Name:        utils.ResourceIDToVolumeName("unknown-id"),
				BlockSize:   512,
				BlocksCount: 64,
			},
			out: &pb.NullVolume{
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
			in: &pb.NullVolume{
				Name:        "-ABC-DEF",
				BlockSize:   testNullVolume.BlockSize,
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

			testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeName] = utils.ProtoClone(&testNullVolume)
			testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeName].Name = testNullVolumeName

			request := &pb.UpdateNullVolumeRequest{NullVolume: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateNullVolume(testEnv.ctx, request)

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

func TestBackEnd_ListNullVolumes(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     []*pb.NullVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with empty result SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"valid request with invalid marshal SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
			size:    0,
			token:   "",
		},
		"valid request with empty SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			size:    0,
			token:   "",
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			size:    0,
			token:   "",
		},
		"valid request with error code from SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			size:    0,
			token:   "",
		},
		"valid request with valid SPDK response": {
			in: testNullVolumeID,
			out: []*pb.NullVolume{
				{
					Name:        "Malloc0",
					Uuid:        &pc.Uuid{Value: "11d3902e-d9bb-49a7-bb27-cd7261ef3217"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
				{
					Name:        "Malloc1",
					Uuid:        &pc.Uuid{Value: "88112c76-8c49-4395-955a-0d695b1d2099"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},` +
				`{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}` +
				`]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"pagination overflow": {
			in: testNullVolumeID,
			out: []*pb.NullVolume{
				{
					Name:        "Malloc0",
					Uuid:        &pc.Uuid{Value: "11d3902e-d9bb-49a7-bb27-cd7261ef3217"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
				{
					Name:        "Malloc1",
					Uuid:        &pc.Uuid{Value: "88112c76-8c49-4395-955a-0d695b1d2099"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1000,
			token:   "",
		},
		"pagination negative": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "negative PageSize is not allowed",
			size:    -10,
			token:   "",
		},
		"pagination error": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			size:    0,
			token:   "unknown-pagination-token",
		},
		"pagination": {
			in: testNullVolumeID,
			out: []*pb.NullVolume{
				{
					Name:        "Malloc0",
					Uuid:        &pc.Uuid{Value: "11d3902e-d9bb-49a7-bb27-cd7261ef3217"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
		},
		"pagination offset": {
			in: testNullVolumeID,
			out: []*pb.NullVolume{
				{
					Name:        "Malloc1",
					Uuid:        &pc.Uuid{Value: "88112c76-8c49-4395-955a-0d695b1d2099"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "existing-pagination-token",
		},
		"no required field": {
			in:      "",
			out:     []*pb.NullVolume{},
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

			request := &pb.ListNullVolumesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNullVolumes(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetNullVolumes(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNullVolumes())
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

func TestBackEnd_GetNullVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.NullVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
		},
		"valid request with empty SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testNullVolumeID,
			out: &pb.NullVolume{
				Name:        "Malloc1",
				Uuid:        &pc.Uuid{Value: "88112c76-8c49-4395-955a-0d695b1d2099"},
				BlockSize:   512,
				BlocksCount: 131072,
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
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

			testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeID] = utils.ProtoClone(&testNullVolume)

			request := &pb.GetNullVolumeRequest{Name: tt.in}
			response, err := testEnv.client.GetNullVolume(testEnv.ctx, request)

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

func TestBackEnd_StatsNullVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevGetIostatResult"),
		},
		"valid request with empty SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testNullVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testNullVolumeID,
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

			testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeID] = utils.ProtoClone(&testNullVolume)

			request := &pb.StatsNullVolumeRequest{Name: tt.in}
			response, err := testEnv.client.StatsNullVolume(testEnv.ctx, request)

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

func TestBackEnd_DeleteNullVolume(t *testing.T) {
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
			in:      testNullVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete Null Dev: %s", testNullVolumeID),
			missing: false,
		},
		"valid request with empty SPDK response": {
			in:      testNullVolumeName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_delete: %v", "EOF"),
			missing: false,
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNullVolumeName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_delete: %v", "json response ID mismatch"),
			missing: false,
		},
		"valid request with error code from SPDK response": {
			in:      testNullVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_null_delete: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			in:      testNullVolumeName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
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

			testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeName] = utils.ProtoClone(&testNullVolume)
			testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolumeName].Name = testNullVolumeName

			request := &pb.DeleteNullVolumeRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNullVolume(testEnv.ctx, request)

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
