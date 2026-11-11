// SPDX-License-Identifier: Apache-2.0
package frontend

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

func TestSubsystemRPCSecretRedaction(t *testing.T) {
	env := createTestEnvironment([]string{
		`{"id":%d,"result":true}`,
		`{"id":%d,"result":true}`,
		`{"id":%d,"result":{"version":"23.09"}}`,
	})
	defer env.Close()
	env.opiSpdkServer.keyToTemporaryFile = func(key []byte) (string, error) {
		name := filepath.Join(t.TempDir(), "psk")
		return name, os.WriteFile(name, key, 0600)
	}
	input := utils.ProtoClone(&testSubsystem)
	input.Spec.Hostnqn = "nqn.2026-01.example:host"
	input.Spec.Psk = []byte("test-only-psk")
	request := &pb.CreateNvmeSubsystemRequest{NvmeSubsystem: input, NvmeSubsystemId: testSubsystemID}
	for i := 0; i < 2; i++ {
		response, err := env.client.CreateNvmeSubsystem(env.ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if len(response.GetSpec().GetPsk()) != 0 || response.GetSpec().GetNqn() != input.Spec.Nqn {
			t.Fatal("subsystem responses must retain metadata without a PSK")
		}
		if !bytes.Equal(env.opiSpdkServer.Nvme.Subsystems[testSubsystemName].Spec.Psk, input.Spec.Psk) {
			t.Fatal("redaction must not erase TLS configuration used by controller transports")
		}
	}
}
