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
		"bus device locator on provided buses": {
			buses:     []string{"bus.opi.0", "bus.opi.1"},
			wantPanic: false,
			expectLocator: busDeviceLocator{
				buses: []string{"bus.opi.0", "bus.opi.1"},
			},
		},
		"panic on empty bus": {
			buses:         []string{"bus.opi.0", ""},
			wantPanic:     true,
			expectLocator: nil,
		},
		"panic on duplicated bus": {
			buses:         []string{"bus.opi.0", "bus.opi.0"},
			wantPanic:     true,
			expectLocator: nil,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("newDeviceLocator() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()
			before := newDeviceLocator(tt.buses)
			if !reflect.DeepEqual(before, tt.expectLocator) {
				t.Error("response: expected", tt.expectLocator, "received", before)
			}
		})
	}
}
