// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

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
)

var (
	testNullVolume = pb.NullDebug{
		Handle:      &pc.ObjectKey{Value: "mytest"},
		BlockSize:   512,
		BlocksCount: 64,
	}
)

func TestBackEnd_CreateNullDebug(t *testing.T) {
	tests := []struct {
		name    string
		in      *pb.NullDebug
		out     *pb.NullDebug
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			&testNullVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Null Dev: %v", "mytest"),
			true,
		},
		{
			"valid request with empty SPDK response",
			&testNullVolume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_null_create: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			&testNullVolume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_null_create: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			&testNullVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_null_create: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			&testNullVolume,
			&testNullVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.CreateNullDebugRequest{NullDebug: tt.in}
			response, err := testEnv.client.CreateNullDebug(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_UpdateNullDebug(_ *testing.T) {

}

func TestBackEnd_ListNullDebugs(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     []*pb.NullDebug
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find any namespaces for NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []models.BdevGetBdevsResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"volume-test",
			[]*pb.NullDebug{
				{
					Handle:      &pc.ObjectKey{Value: "Malloc0"},
					Uuid:        &pc.Uuid{Value: "11d3902e-d9bb-49a7-bb27-cd7261ef3217"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
				{
					Handle:      &pc.ObjectKey{Value: "Malloc1"},
					Uuid:        &pc.Uuid{Value: "88112c76-8c49-4395-955a-0d695b1d2099"},
					BlockSize:   512,
					BlocksCount: 131072,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.ListNullDebugsRequest{Parent: tt.in}
			response, err := testEnv.client.ListNullDebugs(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NullDebugs, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NullDebugs)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_GetNullDebug(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.NullDebug
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []models.BdevGetBdevsResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"volume-test",
			&pb.NullDebug{
				Handle:      &pc.ObjectKey{Value: "Malloc1"},
				Uuid:        &pc.Uuid{Value: "88112c76-8c49-4395-955a-0d695b1d2099"},
				BlockSize:   512,
				BlocksCount: 131072,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.GetNullDebugRequest{Name: tt.in}
			response, err := testEnv.client.GetNullDebug(testEnv.ctx, request)
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
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NullDebugStats(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"mytest",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"mytest",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type models.BdevGetIostatResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"mytest",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"mytest",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"mytest",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"Malloc0",
			&pb.VolumeStats{
				ReadBytesCount:    1,
				ReadOpsCount:      2,
				WriteBytesCount:   3,
				WriteOpsCount:     4,
				ReadLatencyTicks:  7,
				WriteLatencyTicks: 8,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"ticks":18787040917434338,"bdevs":[{"name":"Malloc0","bytes_read":1,"num_read_ops":2,"bytes_written":3,"num_write_ops":4,"bytes_unmapped":0,"num_unmap_ops":0,"read_latency_ticks":7,"write_latency_ticks":8,"unmap_latency_ticks":0}]}}`},
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.NullDebugStatsRequest{Handle: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NullDebugStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_DeleteNullDebug(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"mytest",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN:ID %v", "nqn.2022-09.io.spdk:opi3:17"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"mytest",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_null_delete: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"mytest",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_null_delete: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"mytest",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_null_delete: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"mytest",
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`}, // `{"jsonrpc": "2.0", "id": 1, "result": True}`,
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NullVolumes[testNullVolume.Handle.Value] = &testNullVolume

			request := &pb.DeleteNullDebugRequest{Name: tt.in}
			response, err := testEnv.client.DeleteNullDebug(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
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
