// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package utils contains useful helper functions
package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyToTemporaryFile(t *testing.T) {
	tests := map[string]struct {
		pskKey    []byte
		wantError bool
	}{
		"content is written": {
			pskKey:    []byte("NVMeTLSkey-1:01:MDAxMTIyMzM0NDU1NjY3Nzg4OTlhYWJiY2NkZGVlZmZwJEiQ:"),
			wantError: false,
		},
		"empty key": {
			pskKey:    []byte{},
			wantError: true,
		},
	}
	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			psk, err := KeyToTemporaryFile(tt.pskKey)

			if tt.wantError != (err != nil) {
				t.Errorf("expected error: %v, received: %v", tt.wantError, err)
			}

			if !tt.wantError {
				pskKey, _ := os.ReadFile(filepath.Clean(psk))
				if !bytes.Equal(tt.pskKey, pskKey) {
					t.Error("expected psk key", string(tt.pskKey), "received", string(pskKey))
				}

				file, _ := os.Stat(psk)
				if file.Mode() != keyPermissions {
					t.Errorf("expect key file permissions: %o, received: %o", keyPermissions, file.Mode())
				}
			}
		})
	}
}
