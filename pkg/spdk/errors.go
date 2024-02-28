// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2024 Intel Corporation

// Package spdk implements the spdk json-rpc protocol
package spdk

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrFailedSpdkCall indicates that the bridge failed to execute SPDK call
	ErrFailedSpdkCall = status.Error(codes.Unknown, "Failed to execute SPDK call")
	// ErrUnexpectedSpdkCallResult indicates that the bridge got an error from SPDK
	ErrUnexpectedSpdkCallResult = status.Error(codes.FailedPrecondition, "Unexpected SPDK call result.")
)
