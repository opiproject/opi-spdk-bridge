// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2024 Intel Corporation

// Package spdk implements the spdk json-rpc protocol
package spdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spdk/spdk/go/rpc/client"
)

// SpdkClientAdapter provides an adapter between old gospdk api and new spdk client
// This is needed due to:
// * awkward response unmarshaling in the new client. A Result in Response is
// provided in the form described in json.Unmarshal
// * New error codes affecting a lot of tests. The adapter converts the new
// format to the old one. It enables gradual transition. The tests will be
// reworked to new errors eliminating transformations in the adapter.
type SpdkClientAdapter struct {
	client client.IClient
}

// NewSpdkClientAdapter creates a new instance if SpdkClientAdapter
func NewSpdkClientAdapter(client client.IClient) *SpdkClientAdapter {
	return &SpdkClientAdapter{client}
}

// Call performs a call to spdk client and unmarshalls the result into requested structure
func (c *SpdkClientAdapter) Call(ctx context.Context, method string, params any, result any) error {
	ch := make(chan error)

	go func() {
		response, err := c.client.Call(method, params)
		if err == nil {
			var bytes []byte
			bytes, err = json.Marshal(response.Result)
			if err == nil {
				err = json.Unmarshal(bytes, result)
			}
		}

		ch <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		// The unit tests check err messages. The format used in the spdk
		// client is different. Convert to the old format to preserve behavior
		// and gradually rework the tests
		return c.convertToLegacyErrorFormat(err, method)
	}
}

func (c *SpdkClientAdapter) convertToLegacyErrorFormat(err error, method string) error {
	e := errors.Unwrap(err)
	spdkErr, _ := e.(*client.Error)

	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "EOF"):
		return fmt.Errorf("%s: EOF", method)
	case strings.Contains(err.Error(), "mismatch request and response IDs"):
		return fmt.Errorf("%s: json response ID mismatch", method)
	case strings.Contains(err.Error(), "error received for") && spdkErr != nil:
		return fmt.Errorf("%s: json response error: %s", method, spdkErr.Message)
	default:
		return fmt.Errorf("%s: %s", method, err)
	}
}
