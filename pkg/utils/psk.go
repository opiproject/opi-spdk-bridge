// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package utils contains useful helper functions
package utils

import (
	"log"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const keyPermissions = 0600

// KeyToTemporaryFile writes pskKey into a tmp file located in tmpDir with
// required file permissions to be consumed by SPDK
func KeyToTemporaryFile(pskKey []byte) (string, error) {
	if len(pskKey) == 0 {
		return "", status.Error(codes.FailedPrecondition, "empty psk key")
	}

	keyFile, err := os.CreateTemp("/var/tmp", "opikey")
	if err != nil {
		return "", status.Error(codes.Internal, "failed to create tmp file for key")
	}

	if err := os.WriteFile(keyFile.Name(), pskKey, keyPermissions); err != nil {
		removeErr := os.Remove(keyFile.Name())
		log.Printf("Delete key file after key write: %v", removeErr)
		return "", status.Error(codes.Internal, "failed to write key into tmp file")
	}

	return keyFile.Name(), nil
}
