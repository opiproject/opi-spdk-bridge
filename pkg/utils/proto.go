// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package utils contails useful helper functions
package utils

import (
	"google.golang.org/protobuf/proto"
)

// ProtoClone creates a deep copy of a provided gRPC structure
func ProtoClone[T proto.Message](protoStruct T) T {
	return proto.Clone(protoStruct).(T)
}

// EqualProtoSlices reports if 2 slices containing proto structures are equal
func EqualProtoSlices[T proto.Message](x, y []T) bool {
	if len(x) != len(y) {
		return false
	}

	for i := 0; i < len(x); i++ {
		if !proto.Equal(x[i], y[i]) {
			return false
		}
	}

	return true
}

// ProtoObjChangedReporter used by CheckTestProtoObjectsNotChangedInTestFunc
// to report errors if a test object changed
type ProtoObjChangedReporter interface {
	Fatalf(format string, args ...any)
}

// CheckTestProtoObjectsNotChangedInTestFunc checks test proto objects
// have not changed in a given test case. Accepts an interface to report
// a failure and the name of a test, returns a dedicated function to
// Cleanup method of *testing.T
type CheckTestProtoObjectsNotChangedInTestFunc = func(
	r ProtoObjChangedReporter, testName string) func()

// CheckTestProtoObjectsNotChanged can be used to check if test objects
// describing a resource used in multiple cases changed by a test or not.
// This function return another one for usage in a given test case.
func CheckTestProtoObjectsNotChanged(
	msgs ...proto.Message,
) CheckTestProtoObjectsNotChangedInTestFunc {
	origMsgs := []proto.Message{}
	for _, m := range msgs {
		origMsgs = append(origMsgs, ProtoClone(m))
	}

	return func(r ProtoObjChangedReporter, testName string) func() {
		return func() {
			for i, current := range msgs {
				if !proto.Equal(current, origMsgs[i]) {
					r.Fatalf("Global test object %v was changed to %v by test in %s",
						origMsgs[i], current, testName)
				}
			}
		}
	}
}
