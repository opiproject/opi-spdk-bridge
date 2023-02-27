// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"context"
	"log"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

// TODO: move to a separate (test/server) package to avoid duplication
func dialer() func(context.Context, string) (net.Conn, error) {
	var opiSpdkServer Server

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterFrontendNvmeServiceServer(server, &opiSpdkServer)
	pb.RegisterFrontendVirtioBlkServiceServer(server, &opiSpdkServer)
	pb.RegisterFrontendVirtioScsiServiceServer(server, &opiSpdkServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

// TODO: move to a separate (test/server) package to avoid duplication
func startGrpcMockupServer() (context.Context, *grpc.ClientConn) {
	// start GRPC mockup Server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	return ctx, conn
}

func TestFrontEnd_CreateVirtioBlk(t *testing.T) {
	virtioBlk := &pb.VirtioBlk{
		Id:       &pc.ObjectKey{Value: "virtio-blk-42"},
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 42},
		VolumeId: &pc.ObjectKey{Value: "Malloc42"},
		MaxIoQps: 1,
	}
	tests := map[string]struct {
		in          *pb.VirtioBlk
		out         *pb.VirtioBlk
		spdk        []string
		expectedErr error
	}{
		"valid virtio-blk creation": {
			in:          virtioBlk,
			out:         virtioBlk,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			expectedErr: status.Error(codes.OK, ""),
		},
		"spdk virtio-blk creation error": {
			in:          virtioBlk,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":1,"message":"some internal error"},"result":false}`},
			expectedErr: errFailedSpdkCall,
		},
		"spdk virtio-blk creation returned false response with no error": {
			in:          virtioBlk,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			expectedErr: errUnexpectedSpdkCallResult,
		},
	}

	ctx, conn := startGrpcMockupServer()
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := pb.NewFrontendVirtioBlkServiceClient(conn)

	ln := server.StartSpdkMockupServer()
	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(ln)

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			go server.SpdkMockServer(ln, test.spdk)
			request := &pb.CreateVirtioBlkRequest{VirtioBlk: test.in}
			response, err := client.CreateVirtioBlk(ctx, request)
			if response != nil {
				wantOut, _ := proto.Marshal(test.out)
				gotOut, _ := proto.Marshal(response)

				if !bytes.Equal(wantOut, gotOut) {
					t.Error("response: expected", test.out, "received", response)
				}
			} else if test.out != nil {
				t.Error("response: expected", test.out, "received nil")
			}

			if err != nil {
				if !strings.Contains(err.Error(), test.expectedErr.Error()) {
					t.Error("expected err contains", test.expectedErr, "received", err)
				}
			} else {
				if test.expectedErr != nil {
					t.Error("expected err contains", test.expectedErr, "received nil")
				}
			}
		})
	}
}
