// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package utils contains utility functions
package utils

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

// ProtoCodec encodes/decodes Go values to/from PROTOBUF.
type ProtoCodec struct{}

// Marshal encodes a Go value to PROTOBUF.
func (c ProtoCodec) Marshal(v interface{}) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, errors.New("error casting interface to proto")
	}
	return proto.Marshal(msg)
}

// Unmarshal decodes a PROTOBUF value into a Go value.
func (c ProtoCodec) Unmarshal(data []byte, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return errors.New("error casting interface to proto")
	}
	return proto.Unmarshal(data, msg)
}
