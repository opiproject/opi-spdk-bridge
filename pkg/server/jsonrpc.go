// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package server implements the server
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"

	"github.com/opiproject/opi-spdk-bridge/pkg/models"
)

// JSONRPC represents an interface to execute JSON RPC to SPDK
type JSONRPC interface {
	Call(method string, args, result interface{}) error
}

type unixSocketJSONRPC struct {
	socket *string
	id     uint64
}

// NewUnixSocketJSONRPC creates a new instance of JSONRPC which is capable to
// interact with unix domain socket
func NewUnixSocketJSONRPC(socketPath string) JSONRPC {
	return &unixSocketJSONRPC{
		socket: &socketPath,
		id:     0,
	}
}

// Call implements low level rpc request/response handling
func (r *unixSocketJSONRPC) Call(method string, args, result interface{}) error {
	id := atomic.AddUint64(&r.id, 1)
	request := models.RPCRequest{
		RPCVersion: models.JSONRPCVersion,
		ID:         id,
		Method:     method,
		Params:     args,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("%s: %s", method, err)
	}

	log.Printf("Sending to SPDK: %s", data)

	// TODO: add also web option: resp, _ = webSocketCom(rpcClient, data)
	resp, _ := unixSocketCom(*r.socket, data)

	var response models.RPCResponse
	err = json.NewDecoder(resp).Decode(&response)
	jsonresponse, _ := json.Marshal(response)
	log.Printf("Received from SPDK: %s", jsonresponse)
	if err != nil {
		return fmt.Errorf("%s: %s", method, err)
	}
	if response.ID != id {
		return fmt.Errorf("%s: json response ID mismatch", method)
	}
	if response.Error.Code != 0 {
		return fmt.Errorf("%s: json response error: %s", method, response.Error.Message)
	}
	err = json.Unmarshal(response.Result, &result)
	if err != nil {
		return fmt.Errorf("%s: %s", method, err)
	}
	return nil
}

func unixSocketCom(lrpcSock string, buf []byte) (io.Reader, error) {
	// connect
	conn, err := net.Dial("unix", lrpcSock)
	if err != nil {
		log.Fatal(err)
	}
	// write
	_, err = conn.Write(buf)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	// close
	switch conn := conn.(type) {
	case *net.TCPConn:
		err = conn.CloseWrite()
	case *net.UnixConn:
		err = conn.CloseWrite()
	}
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	// read
	return bufio.NewReader(conn), nil
}
