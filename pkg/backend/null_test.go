// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"bytes"
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func TestBackEnd_CreateNullDebug(t *testing.T) {
	volume := &pb.NullDebug{
		Handle:      &pc.ObjectKey{Value: "mytest"},
		BlockSize:   512,
		BlocksCount: 64,
	}
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
			volume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Null Dev: %v", "mytest"),
			true,
		},
		{
			"valid request with empty SPDK response",
			volume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_null_create: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			volume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_null_create: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			volume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_null_create: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			volume,
			volume,
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

func TestBackEnd_DeleteNullDebug(t *testing.T) {

}

func TestBackEnd_UpdateNullDebug(t *testing.T) {

}

func TestBackEnd_ListNullDebugs(t *testing.T) {

}

func TestBackEnd_GetNullDebug(t *testing.T) {

}

func TestBackEnd_NullDebugStats(t *testing.T) {

}
