// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package server implements the server
package server

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
)

// CreateTestSpdkServer creates a mock spdk server for testing
func CreateTestSpdkServer(socket string, startSpdkServer bool, spdkResponses []string) (net.Listener, JSONRPC) {
	jsonRPC := &unixSocketJSONRPC{
		transport: "unix",
		socket:    &socket,
		id:        0,
	}

	ln := startSpdkMockupServerOnUnixSocket(jsonRPC)

	if startSpdkServer {
		go spdkMockServerOnUnixSocket(jsonRPC, ln, spdkResponses)
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

func startSpdkMockupServerOnUnixSocket(rpc *unixSocketJSONRPC) net.Listener {
	// start SPDK mockup Server
	if err := os.RemoveAll(*rpc.socket); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpc.socket)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	return ln
}

func spdkMockServerOnUnixSocket(rpc *unixSocketJSONRPC, l net.Listener, toSend []string) {
	for _, spdk := range toSend {
		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}
		log.Printf("SPDK mockup Server: client connected [%s]", fd.RemoteAddr().Network())
		log.Printf("SPDK ID [%d]", rpc.id)

		buf := make([]byte, 512)
		nr, err := fd.Read(buf)
		if err != nil {
			return
		}

		data := buf[0:nr]
		if strings.Contains(spdk, "%") {
			spdk = fmt.Sprintf(spdk, rpc.id)
		}

		log.Printf("SPDK mockup Server: got : %s", string(data))
		log.Printf("SPDK mockup Server: snd : %s", spdk)

		_, err = fd.Write([]byte(spdk))
		if err != nil {
			log.Fatal("Write: ", err)
		}
		err = fd.(*net.UnixConn).CloseWrite()
		if err != nil {
			log.Fatal(err)
		}
	}
}
