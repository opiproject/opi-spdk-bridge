// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package server implements the server
package server

import (
	"bytes"
	"math"
	"testing"
)

func TestServer_PageToken(t *testing.T) {
	existingToken := "existing-token"
	tests := map[string]struct {
		size                int32
		token               string
		existingTokenOffset int
		expectErr           bool
		expectOffset        int
		expectEnd           int
	}{
		"negative size": {
			size:                -1,
			token:               existingToken,
			existingTokenOffset: 0,
			expectErr:           true,
			expectOffset:        0,
			expectEnd:           0,
		},
		"non-existing token": {
			size:                0,
			token:               "non-existing-token",
			existingTokenOffset: 0,
			expectErr:           true,
			expectOffset:        0,
			expectEnd:           0,
		},
		"offset in db overflow": {
			size:                0,
			token:               existingToken,
			existingTokenOffset: math.MaxInt,
			expectErr:           true,
			expectOffset:        0,
			expectEnd:           0,
		},
		"empty token": {
			size:                0,
			token:               "",
			existingTokenOffset: 0,
			expectErr:           false,
			expectOffset:        0,
			expectEnd:           50,
		},
		"existing-token": {
			size:                5,
			token:               existingToken,
			existingTokenOffset: 5,
			expectErr:           false,
			expectOffset:        5,
			expectEnd:           10,
		},
		"non-zero size": {
			size:                5,
			token:               existingToken,
			existingTokenOffset: 0,
			expectErr:           false,
			expectOffset:        0,
			expectEnd:           5,
		},
		"size more than allowed": {
			size:                1000,
			token:               existingToken,
			existingTokenOffset: 0,
			expectErr:           false,
			expectOffset:        0,
			expectEnd:           250,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pagination := NewPagination()
			pagination["existing-token"] = test.existingTokenOffset

			pageToken, err := pagination.PageToken(test.size, test.token)
			if (err != nil) != test.expectErr {
				t.Error("Expect error", test.expectErr, "received", err)
			}
			if !test.expectErr {
				if pageToken.offset != test.expectOffset {
					t.Error("Expect offset", test.expectOffset, "received", pageToken.offset)
				}
				if pageToken.end != test.expectEnd {
					t.Error("Expect page end", test.expectEnd, "received", pageToken.end)
				}
			}
		})
	}
}

func TestServer_LimitToPage(t *testing.T) {
	tests := map[string]struct {
		list                []byte
		existingTokenOffset int
		expectList          []byte
		expectEmptyToken    bool
	}{
		"list fits into page": {
			list:                []byte{1, 2, 3},
			existingTokenOffset: 1,
			expectList:          []byte{2, 3},
			expectEmptyToken:    true,
		},
		"list fits into page, but less then requested size": {
			list:                []byte{1, 2, 3},
			existingTokenOffset: 2,
			expectList:          []byte{3},
			expectEmptyToken:    true,
		},
		"list does not fit into page": {
			list:                []byte{1, 2, 3, 4, 5},
			existingTokenOffset: 1,
			expectList:          []byte{2, 3},
			expectEmptyToken:    false,
		},
		"existing token offset out of list": {
			list:                []byte{1, 2, 3},
			existingTokenOffset: 10,
			expectList:          []byte{},
			expectEmptyToken:    true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pagination := NewPagination()
			pagination["existing-token"] = test.existingTokenOffset
			pageToken, _ := pagination.PageToken(2, "existing-token")

			page := LimitToPage(pageToken, test.list)
			if !bytes.Equal(page.List, test.expectList) {
				t.Error("Expect", test.expectList, "received", page.List)
			}
			if (page.NextToken == "") != test.expectEmptyToken {
				t.Error("Expect empty token", test.expectEmptyToken, "received", page.NextToken)
			}
		})
	}
}
