// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023-2024 Intel Corporation

// Package spdk implements the spdk json-rpc protocol
package spdk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spdk/spdk/go/rpc/client"
)

type StubSpdkClient struct {
	StubCall func(method string, params any) (*client.Response, error)
}

func (m *StubSpdkClient) Call(method string, params any) (*client.Response, error) {
	return m.StubCall(method, params)
}

func TestCall(t *testing.T) {
	tests := map[string]struct {
		client  *StubSpdkClient
		wantErr bool
	}{
		"successful call": {
			client: &StubSpdkClient{
				StubCall: func(method string, params any) (*client.Response, error) {
					return &client.Response{Result: "TestResult"}, nil
				},
			},
			wantErr: false,
		},
		"call error": {
			client: &StubSpdkClient{
				StubCall: func(method string, params any) (*client.Response, error) {
					return nil, errors.New("call error")
				},
			},
			wantErr: true,
		},
		"marshall error": {
			client: &StubSpdkClient{
				StubCall: func(method string, params any) (*client.Response, error) {
					return &client.Response{Result: make(chan int)}, nil // json.Marshal will fail for this
				},
			},
			wantErr: true,
		},
		"unmarshall error": {
			client: &StubSpdkClient{
				StubCall: func(method string, params any) (*client.Response, error) {
					return &client.Response{Result: 1.0}, nil
				},
			},
			wantErr: true,
		},
		"context canceled": {
			client: &StubSpdkClient{
				StubCall: func(method string, params any) (*client.Response, error) {
					time.Sleep(100 * time.Millisecond)
					return &client.Response{Result: "TestResult"}, nil
				},
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewSpdkClientAdapter(tt.client)

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			result := ""
			err := c.Call(ctx, "TestMethod", "TestParams", &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("error: %v, want error: %v", err, tt.wantErr)
			}
		})
	}
}
