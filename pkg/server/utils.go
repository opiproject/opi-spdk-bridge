// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package server implements the server
package server

import (
	"crypto/rand"
	"fmt"
	"log"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExtractPagination is a helper function for List pagination to fetch PageSize and PageToken
func ExtractPagination(pageSize int32, pageToken string, pagination map[string]int) (int, int, error) {
	if pageSize < 0 {
		err := status.Error(codes.InvalidArgument, "negative PageSize is not allowed")
		return -1, -1, err
	}
	// pick reasonable default and max sizes
	size := 50
	if pageSize > 0 {
		size = int(math.Min(float64(pageSize), 250))
	}
	// fetch offset from the database using opaque token
	offset := 0
	if pageToken != "" {
		var ok bool
		offset, ok = pagination[pageToken]
		if !ok {
			err := status.Errorf(codes.NotFound, "unable to find pagination token %s", pageToken)
			return -1, -1, err
		}
		log.Printf("Found offset %d from pagination token: %s", offset, pageToken)
	}
	return size, offset, nil
}

// LimitPagination is a helper function for slice the result by offset and size
func LimitPagination[T any](result []T, offset int, size int) ([]T, bool) {
	end := offset + size
	hasMoreElements := false
	if end < len(result) {
		hasMoreElements = true
	} else {
		end = len(result)
	}
	return result[offset:end], hasMoreElements
}

// CreateTestSpdkServer creates a mock spdk server for testing
func CreateTestSpdkServer(socket string, startSpdkServer bool, spdkResponses []string) (net.Listener, JSONRPC) {
	jsonRPC := NewSpdkJSONRPC(socket).(*spdkJSONRPC)
	ln := startSpdkMockupServerOnUnixSocket(jsonRPC)
	if startSpdkServer {
		go spdkMockServerCommunicate(jsonRPC, ln, spdkResponses)
	}
	return ln, jsonRPC
}

// CloseGrpcConnection is utility function used to defer grpc connection close is tests
func CloseGrpcConnection(conn *grpc.ClientConn) {
	err := conn.Close()
	if err != nil {
		log.Fatal(err)
	}
}

// CloseListener is utility function used to defer listener close in tests
func CloseListener(ln net.Listener) {
	err := ln.Close()
	if err != nil {
		log.Fatal(err)
	}
}

// GenerateSocketName generates unique socket names for tests
func GenerateSocketName(testType string) string {
	nBig, err := rand.Int(rand.Reader, big.NewInt(9223372036854775807))
	if err != nil {
		panic(err)
	}
	n := nBig.Uint64()
	return filepath.Join(os.TempDir(), "opi-spdk-"+testType+"-test-"+fmt.Sprint(n)+".sock")
}

func startSpdkMockupServerOnUnixSocket(rpc *spdkJSONRPC) net.Listener {
	// start SPDK mockup Server
	if err := os.RemoveAll(rpc.socket); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", rpc.socket)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	return ln
}

func spdkMockServerCommunicate(rpc *spdkJSONRPC, l net.Listener, toSend []string) {
	for _, spdk := range toSend {
		// wait for client to connect (accept stage)
		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}
		log.Printf("SPDK mockup Server: client connected [%s]", fd.RemoteAddr().Network())
		log.Printf("SPDK ID [%d]", rpc.id)
		// read from client
		// we just read to extract ID, rest of the data is discarded here
		buf := make([]byte, 512)
		nr, err := fd.Read(buf)
		if err != nil {
			log.Panic("Read: ", err)
		}
		// fill in ID, since client expects the same ID in the response
		data := buf[0:nr]
		if strings.Contains(spdk, "%") {
			spdk = fmt.Sprintf(spdk, rpc.id)
		}
		log.Printf("SPDK mockup Server: got : %s", string(data))
		log.Printf("SPDK mockup Server: snd : %s", spdk)
		// send data back to client
		_, err = fd.Write([]byte(spdk))
		if err != nil {
			log.Panic("Write: ", err)
		}
		// close connection
		switch fd := fd.(type) {
		case *net.TCPConn:
			err = fd.CloseWrite()
		case *net.UnixConn:
			err = fd.CloseWrite()
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}
