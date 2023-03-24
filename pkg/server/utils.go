// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package server implements the server
package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

// TestOpiClient represents a client which is able to perform all OPI requests
type TestOpiClient struct {
	pb.FrontendNvmeServiceClient
	pb.FrontendVirtioBlkServiceClient
	pb.FrontendVirtioScsiServiceClient

	pb.MiddleendServiceClient

	pb.AioControllerServiceClient
	pb.NullDebugServiceClient
	pb.NVMfRemoteControllerServiceClient
}

// TestOpiServer represents a test server which implements all OPI calls
type TestOpiServer struct {
	pb.FrontendNvmeServiceServer
	pb.FrontendVirtioBlkServiceServer
	pb.FrontendVirtioScsiServiceServer

	pb.MiddleendServiceServer

	pb.AioControllerServiceServer
	pb.NullDebugServiceServer
	pb.NVMfRemoteControllerServiceServer
}

// TestEnv represents a testing environment with all required objects for testing
type TestEnv struct {
	OpiSpdkServer TestOpiServer
	Client        *TestOpiClient
	Ctx           context.Context

	ln         net.Listener
	testSocket string
	conn       *grpc.ClientConn
	jsonRPC    JSONRPC
}

// Close used to close all created resources for testing
func (e *TestEnv) Close() {
	CloseListener(e.ln)
	CloseGrpcConnection(e.conn)
	if err := os.RemoveAll(e.testSocket); err != nil {
		log.Fatal(err)
	}
}

// CreateTestEnvironment creates an instance of TestEnv
func CreateTestEnvironment(startSpdkServer bool, spdkResponses []string, serverCreator func(JSONRPC) TestOpiServer) *TestEnv {
	env := &TestEnv{}
	env.testSocket = GenerateSocketName("opi-unit-test")
	env.ln, env.jsonRPC = CreateTestSpdkServer(env.testSocket, startSpdkServer, spdkResponses)
	env.OpiSpdkServer = serverCreator(env.jsonRPC)

	env.Ctx = context.Background()
	conn, err := grpc.DialContext(env.Ctx,
		"",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer(env.OpiSpdkServer)))
	if err != nil {
		log.Fatal(err)
	}
	env.conn = conn

	env.Client = &TestOpiClient{
		pb.NewFrontendNvmeServiceClient(env.conn),
		pb.NewFrontendVirtioBlkServiceClient(env.conn),
		pb.NewFrontendVirtioScsiServiceClient(env.conn),
		pb.NewMiddleendServiceClient(env.conn),
		pb.NewAioControllerServiceClient(env.conn),
		pb.NewNullDebugServiceClient(env.conn),
		pb.NewNVMfRemoteControllerServiceClient(env.conn),
	}

	return env
}

func dialer(opiSpdkServer TestOpiServer) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterFrontendNvmeServiceServer(server, opiSpdkServer)
	pb.RegisterFrontendVirtioBlkServiceServer(server, opiSpdkServer)
	pb.RegisterFrontendVirtioScsiServiceServer(server, opiSpdkServer)
	pb.RegisterMiddleendServiceServer(server, opiSpdkServer)
	pb.RegisterAioControllerServiceServer(server, opiSpdkServer)
	pb.RegisterNullDebugServiceServer(server, opiSpdkServer)
	pb.RegisterNVMfRemoteControllerServiceServer(server, opiSpdkServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
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
	if err := os.RemoveAll(*rpc.socket); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpc.socket)
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
