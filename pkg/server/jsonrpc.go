// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package server implements the server
package server

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
)

var (
	RpcID   int32 // json request message ID, auto incremented
	RpcSock = flag.String("rpc_sock", "/var/tmp/spdk.sock", "Path to SPDK JSON RPC socket")
)

// low level rpc request/response handling
func Call(method string, args, result interface{}) error {
	type rpcRequest struct {
		Ver    string `json:"jsonrpc"`
		ID     int32  `json:"id"`
		Method string `json:"method"`
	}

	id := atomic.AddInt32(&RpcID, 1)
	request := rpcRequest{
		Ver:    "2.0",
		ID:     id,
		Method: method,
	}

	var data []byte
	var err error

	if args == nil {
		data, err = json.Marshal(request)
	} else {
		requestWithParams := struct {
			rpcRequest
			Params interface{} `json:"params"`
		}{
			request,
			args,
		}
		data, err = json.Marshal(requestWithParams)
	}
	if err != nil {
		return fmt.Errorf("%s: %s", method, err)
	}

	log.Printf("Sending to SPDK: %s", data)

	// TODO: add also web option: resp, _ = webSocketCom(rpcClient, data)
	resp, _ := unixSocketCom(*RpcSock, data)

	response := struct {
		ID    int32 `json:"id"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result interface{} `json:"result"`
	}{
		Result: result,
	}
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

	return nil
}

func unixSocketCom(lrpcSock string, buf []byte) (io.Reader, error) {
	conn, err := net.Dial("unix", lrpcSock)
	if err != nil {
		log.Fatal(err)
	}
	_, err = conn.Write(buf)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	err = conn.(*net.UnixConn).CloseWrite()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return bufio.NewReader(conn), nil
}
