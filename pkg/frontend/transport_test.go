// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"reflect"
	"testing"

	"github.com/opiproject/opi-spdk-bridge/pkg/spdk"
	"github.com/spdk/spdk/go/rpc/client"
)

func TestNewNvmeVfiouserTransport(t *testing.T) {
	tests := map[string]struct {
		rpc       client.IClient
		wantPanic bool
	}{
		"nil json rpc": {
			rpc:       nil,
			wantPanic: true,
		},
		"valid transport": {
			rpc:       spdkCLientStub{},
			wantPanic: false,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("NewNvmeVfiouserTransport() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			gotTransport := NewNvmeTCPTransport(tt.rpc)
			wantTransport := &nvmeTCPTransport{
				rpc: spdk.NewSpdkClientAdapter(spdkCLientStub{}),
			}

			if !reflect.DeepEqual(gotTransport, wantTransport) {
				t.Errorf("Received transport %v not equal to expected one %v", gotTransport, wantTransport)
			}
		})
	}
}
