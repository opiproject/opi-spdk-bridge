// SPDX-License-Identifier: Apache-2.0
package middleend

import (
	"bytes"
	"context"
	"strings"
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestEncryptedVolumeRPCSecretRedaction(t *testing.T) {
	env := createTestEnvironment([]string{
		`{"id":%d,"result":true}`,
		`{"id":%d,"result":"crypto-test"}`,
		`{"id":%d,"result":true}`,
		`{"id":%d,"result":true}`,
		`{"id":%d,"result":true}`,
		`{"id":%d,"result":"crypto-test"}`,
	})
	defer env.Close()
	input := utils.ProtoClone(&encryptedVolume)
	before := utils.ProtoClone(input)
	request := &pb.CreateEncryptedVolumeRequest{EncryptedVolume: input, EncryptedVolumeId: encryptedVolumeID}
	for i := 0; i < 2; i++ {
		response, err := env.client.CreateEncryptedVolume(env.ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Key) != 0 || response.Name != encryptedVolumeName ||
			response.VolumeNameRef != input.VolumeNameRef || response.Cipher != input.Cipher {
			t.Fatal("create and repeated-create responses must preserve metadata without keys")
		}
		if !bytes.Equal(env.opiSpdkServer.volumes.encVolumes[encryptedVolumeName].Key, input.Key) {
			t.Fatal("redaction must not erase internal encryption configuration")
		}
	}
	update := utils.ProtoClone(input)
	update.Name = encryptedVolumeName
	response, err := env.client.UpdateEncryptedVolume(env.ctx, &pb.UpdateEncryptedVolumeRequest{EncryptedVolume: update})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Key) != 0 {
		t.Fatal("update response exposed encryption key material")
	}
	if !proto.Equal(input, before) {
		t.Fatal("request key data must not be changed by response redaction")
	}
}

func TestCryptoKeyErrorsDoNotEchoSecrets(t *testing.T) {
	for _, response := range []string{
		`{"id":%d,"result":false}`,
		`{"id":%d,"error":{"code":1,"message":"0123456789abcdef0123456789abcdef"}}`,
	} {
		env := createTestEnvironment([]string{response})
		_, err := env.client.CreateEncryptedVolume(env.ctx, &pb.CreateEncryptedVolumeRequest{
			EncryptedVolume: utils.ProtoClone(&encryptedVolume), EncryptedVolumeId: encryptedVolumeID,
		})
		env.Close()
		if err == nil || strings.Contains(err.Error(), string(encryptedVolume.Key)) {
			t.Fatalf("crypto-key failure must be explicit without key material: %v", err)
		}
		if !strings.Contains(err.Error(), encryptedVolumeID) {
			t.Fatal("redacted errors must retain the failing resource identifier")
		}
	}
	for _, test := range []struct {
		err  error
		code codes.Code
	}{
		{context.Canceled, codes.Canceled},
		{context.DeadlineExceeded, codes.DeadlineExceeded},
		{status.Error(codes.Unavailable, "test-only-secret"), codes.Unavailable},
	} {
		err := cryptoKeyCreateError(test.err, encryptedVolumeID)
		if status.Code(err) != test.code || strings.Contains(err.Error(), "test-only-secret") {
			t.Fatalf("redaction must preserve failure codes without backend text: %v", err)
		}
	}
}
