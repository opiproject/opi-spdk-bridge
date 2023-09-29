// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

type stubNvmeTransport struct {
	err error
}

func (t *stubNvmeTransport) Params(_ *pb.NvmeController, _ string) (spdk.NvmfSubsystemAddListenerParams, error) {
	return spdk.NvmfSubsystemAddListenerParams{}, t.err
}

var (
	alwaysValidNvmeTransport  = &stubNvmeTransport{}
	alwaysFailedNvmeTransport = &stubNvmeTransport{errors.New("some transport error")}
)

func TestFrontEnd_NewNvmeTCPTransport(t *testing.T) {
	tests := map[string]struct {
		listenAddress string
		wantPanic     bool
		protocol      string
	}{
		"ipv4 valid address": {
			listenAddress: "10.10.10.10:12345",
			wantPanic:     false,
			protocol:      ipv4NvmeTCPProtocol,
		},
		"valid ipv6 addresses": {
			listenAddress: "[2002:0db0:8833:0000:0000:8a8a:0330:7337]:54321",
			wantPanic:     false,
			protocol:      ipv6NvmeTCPProtocol,
		},
		"empty string as listen address": {
			listenAddress: "",
			wantPanic:     true,
			protocol:      "",
		},
		"missing port": {
			listenAddress: "10.10.10.10",
			wantPanic:     true,
			protocol:      "",
		},
		"valid port invalid ip": {
			listenAddress: "wrong:12345",
			wantPanic:     true,
			protocol:      "",
		},
		"meaningless listen address": {
			listenAddress: "some string which is not ip address",
			wantPanic:     true,
			protocol:      "",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("NewNvmeTCPTransport() recover = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			gotTransport := NewNvmeTCPTransport(tt.listenAddress)
			host, port, _ := net.SplitHostPort(tt.listenAddress)
			wantTransport := &nvmeTCPTransport{
				listenAddr: net.ParseIP(host),
				listenPort: port,
				protocol:   tt.protocol,
			}

			if !reflect.DeepEqual(gotTransport, wantTransport) {
				t.Errorf("Expect %v transport, received %v", wantTransport, gotTransport)
			}
		})
	}
}
