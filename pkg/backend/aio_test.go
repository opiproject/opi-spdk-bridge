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

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	testAioVolumeID   = "mytest"
	testAioVolumeName = server.ResourceIDToVolumeName(testAioVolumeID)
	testAioVolume     = pb.AioVolume{
		BlockSize:   512,
		BlocksCount: 12,
		Filename:    "/tmp/aio_bdev_file",
	}
)

func TestBackEnd_CreateAioVolume(t *testing.T) {
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(t, t.Name(), &testAioVolume))
	tests := map[string]struct {
		id      string
		in      *pb.AioVolume
		out     *pb.AioVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&testAioVolume,
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
		},
		"valid request with invalid SPDK response": {
			testAioVolumeID,
			&testAioVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Aio Dev: %v", testAioVolumeID),
			false,
		},
		"valid request with empty SPDK response": {
			testAioVolumeID,
			&testAioVolume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testAioVolumeID,
			&testAioVolume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testAioVolumeID,
			&testAioVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			testAioVolumeID,
			&testAioVolume,
			&testAioVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			codes.OK,
			"",
			false,
		},
		"already exists": {
			testAioVolumeID,
			&testAioVolume,
			&testAioVolume,
			[]string{},
			codes.OK,
			"",
			true,
		},
		"no required field": {
			testAioVolumeID,
			nil,
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: aio_volume",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeName] = server.ProtoClone(&testAioVolume)
				testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeName].Name = testAioVolumeName
			}
			if tt.out != nil {
				tt.out = server.ProtoClone(tt.out)
				tt.out.Name = testAioVolumeName
			}

			request := &pb.CreateAioVolumeRequest{AioVolume: tt.in, AioVolumeId: tt.id}
			response, err := testEnv.client.CreateAioVolume(testEnv.ctx, request)

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

func TestBackEnd_UpdateAioVolume(t *testing.T) {
	testAioVolumeWithName := server.ProtoClone(&testAioVolume)
	testAioVolumeWithName.Name = testAioVolumeName
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(t, t.Name(), &testAioVolume, testAioVolumeWithName))
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.AioVolume
		out     *pb.AioVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			testAioVolumeWithName,
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
		},
		"delete fails": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Aio Dev: %s", testAioVolumeID),
			false,
		},
		"delete empty": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_delete: %v", "EOF"),
			false,
		},
		"delete ID mismatch": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_delete: %v", "json response ID mismatch"),
			false,
		},
		"delete exception": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_delete: %v", "json response error: myopierr"),
			false,
		},
		"delete ok create fails": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Aio Dev: %v", testAioVolumeID),
			false,
		},
		"delete ok create empty": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "EOF"),
			false,
		},
		"delete ok create ID mismatch": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "json response ID mismatch"),
			false,
		},
		"delete ok create exception": {
			nil,
			testAioVolumeWithName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			nil,
			testAioVolumeWithName,
			testAioVolumeWithName,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.AioVolume{
				Name:        server.ResourceIDToVolumeName("unknown-id"),
				BlockSize:   512,
				BlocksCount: 12,
				Filename:    "/tmp/aio_bdev_file",
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
		},
		"unknown key with missing allowed": {
			nil,
			&pb.AioVolume{
				Name:        server.ResourceIDToVolumeName("unknown-id"),
				BlockSize:   512,
				BlocksCount: 12,
				Filename:    "/tmp/aio_bdev_file",
			},
			&pb.AioVolume{
				Name:        server.ResourceIDToVolumeName("unknown-id"),
				BlockSize:   512,
				BlocksCount: 12,
				Filename:    "/tmp/aio_bdev_file",
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			codes.OK,
			"",
			true,
		},
		"malformed name": {
			nil,
			&pb.AioVolume{Name: "-ABC-DEF"},
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

			testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeName] = server.ProtoClone(&testAioVolume)
			testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeName].Name = testAioVolumeName

			request := &pb.UpdateAioVolumeRequest{AioVolume: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateAioVolume(testEnv.ctx, request)

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

func TestBackEnd_ListAioVolumes(t *testing.T) {
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(t, t.Name(), &testAioVolume))
	tests := map[string]struct {
		in      string
		out     []*pb.AioVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with empty result SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.OK,
			"",
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testAioVolumeID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			0,
			"",
		},
		"valid request with valid SPDK response": {
			testAioVolumeID,
			[]*pb.AioVolume{
				{
					Name:        "Malloc0",
					BlockSize:   512,
					BlocksCount: 131072,
				},
				{
					Name:        "Malloc1",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},` +
				`{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}` +
				`]}`},
			codes.OK,
			"",
			0,
			"",
		},
		"pagination overflow": {
			testAioVolumeID,
			[]*pb.AioVolume{
				{
					Name:        "Malloc0",
					BlockSize:   512,
					BlocksCount: 131072,
				},
				{
					Name:        "Malloc1",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			1000,
			"",
		},
		"pagination negative": {
			testAioVolumeID,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			-10,
			"",
		},
		"pagination error": {
			testAioVolumeID,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			testAioVolumeID,
			[]*pb.AioVolume{
				{
					Name:        "Malloc0",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			1,
			"",
		},
		"pagination offset": {
			testAioVolumeID,
			[]*pb.AioVolume{
				{
					Name:        "Malloc1",
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			1,
			"existing-pagination-token",
		},
		"no required field": {
			"",
			[]*pb.AioVolume{},
			[]string{},
			codes.Unknown,
			"missing required field: parent",
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListAioVolumesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListAioVolumes(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetAioVolumes(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetAioVolumes())
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

func TestBackEnd_GetAioVolume(t *testing.T) {
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(t, t.Name(), &testAioVolume))
	tests := map[string]struct {
		in      string
		out     *pb.AioVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
		},
		"valid request with empty SPDK response": {
			testAioVolumeID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testAioVolumeID,
			&pb.AioVolume{Name: "Malloc1", BlockSize: 512, BlocksCount: 131072},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
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

			testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeID] = server.ProtoClone(&testAioVolume)

			request := &pb.GetAioVolumeRequest{Name: tt.in}
			response, err := testEnv.client.GetAioVolume(testEnv.ctx, request)

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

func TestBackEnd_StatsAioVolume(t *testing.T) {
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(t, t.Name(), &testAioVolume))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevGetIostatResult"),
		},
		"valid request with empty SPDK response": {
			testAioVolumeID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testAioVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testAioVolumeID,
			&pb.VolumeStats{
				ReadBytesCount:    1,
				ReadOpsCount:      2,
				WriteBytesCount:   3,
				WriteOpsCount:     4,
				ReadLatencyTicks:  7,
				WriteLatencyTicks: 8,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"ticks":18787040917434338,"bdevs":[{"name":"mytest","bytes_read":1,"num_read_ops":2,"bytes_written":3,"num_write_ops":4,"bytes_unmapped":0,"num_unmap_ops":0,"read_latency_ticks":7,"write_latency_ticks":8,"unmap_latency_ticks":0}]}}`},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
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

			testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeID] = server.ProtoClone(&testAioVolume)

			request := &pb.StatsAioVolumeRequest{Name: tt.in}
			response, err := testEnv.client.StatsAioVolume(testEnv.ctx, request)

			if !proto.Equal(tt.out, response.GetStats()) {
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

func TestBackEnd_DeleteAioVolume(t *testing.T) {
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(t, t.Name(), &testAioVolume))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testAioVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Aio Dev: %s", testAioVolumeID),
			false,
		},
		"valid request with empty SPDK response": {
			testAioVolumeName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_delete: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testAioVolumeName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_delete: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testAioVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_delete: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			testAioVolumeName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
		},
		"unknown key with missing allowed": {
			server.ResourceIDToVolumeName("unknown-id"),
			&emptypb.Empty{},
			[]string{},
			codes.OK,
			"",
			true,
		},
		"malformed name": {
			server.ResourceIDToVolumeName("-ABC-DEF"),
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

			testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeName] = server.ProtoClone(&testAioVolume)
			testEnv.opiSpdkServer.Volumes.AioVolumes[testAioVolumeName].Name = testAioVolumeName

			request := &pb.DeleteAioVolumeRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteAioVolume(testEnv.ctx, request)

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
