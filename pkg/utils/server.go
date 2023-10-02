// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package utils contails useful helper functions
package utils

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// ExtractPagination is a helper function for List pagination to fetch PageSize and PageToken
func ExtractPagination(pageSize int32, pageToken string, pagination map[string]int) (size int, offset int, err error) {
	const (
		maxPageSize     = 250
		defaultPageSize = 50
	)
	switch {
	case pageSize < 0:
		return -1, -1, status.Error(codes.InvalidArgument, "negative PageSize is not allowed")
	case pageSize == 0:
		size = defaultPageSize
	case pageSize > maxPageSize:
		size = maxPageSize
	default:
		size = int(pageSize)
	}
	// fetch offset from the database using opaque token
	offset = 0
	if pageToken != "" {
		var ok bool
		offset, ok = pagination[pageToken]
		if !ok {
			return -1, -1, status.Errorf(codes.NotFound, "unable to find pagination token %s", pageToken)
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
func CreateTestSpdkServer(socket string, spdkResponses []string) (net.Listener, spdk.JSONRPC) {
	jsonRPC := spdk.NewSpdkJSONRPC(socket)
	ln := jsonRPC.StartUnixListener()
	if len(spdkResponses) > 0 {
		go spdkMockServerCommunicate(jsonRPC, ln, spdkResponses)
	}
	return ln, jsonRPC.(*spdk.SpdkJSONRPC)
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

func spdkMockServerCommunicate(rpc spdk.JSONRPC, l net.Listener, toSend []string) {
	for _, spdk := range toSend {
		// wait for client to connect (accept stage)
		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}
		log.Printf("SPDK mockup Server: client connected [%s]", fd.RemoteAddr().Network())
		id := rpc.GetID()
		log.Printf("SPDK ID [%d]", id)
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
			spdk = fmt.Sprintf(spdk, id)
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

// ResourceIDToVolumeName creates name of volume resource based on ID
func ResourceIDToVolumeName(resourceID string) string {
	return fmt.Sprintf("//storage.opiproject.org/volumes/%s", resourceID)
}

// OpiAdressFamilyToSpdk converts opi address family to the one used in spdk
func OpiAdressFamilyToSpdk(adrfam pb.NvmeAddressFamily) string {
	if adrfam == pb.NvmeAddressFamily_NVME_ADDRESS_FAMILY_UNSPECIFIED {
		return ""
	}

	return strings.ReplaceAll(adrfam.String(), "NVME_ADRFAM_", "")
}
