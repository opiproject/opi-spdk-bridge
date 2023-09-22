// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package middleend implememnts the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"fmt"

	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func (s *Server) validateCreateEncryptedVolumeRequest(in *pb.CreateEncryptedVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.EncryptedVolume.VolumeNameRef); err != nil {
		return err
	}
	// see https://google.aip.dev/133#user-specified-ids
	if in.EncryptedVolumeId != "" {
		if err := resourceid.ValidateUserSettable(in.EncryptedVolumeId); err != nil {
			return err
		}
	}
	// TODO: validate also: block_size, blocks_count, uuid, filename
	return nil
}

func (s *Server) validateDeleteEncryptedVolumeRequest(in *pb.DeleteEncryptedVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateUpdateEncryptedVolumeRequest(in *pb.UpdateEncryptedVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.EncryptedVolume.VolumeNameRef); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.EncryptedVolume.Name)
}

func (s *Server) validateGetEncryptedVolumeRequest(in *pb.GetEncryptedVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) validateStatsEncryptedVolumeRequest(in *pb.StatsEncryptedVolumeRequest) error {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	return resourcename.Validate(in.Name)
}

func (s *Server) verifyEncryptedVolume(volume *pb.EncryptedVolume) error {
	keyLengthInBits := len(volume.Key) * 8
	expectedKeyLengthInBits := 0
	switch {
	case volume.Cipher == pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256:
		expectedKeyLengthInBits = 512
	case volume.Cipher == pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128:
		expectedKeyLengthInBits = 256
	default:
		return fmt.Errorf("only AES_XTS_256 and AES_XTS_128 are supported")
	}

	if keyLengthInBits != expectedKeyLengthInBits {
		return fmt.Errorf("expected key size %vb, provided size %vb",
			expectedKeyLengthInBits, keyLengthInBits)
	}

	return nil
}
