// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation
package extension

import (
	"errors"
	"testing"

	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

type stubServer struct {
	server.OpiServer
	tag int
}

func TestExtend(t *testing.T) {
	extensions = nil
	stubErr := errors.New("Stub error")
	baseSpdkServer := &server.Server{}
	tests := map[string]struct {
		expectedErr     error
		extensionFuncs  []Extension
		checkOutputType func(server.OpiServer, *testing.T)
	}{
		"No extensions are applied by default": {
			checkOutputType: func(s server.OpiServer, t *testing.T) {
				if s != baseSpdkServer {
					t.Errorf("Expected unmodified baseSpdkServer, received: %v", s)
				}
			},
		},
		"Applying extensions failed": {
			expectedErr: stubErr,
			extensionFuncs: []Extension{func(baseSpdkServer server.OpiServer) (server.OpiServer, error) {
				return baseSpdkServer, stubErr
			}},
			checkOutputType: func(s server.OpiServer, t *testing.T) {
				if s != nil {
					t.Errorf("Expected nil as output, received: %v", s)
				}
			},
		},
		"Extensions are applied successfully in right order": {
			extensionFuncs: []Extension{
				func(baseSpdkServer server.OpiServer) (server.OpiServer, error) {
					return &stubServer{baseSpdkServer, 1}, nil
				}, func(baseSpdkServer server.OpiServer) (server.OpiServer, error) {
					return &stubServer{baseSpdkServer, 2}, nil
				}},
			checkOutputType: func(s server.OpiServer, t *testing.T) {
				server, ok := s.(*stubServer)
				if !ok {
					t.Errorf("Couldn't type assert %T to *stubServer", s)
				}
				if server.tag != 2 {
					t.Errorf("Expected tag: %v, received: %v", 2, server.tag)
				}

				embeddedServer, ok := server.OpiServer.(*stubServer)
				if !ok {
					t.Errorf("Couldn't type assert %T to *stubServer", server.OpiServer)
				}
				if embeddedServer.tag != 1 {
					t.Errorf("Expected tag: %v, received: %v", 1, embeddedServer.tag)
				}

				if embeddedServer.OpiServer != baseSpdkServer {
					t.Errorf("Expected baseSpdkServer, received: %T", embeddedServer.OpiServer)
				}
			},
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			for _, ext := range test.extensionFuncs {
				RegisterExtension(ext)
			}
			newServer, err := Extend(baseSpdkServer)
			extensions = nil

			if err != test.expectedErr {
				t.Errorf("Expected err: %v, received: %v", test.expectedErr, err)
			}
			test.checkOutputType(newServer, t)
		})
	}
}
