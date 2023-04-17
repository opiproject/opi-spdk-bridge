// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"reflect"
	"testing"
)

func TestNewDeviceLocator(t *testing.T) {
	tests := map[string]struct {
		buses         []string
		wantPanic     bool
		expectLocator deviceLocator
	}{
		"random device locator on not provided buses": {
			buses:         []string{},
			wantPanic:     false,
			expectLocator: defaultDeviceLocator{},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != test.wantPanic {
					t.Errorf("newDeviceLocator() recover = %v, wantPanic = %v", r, test.wantPanic)
				}
			}()
			before := newDeviceLocator(test.buses, nvmeDeviceType)
			if !reflect.DeepEqual(before, test.expectLocator) {
				t.Error("response: expected", test.expectLocator, "received", before)
			}
		})
	}
}
