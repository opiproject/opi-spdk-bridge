// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

// Package server implements the server
package server

import (
	"reflect"
	"testing"
)

func TestSpdk_NewSpdkJSONRPC(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		transport string
		wantPanic bool
	}{
		{
			"testing unix",
			"/var/tmp/spdk.sock",
			"unix",
			false,
		},
		{
			"testing tcp",
			"10.10.10.1:1234",
			"tcp",
			false,
		},
		{
			"testing empty",
			"",
			"",
			true,
		},
		{
			"testing nonsense assuming unix",
			"nonsense",
			"unix",
			false,
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("NewSpdkJSONRPC() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()
			before := NewSpdkJSONRPC(tt.address)
			after := &spdkJSONRPC{
				transport: tt.transport,
				socket:    &tt.address,
				id:        0,
			}
			if !reflect.DeepEqual(before, after) {
				t.Error("response: expected", after, "received", before)
			}
		})
	}
}

func TestSpdk_Call(_ *testing.T) {

}
