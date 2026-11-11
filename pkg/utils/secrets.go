// SPDX-License-Identifier: Apache-2.0
package utils

import pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"

// SecretSafeEncryptedVolume clones a response without its input-only key.
func SecretSafeEncryptedVolume(volume *pb.EncryptedVolume) *pb.EncryptedVolume {
	if volume == nil {
		return nil
	}
	response := ProtoClone(volume)
	response.Key = nil
	return response
}

// SecretSafeNvmeSubsystem clones a response without its input-only PSK.
func SecretSafeNvmeSubsystem(subsystem *pb.NvmeSubsystem) *pb.NvmeSubsystem {
	if subsystem == nil {
		return nil
	}
	response := ProtoClone(subsystem)
	if response.Spec != nil {
		response.Spec.Psk = nil
	}
	return response
}

// SecretSafeNvmeRemoteController clones a response without its input-only PSK.
func SecretSafeNvmeRemoteController(controller *pb.NvmeRemoteController) *pb.NvmeRemoteController {
	if controller == nil {
		return nil
	}
	response := ProtoClone(controller)
	if response.Tcp != nil {
		response.Tcp.Psk = nil
	}
	return response
}
