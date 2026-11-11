// SPDX-License-Identifier: Apache-2.0
package utils

import (
	"bytes"
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/protobuf/proto"
)

func TestSecretSafeResponses(t *testing.T) {
	key := []byte("test-only-secret")
	volume := &pb.EncryptedVolume{Name: "volumes/v", VolumeNameRef: "volumes/backing", Key: key}
	beforeVolume := ProtoClone(volume)
	redactedVolume := SecretSafeEncryptedVolume(volume)
	if len(redactedVolume.Key) != 0 || redactedVolume.Name != volume.Name ||
		redactedVolume.VolumeNameRef != volume.VolumeNameRef || !proto.Equal(volume, beforeVolume) {
		t.Fatal("encryption response redaction must preserve metadata and its source")
	}

	subsystem := &pb.NvmeSubsystem{
		Name: "nvmeSubsystems/s",
		Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2026-01.example:s", Hostnqn: "nqn.2026-01.example:h", Psk: key},
	}
	beforeSubsystem := ProtoClone(subsystem)
	redactedSubsystem := SecretSafeNvmeSubsystem(subsystem)
	if len(redactedSubsystem.Spec.Psk) != 0 || redactedSubsystem.Spec.Nqn != subsystem.Spec.Nqn ||
		!proto.Equal(subsystem, beforeSubsystem) {
		t.Fatal("subsystem redaction must not disable stored TLS configuration")
	}
	redactedSubsystem.Spec.Nqn = "changed"
	if subsystem.Spec.Nqn != beforeSubsystem.Spec.Nqn {
		t.Fatal("response fields must not alias internal configuration")
	}

	controller := &pb.NvmeRemoteController{Name: "nvmeRemoteControllers/c", Tcp: &pb.TcpController{Psk: key, Hdgst: true}}
	beforeController := ProtoClone(controller)
	redactedController := SecretSafeNvmeRemoteController(controller)
	if len(redactedController.Tcp.Psk) != 0 || !redactedController.Tcp.Hdgst ||
		!proto.Equal(controller, beforeController) || !bytes.Equal(controller.Tcp.Psk, key) {
		t.Fatal("controller redaction must preserve options and internal PSKs")
	}
}

func TestSecretSafeAbsentResources(t *testing.T) {
	if SecretSafeEncryptedVolume(nil) != nil || SecretSafeNvmeSubsystem(nil) != nil ||
		SecretSafeNvmeRemoteController(nil) != nil {
		t.Fatal("absent responses must remain absent")
	}
	if SecretSafeNvmeSubsystem(&pb.NvmeSubsystem{}).Spec != nil ||
		SecretSafeNvmeRemoteController(&pb.NvmeRemoteController{}).Tcp != nil {
		t.Fatal("redaction must not manufacture absent nested configuration")
	}
}
