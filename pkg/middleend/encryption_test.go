// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implememnts the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"fmt"
	"reflect"
	"testing"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestMiddleEnd_CreateEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		id      string
		in      *pb.EncryptedVolume
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"valid request with invalid SPDK response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Key: %v", "0123456789abcdef0123456789abcdef"),
			false,
		},
		"valid request with invalid marshal SPDK response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json: cannot unmarshal string into Go value of type spdk.AccelCryptoKeyCreateResult"),
			false,
		},
		"valid request with empty SPDK response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid key and invalid bdev response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Dev: %v", encryptedVolumeID),
			false,
		},
		"valid request with valid key and invalid marshal bdev response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevCryptoCreateResult"),
			false,
		},
		"valid request with valid key and error code bdev response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid key and ID mismatch bdev response": {
			encryptedVolumeID,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response ID mismatch"),
			false,
		},
		"valid request with valid SPDK response and AES_XTS_128 cipher": {
			encryptedVolumeID,
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"my_crypto_bdev"}`},
			codes.OK,
			"",
			false,
		},
		"invalid request with AES_XTS_192 cipher": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
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
		"valid request with valid SPDK response and AES_XTS_256 cipher": {
			encryptedVolumeID,
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
			false,
		},
		"invalid request with AES_CBC_128 cipher": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
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
		"invalid request with AES_CBC_192 cipher": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
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
		"invalid request with AES_CBC_256 cipher": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
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
		"invalid request with unspecified cipher": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
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
		"invalid request with invalid key size for AES_XTS_128": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
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
		"invalid request with invalid key size for AES_XTS_256": {
			encryptedVolumeID,
			&pb.EncryptedVolume{
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
		"already exists": {
			encryptedVolumeID,
			&encryptedVolume,
			&encryptedVolume,
			[]string{},
			codes.OK,
			"",
			true,
		},
		"no required field": {
			encryptedVolumeID,
			nil,
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: encrypted_volume",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = &encryptedVolume
			}
			if tt.out != nil {
				tt.out.Name = encryptedVolumeName
			}

			request := &pb.CreateEncryptedVolumeRequest{EncryptedVolume: tt.in, EncryptedVolumeId: tt.id}
			response, err := testEnv.client.CreateEncryptedVolume(testEnv.ctx, request)

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

func TestMiddleEnd_UpdateEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.EncryptedVolume
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		// "invalid fieldmask": {
		// 	&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
		// 	&encryptedVolume,
		// 	nil,
		// 	[]string{},
		// 	codes.Unknown,
		// 	fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
		//  false,
		// },
		"bdev delete fails": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Crypto: %s", encryptedVolumeID),
			false,
		},
		"bdev delete empty": {
			nil,
			&encryptedVolume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "EOF"),
			false,
		},
		"bdev delete ID mismatch": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response ID mismatch"),
			false,
		},
		"bdev delete exception": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response error: myopierr"),
			false,
		},
		"bdev delete ok ; key delete fails": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not destroy Crypto Key: %v", encryptedVolumeID),
			false,
		},
		"bdev delete ok ; key delete empty": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "EOF"),
			false,
		},
		"bdev delete ok ; key delete ID mismatch": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "json response ID mismatch"),
			false,
		},
		"bdev delete ok ; key delete exception": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "json response error: myopierr"),
			false,
		},
		"bdev delete ok ; key delete ok ; key create fails": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Key: %v", "0123456789abcdef0123456789abcdef"),
			false,
		},
		"bdev delete ok ; key delete ok ; key create empty": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "EOF"),
			false,
		},
		"bdev delete ok ; key delete ok ; key create ID mismatch": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response ID mismatch"),
			false,
		},
		"bdev delete ok ; key delete ok ; key create exception": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response error: myopierr"),
			false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create fails": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Dev: %v", encryptedVolumeID),
			false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create empty": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "EOF"),
			false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create ID mismatch": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response ID mismatch"),
			false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create exception": {
			nil,
			&encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response error: myopierr"),
			false,
		},
		"use AES_XTS_128 cipher ; bdev delete ok ; key delete ok ; key create ok ; bdev create ok": {
			nil,
			&encryptedVolume,
			&encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			codes.OK,
			"",
			false,
		},
		"use AES_XTS_192 cipher": {
			nil,
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
			nil,
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
			false,
		},
		"use AES_CBC_128 cipher": {
			nil,
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
			nil,
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
			nil,
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
			nil,
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
			nil,
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
			nil,
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
		"malformed name": {
			nil,
			&pb.EncryptedVolume{Name: "-ABC-DEF"},
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

			request := &pb.UpdateEncryptedVolumeRequest{EncryptedVolume: tt.in, UpdateMask: tt.mask, AllowMissing: tt.missing}
			response, err := testEnv.client.UpdateEncryptedVolume(testEnv.ctx, request)

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

func TestMiddleEnd_ListEncryptedVolumes(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     []*pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with empty result SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.OK,
			"",
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
			0,
			"",
		},
		"valid request with empty SPDK response": {
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
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
			1000,
			"",
		},
		"pagination negative": {
			"volume-test",
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			-10,
			"",
		},
		"pagination error": {
			"volume-test",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
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
			1,
			"existing-pagination-token",
		},
		"no required field": {
			"",
			[]*pb.EncryptedVolume{},
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

			request := &pb.ListEncryptedVolumesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListEncryptedVolumes(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetEncryptedVolumes(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetEncryptedVolumes())
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

func TestMiddleEnd_GetEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
		},
		"valid request with empty SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			encryptedVolumeName,
			&pb.EncryptedVolume{
				Name: encryptedVolumeID,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"crypto-test","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
		},
		"malformed name": {
			server.ResourceIDToVolumeName("-ABC-DEF"),
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

			testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = &encryptedVolume

			request := &pb.GetEncryptedVolumeRequest{Name: tt.in}
			response, err := testEnv.client.GetEncryptedVolume(testEnv.ctx, request)

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

func TestMiddleEnd_EncryptedVolumeStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevGetIostatResult"),
		},
		"valid request with empty SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			encryptedVolumeID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
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
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
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

			fname1 := server.ResourceIDToVolumeName(tt.in)
			testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = &encryptedVolume

			request := &pb.EncryptedVolumeStatsRequest{EncryptedVolumeId: &pc.ObjectKey{Value: fname1}}
			response, err := testEnv.client.EncryptedVolumeStats(testEnv.ctx, request)

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

func TestMiddleEnd_DeleteEncryptedVolume(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid bdev delete SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Crypto: %v", encryptedVolumeID),
			false,
		},
		"valid request with invalid bdev delete marshal SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json: cannot unmarshal array into Go value of type spdk.BdevCryptoDeleteResult"),
			false,
		},
		"valid request with empty bdev delete SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch on bdev delete SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from bdev delete SPDK response": {
			encryptedVolumeName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			encryptedVolumeName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			codes.OK,
			"",
			false,
		},
		"valid request with key delete fails": {
			encryptedVolumeName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not destroy Crypto Key: %v", encryptedVolumeID),
			false,
		},
		"valid request with error code from key delete SPDK response": {
			encryptedVolumeName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":true}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_destroy: %v", "json response error: myopierr"),
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

			testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = &encryptedVolume

			request := &pb.DeleteEncryptedVolumeRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteEncryptedVolume(testEnv.ctx, request)

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
