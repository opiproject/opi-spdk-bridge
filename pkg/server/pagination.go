// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package server implements the server
package server

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Pagination is type used to track pagination for List calls
type Pagination map[string]int

// NewPagination creates a new Pagination instance
func NewPagination() Pagination {
	return make(Pagination)
}

// PageToken returns page token describing an existing token or a new one based
// on provided request parameters
func (p Pagination) PageToken(size int32, token string) (PageToken, error) {
	const (
		maxPageSize     = 250
		defaultPageSize = 50
	)
	switch {
	case size < 0:
		return PageToken{}, status.Error(codes.InvalidArgument, "negative PageSize is not allowed")
	case size == 0:
		size = defaultPageSize
	case size > maxPageSize:
		size = maxPageSize
	}

	offset := 0
	if token != "" {
		var ok bool
		offset, ok = p[token]
		if !ok {
			return PageToken{}, status.Errorf(codes.NotFound, "unable to find pagination token %s", token)
		}
		log.Printf("Found offset %d from pagination token: %s", offset, token)
	}

	end, err := addInt(offset, int(size))
	if err != nil {
		return PageToken{}, status.Errorf(codes.InvalidArgument, "Invalid page argument")
	}

	return PageToken{
		offset:     offset,
		end:        end,
		pagination: p,
	}, nil
}

// PageToken describes an existing token or a new one based
// on provided request parameters
type PageToken struct {
	offset     int
	end        int
	pagination Pagination
}

// LimitToPage returns a list for response based on provided page token
func LimitToPage[T any](pt PageToken, list []T) Page[T] {
	listSize := len(list)
	nextToken := ""
	offset := pt.offset
	end := pt.end
	if offset >= listSize {
		log.Printf("Offset %v is greater than list size %v", offset, listSize)
		return Page[T]{}
	}

	if end < listSize {
		nextToken = uuid.NewString()
		pt.pagination[nextToken] = end
	} else {
		end = listSize
	}

	log.Printf("Limiting result len(%d) to [%d:%d]", len(list), offset, end)
	return Page[T]{
		List:      list[offset:end],
		NextToken: nextToken,
	}
}

// Page describes results to send in response based on pagination
type Page[T any] struct {
	List      []T
	NextToken string
}

func addInt(a, b int) (int, error) {
	r := a + b
	if a > 0 && b > 0 && r < 0 {
		return 0, fmt.Errorf("integer overflow for %v and %v", a, b)
	} else if a < 0 && b < 0 && r > 0 {
		return 0, fmt.Errorf("integer underflow for %v and %v", a, b)
	}
	return r, nil
}
