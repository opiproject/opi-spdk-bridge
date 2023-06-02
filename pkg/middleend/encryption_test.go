// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implememnts the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestMiddleEnd_CreateEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		in      *pb.EncryptedVolume
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"valid request with invalid SPDK response": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Key: %v", "0123456789abcdef0123456789abcdef"),
			true,
			false,
		},
		"valid request with invalid marshal SPDK response": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json: cannot unmarshal string into Go value of type spdk.AccelCryptoKeyCreateResult"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			&encryptedVolume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			&encryptedVolume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid key and invalid bdev response": {
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Dev: %v", encryptedVolumeID),
			true,
			false,
		},
		"valid request with valid key and invalid marshal bdev response": {
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevCryptoCreateResult"),
			true,
			false,
		},
		"valid request with valid key and error code bdev response": {
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid key and ID mismatch bdev response": {
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with valid SPDK response and AES_XTS_128 cipher": {
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"my_crypto_bdev"}`},
			codes.OK,
			"",
			true,
			false,
		},
		"invalid request with AES_XTS_192 cipher": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_192,
				Key:      []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			&encryptedVolume,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
			false,
		},
		"valid request with valid SPDK response and AES_XTS_256 cipher": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:      []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:      []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"my_crypto_bdev"}`},
			codes.OK,
			"",
			true,
			false,
		},
		"invalid request with AES_CBC_128 cipher": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_128,
				Key:      []byte("0123456789abcdef"),
			},
			&encryptedVolume,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
			false,
		},
		"invalid request with AES_CBC_192 cipher": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_192,
				Key:      []byte("0123456789abcdef01234567"),
			},
			&encryptedVolume,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
			false,
		},
		"invalid request with AES_CBC_256 cipher": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_256,
				Key:      []byte("0123456789abcdef0123456789abcdef"),
			},
			&encryptedVolume,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
			false,
		},
		"invalid request with unspecified cipher": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_UNSPECIFIED,
				Key:      []byte("0123456789abcdef0123456789abcdef"),
			},
			&encryptedVolume,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
			false,
		},
		"invalid request with invalid key size for AES_XTS_128": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128,
				Key:      []byte("1234"),
			},
			&encryptedVolume,
			[]string{},
			codes.InvalidArgument,
			fmt.Sprintf("expected key size %vb, provided size %vb", 256, (4 * 8)),
			false,
			false,
		},
		"invalid request with invalid key size for AES_XTS_256": {
			&pb.EncryptedVolume{
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:      []byte("1234"),
			},
			&encryptedVolume,
			[]string{},
			codes.InvalidArgument,
			fmt.Sprintf("expected key size %vb, provided size %vb", 512, (4 * 8)),
			false,
			false,
		},
		"already exists": {
			&encryptedVolume,
			&encryptedVolume,
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

			fullname := fmt.Sprintf("//storage.opiproject.org/volumes/%s", encryptedVolumeID)
			if tt.exist {
				testEnv.opiSpdkServer.volumes.encVolumes[fullname] = &encryptedVolume
			}
			if tt.out != nil {
				tt.out.Name = fullname
			}

			request := &pb.CreateEncryptedVolumeRequest{EncryptedVolume: tt.in, EncryptedVolumeId: encryptedVolumeID}
			response, err := testEnv.client.CreateEncryptedVolume(testEnv.ctx, request)
			if response != nil {
				if string(response.Key) != string(tt.out.Key) &&
					response.Name != tt.out.Name &&
					response.VolumeId.Value != tt.out.VolumeId.Value {
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

func TestMiddleEnd_UpdateEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		in      *pb.EncryptedVolume
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"bdev delete fails": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Crypto: %s", encryptedVolumeID),
			true,
		},
		"bdev delete empty": {
			&encryptedVolume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "EOF"),
			true,
		},
		"bdev delete ID mismatch": {
			&encryptedVolume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response ID mismatch"),
			true,
		},
		"bdev delete exception": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response error: myopierr"),
			true,
		},
		"bdev delete ok ; key delete fails": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not destroy Crypto Key: %v", encryptedVolumeID),
			true,
		},
		"bdev delete ok ; key delete empty": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "EOF"),
			true,
		},
		"bdev delete ok ; key delete ID mismatch": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "json response ID mismatch"),
			true,
		},
		"bdev delete ok ; key delete exception": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "json response error: myopierr"),
			true,
		},
		"bdev delete ok ; key delete ok ; key create fails": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Key: %v", "0123456789abcdef0123456789abcdef"),
			true,
		},
		"bdev delete ok ; key delete ok ; key create empty": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "EOF"),
			true,
		},
		"bdev delete ok ; key delete ok ; key create ID mismatch": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response ID mismatch"),
			true,
		},
		"bdev delete ok ; key delete ok ; key create exception": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response error: myopierr"),
			true,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create fails": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Dev: %v", encryptedVolumeID),
			true,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create empty": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "EOF"),
			true,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create ID mismatch": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response ID mismatch"),
			true,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create exception": {
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response error: myopierr"),
			true,
		},
		"use AES_XTS_128 cipher ; bdev delete ok ; key delete ok ; key create ok ; bdev create ok": {
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			codes.OK,
			"",
			true,
		},
		"use AES_XTS_192 cipher": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_192,
				Key:      []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			nil,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
		},
		"use AES_XTS_256 cipher ; bdev delete ok ; key delete ok ; key create ok ; bdev create ok": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:      []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:      []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			codes.OK,
			"",
			true,
		},
		"use AES_CBC_128 cipher": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_128,
				Key:      []byte("0123456789abcdef"),
			},
			nil,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
		},
		"use AES_CBC_192 cipher": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_192,
				Key:      []byte("0123456789abcdef01234567"),
			},
			nil,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
		},
		"use AES_CBC_256 cipher": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_256,
				Key:      []byte("0123456789abcdef0123456789abcdef"),
			},
			nil,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
		},
		"use UNSPECIFIED cipher": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_UNSPECIFIED,
				Key:      []byte("0123456789abcdef0123456789abcdef"),
			},
			nil,
			[]string{},
			codes.InvalidArgument,
			"only AES_XTS_256 and AES_XTS_128 are supported",
			false,
		},
		"invalid key size for AES_XTS_128": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128,
				Key:      []byte("1234"),
			},
			nil,
			[]string{},
			codes.InvalidArgument,
			fmt.Sprintf("expected key size %vb, provided size %vb", 256, (4 * 8)),
			false,
		},
		"invalid key size for AES_XTS_256": {
			&pb.EncryptedVolume{
				Name:     encryptedVolumeID,
				VolumeId: encryptedVolume.VolumeId,
				Cipher:   pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:      []byte("1234"),
			},
			nil,
			[]string{},
			codes.InvalidArgument,
			fmt.Sprintf("expected key size %vb, provided size %vb", 512, (4 * 8)),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.UpdateEncryptedVolumeRequest{EncryptedVolume: tt.in}
			response, err := testEnv.client.UpdateEncryptedVolume(testEnv.ctx, request)
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

func TestMiddleEnd_ListEncryptedVolumes(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     []*pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find any namespaces for NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"valid request with valid SPDK response": {
			"volume-test",
			[]*pb.EncryptedVolume{
				{
					Name: "Malloc0",
				},
				{
					Name: "Malloc1",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},` +
				`{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
		"pagination overflow": {
			"volume-test",
			[]*pb.EncryptedVolume{
				{
					Name: "Malloc0",
				},
				{
					Name: "Malloc1",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			true,
			1000,
			"",
		},
		"pagination negative": {
			"volume-test",
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			"volume-test",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			"volume-test",
			[]*pb.EncryptedVolume{
				{
					Name: "Malloc0",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination offset": {
			"volume-test",
			[]*pb.EncryptedVolume{
				{
					Name: "Malloc1",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListEncryptedVolumesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListEncryptedVolumes(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.EncryptedVolumes, tt.out) {
					t.Error("response: expected", tt.out, "received", response.EncryptedVolumes)
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

func TestMiddleEnd_GetEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
				Name: encryptedVolumeID,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"crypto-test","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "//storage.opiproject.org/volumes/unknown-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			fname1 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", tt.in)
			fname2 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", encryptedVolumeID)
			testEnv.opiSpdkServer.volumes.encVolumes[fname2] = &encryptedVolume

			request := &pb.GetEncryptedVolumeRequest{Name: fname1}
			response, err := testEnv.client.GetEncryptedVolume(testEnv.ctx, request)
			if response != nil {
				if response.Name != tt.out.Name {
					// if !reflect.DeepEqual(response, tt.out) {
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

func TestMiddleEnd_EncryptedVolumeStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevGetIostatResult"),
			true,
		},
		"valid request with empty SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			encryptedVolumeID,
			&pb.VolumeStats{
				ReadBytesCount:    1,
				ReadOpsCount:      2,
				WriteBytesCount:   3,
				WriteOpsCount:     4,
				ReadLatencyTicks:  7,
				WriteLatencyTicks: 8,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"ticks":18787040917434338,"bdevs":[{"name":"crypto-test","bytes_read":1,"num_read_ops":2,"bytes_written":3,"num_write_ops":4,"bytes_unmapped":0,"num_unmap_ops":0,"read_latency_ticks":7,"write_latency_ticks":8,"unmap_latency_ticks":0}]}}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "//storage.opiproject.org/volumes/unknown-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			fname1 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", tt.in)
			fname2 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", encryptedVolumeID)
			testEnv.opiSpdkServer.volumes.encVolumes[fname2] = &encryptedVolume

			request := &pb.EncryptedVolumeStatsRequest{EncryptedVolumeId: &pc.ObjectKey{Value: fname1}}
			response, err := testEnv.client.EncryptedVolumeStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
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

func TestMiddleEnd_DeleteEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"valid request with invalid bdev delete SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Crypto: %v", encryptedVolumeID),
			true,
			false,
		},
		"valid request with invalid bdev delete marshal SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json: cannot unmarshal array into Go value of type spdk.BdevCryptoDeleteResult"),
			true,
			false,
		},
		"valid request with empty bdev delete SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch on bdev delete SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from bdev delete SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			encryptedVolumeID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with key delete fails": {
			encryptedVolumeID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not destroy Crypto Key: %v", encryptedVolumeID),
			true,
			false,
		},
		"valid request with error code from key delete SPDK response": {
			encryptedVolumeID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":true}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "//storage.opiproject.org/volumes/unknown-id"),
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

			fname1 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", tt.in)
			fname2 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", encryptedVolumeID)
			testEnv.opiSpdkServer.volumes.encVolumes[fname2] = &encryptedVolume

			request := &pb.DeleteEncryptedVolumeRequest{Name: fname1, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteEncryptedVolume(testEnv.ctx, request)
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
