// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implememnts the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"fmt"
	"reflect"
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestMiddleEnd_CreateEncryptedVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
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
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Crypto Key: %v", "0123456789abcdef0123456789abcdef"),
			exist:   false,
		},
		"valid request with invalid marshal SPDK response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_create: %v", "json: cannot unmarshal string into Go value of type spdk.AccelCryptoKeyCreateResult"),
			exist:   false,
		},
		"valid request with empty SPDK response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_create: %v", "EOF"),
			exist:   false,
		},
		"valid request with ID mismatch SPDK response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_create: %v", "json response ID mismatch"),
			exist:   false,
		},
		"valid request with error code from SPDK response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_create: %v", "json response error: myopierr"),
			exist:   false,
		},
		"valid request with valid key and invalid bdev response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Crypto Dev: %v", encryptedVolumeID),
			exist:   false,
		},
		"valid request with valid key and invalid marshal bdev response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_create: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevCryptoCreateResult"),
			exist:   false,
		},
		"valid request with valid key and error code bdev response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_create: %v", "json response error: myopierr"),
			exist:   false,
		},
		"valid request with valid key and ID mismatch bdev response": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_create: %v", "json response ID mismatch"),
			exist:   false,
		},
		"valid request with valid SPDK response and AES_XTS_128 cipher": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     &encryptedVolume,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"my_crypto_bdev"}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
		},
		"invalid request with AES_XTS_192 cipher": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_192,
				Key:           []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			exist:   false,
		},
		"valid request with valid SPDK response and AES_XTS_256 cipher": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:           []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			out: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:           []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"my_crypto_bdev"}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
		},
		"invalid request with AES_CBC_128 cipher": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_128,
				Key:           []byte("0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			exist:   false,
		},
		"invalid request with AES_CBC_192 cipher": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_192,
				Key:           []byte("0123456789abcdef01234567"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			exist:   false,
		},
		"invalid request with AES_CBC_256 cipher": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_256,
				Key:           []byte("0123456789abcdef0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			exist:   false,
		},
		"invalid request with unspecified cipher": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_UNSPECIFIED,
				Key:           []byte("0123456789abcdef0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: encrypted_volume.cipher",
			exist:   false,
		},
		"invalid request with invalid key size for AES_XTS_128": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128,
				Key:           []byte("1234"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expected key size %vb, provided size %vb", 256, (4 * 8)),
			exist:   false,
		},
		"invalid request with invalid key size for AES_XTS_256": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:           []byte("1234"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expected key size %vb, provided size %vb", 512, (4 * 8)),
			exist:   false,
		},
		"already exists": {
			id:      encryptedVolumeID,
			in:      &encryptedVolume,
			out:     &encryptedVolume,
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			exist:   true,
		},
		"no required field": {
			id:      encryptedVolumeID,
			in:      nil,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: encrypted_volume",
			exist:   false,
		},
		"no required volume field": {
			id:      encryptedVolumeID,
			in:      &pb.EncryptedVolume{},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: encrypted_volume.volume_name_ref",
			exist:   false,
		},
		"malformed volume name": {
			id: encryptedVolumeID,
			in: &pb.EncryptedVolume{
				VolumeNameRef: "-ABC-DEF",
				Key:           encryptedVolume.Key,
				Cipher:        encryptedVolume.Cipher,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			exist:   false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = utils.ProtoClone(&encryptedVolume)
				testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName].Name = encryptedVolumeName
			}
			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
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
	encryptedVolumeWithName := utils.ProtoClone(&encryptedVolume)
	encryptedVolumeWithName.Name = encryptedVolumeName
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(encryptedVolumeWithName)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

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
		// 	mask: &fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
		// 	in: &encryptedVolume,
		// 	out: nil,
		// 	spdk: []string{},
		// 	errCode: codes.Unknown,
		// 	errMsg: fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
		//  id: false,
		// },
		"bdev delete fails": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete Crypto: %s", encryptedVolumeID),
			missing: false,
		},
		"bdev delete empty": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_delete: %v", "EOF"),
			missing: false,
		},
		"bdev delete ID mismatch": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_delete: %v", "json response ID mismatch"),
			missing: false,
		},
		"bdev delete exception": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_delete: %v", "json response error: myopierr"),
			missing: false,
		},
		"bdev delete ok ; key delete fails": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not destroy Crypto Key: %v", encryptedVolumeID),
			missing: false,
		},
		"bdev delete ok ; key delete empty": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_destroy: %v", "EOF"),
			missing: false,
		},
		"bdev delete ok ; key delete ID mismatch": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_destroy: %v", "json response ID mismatch"),
			missing: false,
		},
		"bdev delete ok ; key delete exception": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_destroy: %v", "json response error: myopierr"),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create fails": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Crypto Key: %v", "0123456789abcdef0123456789abcdef"),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create empty": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_create: %v", "EOF"),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create ID mismatch": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_create: %v", "json response ID mismatch"),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create exception": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_create: %v", "json response error: myopierr"),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create fails": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create Crypto Dev: %v", encryptedVolumeID),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create empty": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, ""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_create: %v", "EOF"),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create ID mismatch": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_create: %v", "json response ID mismatch"),
			missing: false,
		},
		"bdev delete ok ; key delete ok ; key create ok ; bdev create exception": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_create: %v", "json response error: myopierr"),
			missing: false,
		},
		"use AES_XTS_128 cipher ; bdev delete ok ; key delete ok ; key create ok ; bdev create ok": {
			mask:    nil,
			in:      encryptedVolumeWithName,
			out:     encryptedVolumeWithName,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"use AES_XTS_192 cipher": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_192,
				Key:           []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			missing: false,
		},
		"use AES_XTS_256 cipher ; bdev delete ok ; key delete ok ; key create ok ; bdev create ok": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:           []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			out: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:           []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"mytest"}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"use AES_CBC_128 cipher": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_128,
				Key:           []byte("0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			missing: false,
		},
		"use AES_CBC_192 cipher": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_192,
				Key:           []byte("0123456789abcdef01234567"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			missing: false,
		},
		"use AES_CBC_256 cipher": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_CBC_256,
				Key:           []byte("0123456789abcdef0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "only AES_XTS_256 and AES_XTS_128 are supported",
			missing: false,
		},
		"use UNSPECIFIED cipher": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_UNSPECIFIED,
				Key:           []byte("0123456789abcdef0123456789abcdef"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: encrypted_volume.cipher",
			missing: false,
		},
		"invalid key size for AES_XTS_128": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128,
				Key:           []byte("1234"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expected key size %vb, provided size %vb", 256, (4 * 8)),
			missing: false,
		},
		"invalid key size for AES_XTS_256": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Cipher:        pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256,
				Key:           []byte("1234"),
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expected key size %vb, provided size %vb", 512, (4 * 8)),
			missing: false,
		},
		"malformed name": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          "-ABC-DEF",
				VolumeNameRef: encryptedVolume.VolumeNameRef,
				Key:           encryptedVolume.Key,
				Cipher:        encryptedVolume.Cipher,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			missing: false,
		},
		"malformed volume name": {
			mask: nil,
			in: &pb.EncryptedVolume{
				Name:          encryptedVolumeID,
				VolumeNameRef: "-ABC-DEF",
				Key:           encryptedVolume.Key,
				Cipher:        encryptedVolume.Cipher,
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
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
			in:      "volume-test",
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"valid request with invalid marshal SPDK response": {
			in:      "volume-test",
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
			size:    0,
			token:   "",
		},
		"valid request with empty SPDK response": {
			in:      "volume-test",
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			size:    0,
			token:   "",
		},
		"valid request with ID mismatch SPDK response": {
			in:      "volume-test",
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			size:    0,
			token:   "",
		},
		"valid request with error code from SPDK response": {
			in:      "volume-test",
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			size:    0,
			token:   "",
		},
		"valid request with valid SPDK response": {
			in: "volume-test",
			out: []*pb.EncryptedVolume{
				{
					Name: "Malloc0",
				},
				{
					Name: "Malloc1",
				},
			},
			spdk: []string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},` +
				`{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"pagination overflow": {
			in: "volume-test",
			out: []*pb.EncryptedVolume{
				{
					Name: "Malloc0",
				},
				{
					Name: "Malloc1",
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1000,
			token:   "",
		},
		"pagination negative": {
			in:      "volume-test",
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "negative PageSize is not allowed",
			size:    -10,
			token:   "",
		},
		"pagination error": {
			in:      "volume-test",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			size:    0,
			token:   "unknown-pagination-token",
		},
		"pagination": {
			in: "volume-test",
			out: []*pb.EncryptedVolume{
				{
					Name: "Malloc0",
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
		},
		"pagination offset": {
			in: "volume-test",
			out: []*pb.EncryptedVolume{
				{
					Name: "Malloc1",
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
			out:     []*pb.EncryptedVolume{},
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

			request := &pb.ListEncryptedVolumesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListEncryptedVolumes(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetEncryptedVolumes(), tt.out) {
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevGetBdevsResult"),
		},
		"valid request with empty SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: encryptedVolumeName,
			out: &pb.EncryptedVolume{
				Name: encryptedVolumeID,
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"crypto-test","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      utils.ResourceIDToVolumeName("unknown-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
		},
		"malformed name": {
			in:      utils.ResourceIDToVolumeName("-ABC-DEF"),
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

			testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = utils.ProtoClone(&encryptedVolume)

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

func TestMiddleEnd_StatsEncryptedVolume(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      encryptedVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
		},
		"valid request with invalid marshal SPDK response": {
			in:      encryptedVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go value of type spdk.BdevGetIostatResult"),
		},
		"valid request with empty SPDK response": {
			in:      encryptedVolumeID,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      encryptedVolumeID,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      encryptedVolumeID,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: encryptedVolumeID,
			out: &pb.VolumeStats{
				ReadBytesCount:    1,
				ReadOpsCount:      2,
				WriteBytesCount:   3,
				WriteOpsCount:     4,
				ReadLatencyTicks:  7,
				WriteLatencyTicks: 8,
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"ticks":18787040917434338,"bdevs":[{"name":"crypto-test","bytes_read":1,"num_read_ops":2,"bytes_written":3,"num_write_ops":4,"bytes_unmapped":0,"num_unmap_ops":0,"read_latency_ticks":7,"write_latency_ticks":8,"unmap_latency_ticks":0}]}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      "unknown-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", utils.ResourceIDToVolumeName("unknown-id")),
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

			fname1 := utils.ResourceIDToVolumeName(tt.in)
			testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = utils.ProtoClone(&encryptedVolume)

			request := &pb.StatsEncryptedVolumeRequest{Name: fname1}
			response, err := testEnv.client.StatsEncryptedVolume(testEnv.ctx, request)

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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid bdev delete SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete Crypto: %v", encryptedVolumeID),
			missing: false,
		},
		"valid request with invalid bdev delete marshal SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_delete: %v", "json: cannot unmarshal array into Go value of type spdk.BdevCryptoDeleteResult"),
			missing: false,
		},
		"valid request with empty bdev delete SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_delete: %v", "EOF"),
			missing: false,
		},
		"valid request with ID mismatch on bdev delete SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_delete: %v", "json response ID mismatch"),
			missing: false,
		},
		"valid request with error code from bdev delete SPDK response": {
			in:      encryptedVolumeName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("bdev_crypto_delete: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			in:      encryptedVolumeName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with key delete fails": {
			in:      encryptedVolumeName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not destroy Crypto Key: %v", encryptedVolumeID),
			missing: false,
		},
		"valid request with error code from key delete SPDK response": {
			in:      encryptedVolumeName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":true}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("accel_crypto_key_destroy: %v", "json response error: myopierr"),
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

			testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName] = utils.ProtoClone(&encryptedVolume)
			testEnv.opiSpdkServer.volumes.encVolumes[encryptedVolumeName].Name = encryptedVolumeName

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
