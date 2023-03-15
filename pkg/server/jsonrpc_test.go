// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package server implements the server
package server

import (
	"reflect"
	"testing"
)

func TestSpdk_NewUnixSocketJSONRPC(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		transport string
	}{
		{
			"testing unix",
			"/var/tmp/spdk.sock",
			"unix",
		},
		{
			"testing tcp",
			"10.10.10.1:1234",
			"tcp",
		},
		{
			"testing nonsense assuming unix",
			"nonsense",
			"unix",
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := NewUnixSocketJSONRPC(tt.address)
			after := &unixSocketJSONRPC{
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

func TestSpdk_Call(t *testing.T) {

}
