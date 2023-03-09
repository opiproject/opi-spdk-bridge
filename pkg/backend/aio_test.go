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

func TestBackEnd_CreateAioController(t *testing.T) {
	volume := &pb.AioController{
		Handle:      &pc.ObjectKey{Value: "mytest"},
		BlockSize:   512,
		BlocksCount: 12,
		Filename:    "/tmp/aio_bdev_file",
	}
	tests := []struct {
		name    string
		in      *pb.AioController
		out     *pb.AioController
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
			fmt.Sprintf("Could not create Aio Dev: %v", "mytest"),
			true,
		},
		{
			"valid request with empty SPDK response",
			volume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			volume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			volume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_aio_create: %v", "json response error: myopierr"),
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

			request := &pb.CreateAioControllerRequest{AioController: tt.in}
			response, err := testEnv.client.CreateAioController(testEnv.ctx, request)
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

func TestBackEnd_DeleteAioController(t *testing.T) {

}

func TestBackEnd_UpdateAioController(t *testing.T) {

}

func TestBackEnd_ListAioControllers(t *testing.T) {

}

func TestBackEnd_GetAioController(t *testing.T) {

}

func TestBackEnd_AioControllerStats(t *testing.T) {

}
