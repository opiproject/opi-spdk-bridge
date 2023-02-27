// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

// TODO: move to a separate (test/server) package to avoid duplication
func dialer() func(context.Context, string) (net.Conn, error) {
	var opiSpdkServer Server

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterNVMfRemoteControllerServiceServer(server, &opiSpdkServer)
	pb.RegisterNullDebugServiceServer(server, &opiSpdkServer)
	pb.RegisterAioControllerServiceServer(server, &opiSpdkServer)

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

func TestBackEnd_CreateNVMfRemoteController(t *testing.T) {
	controller := &pb.NVMfRemoteController{
		Id:      &pc.ObjectKey{Value: "OpiNvme8"},
		Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
		Traddr:  "127.0.0.1",
		Trsvcid: 4444,
		Subnqn:  "nqn.2016-06.io.spdk:cnode1",
		Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
	}
	tests := []struct {
		name    string
		in      *pb.NVMfRemoteController
		out     *pb.NVMfRemoteController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid marshal SPDK response",
			controller,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json: cannot unmarshal bool into Go struct field .result of type []server.BdevNvmeAttachControllerResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			controller,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			controller,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			controller,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			controller,
			controller,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":["my_remote_nvmf_bdev"]}`},
			codes.OK,
			"",
			true,
		},
	}

	ctx, conn := startGrpcMockupServer()

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := pb.NewNVMfRemoteControllerServiceClient(conn)

	ln := server.StartSpdkMockupServer()

	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(ln)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go server.SpdkMockServer(ln, tt.spdk)
			}
			request := &pb.CreateNVMfRemoteControllerRequest{NvMfRemoteController: tt.in}
			response, err := client.CreateNVMfRemoteController(ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NVMfRemoteControllerReset(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request without SPDK",
			"volume-test",
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
		},
	}

	ctx, conn := startGrpcMockupServer()

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := pb.NewNVMfRemoteControllerServiceClient(conn)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &pb.NVMfRemoteControllerResetRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := client.NVMfRemoteControllerReset(ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_ListNVMfRemoteControllers(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     []*pb.NVMfRemoteController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find any namespaces for NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json: cannot unmarshal bool into Go struct field .result of type []server.BdevNvmeGetControllerResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"volume-test",
			[]*pb.NVMfRemoteController{
				{
					Id:      &pc.ObjectKey{Value: "OpiNvme12"},
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 4444,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
				{
					Id:      &pc.ObjectKey{Value: "OpiNvme13"},
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 8888,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"OpiNvme12","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"4444","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]},{"name":"OpiNvme13","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"8888","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]}]}`},
			codes.OK,
			"",
			true,
		},
	}

	ctx, conn := startGrpcMockupServer()

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := pb.NewNVMfRemoteControllerServiceClient(conn)

	ln := server.StartSpdkMockupServer()

	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(ln)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go server.SpdkMockServer(ln, tt.spdk)
			}
			request := &pb.ListNVMfRemoteControllersRequest{Parent: tt.in}
			response, err := client.ListNVMfRemoteControllers(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvMfRemoteControllers, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvMfRemoteControllers)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_GetNVMfRemoteController(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.NVMfRemoteController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json: cannot unmarshal bool into Go struct field .result of type []server.BdevNvmeGetControllerResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"OpiNvme12",
			&pb.NVMfRemoteController{
				Id:      &pc.ObjectKey{Value: "OpiNvme12"},
				Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
				Traddr:  "127.0.0.1",
				Trsvcid: 4444,
				Subnqn:  "nqn.2016-06.io.spdk:cnode1",
				Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"OpiNvme12","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"4444","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]}]}`},
			codes.OK,
			"",
			true,
		},
	}

	ctx, conn := startGrpcMockupServer()

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := pb.NewNVMfRemoteControllerServiceClient(conn)

	ln := server.StartSpdkMockupServer()

	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(ln)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go server.SpdkMockServer(ln, tt.spdk)
			}
			request := &pb.GetNVMfRemoteControllerRequest{Name: tt.in}
			response, err := client.GetNVMfRemoteController(ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NVMfRemoteControllerStats(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with valid SPDK response",
			"Malloc0",
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{""},
			codes.OK,
			"",
			false,
		},
	}

	ctx, conn := startGrpcMockupServer()

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := pb.NewNVMfRemoteControllerServiceClient(conn)

	ln := server.StartSpdkMockupServer()

	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(ln)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go server.SpdkMockServer(ln, tt.spdk)
			}
			request := &pb.NVMfRemoteControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := client.NVMfRemoteControllerStats(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_DeleteNVMfRemoteController(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Crypto: %v", "volume-test"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json: cannot unmarshal array into Go struct field .result of type server.BdevNvmeDetachControllerResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"volume-test",
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			codes.OK,
			"",
			true,
		},
	}

	ctx, conn := startGrpcMockupServer()

	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := pb.NewNVMfRemoteControllerServiceClient(conn)

	ln := server.StartSpdkMockupServer()

	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(ln)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go server.SpdkMockServer(ln, tt.spdk)
			}
			request := &pb.DeleteNVMfRemoteControllerRequest{Name: tt.in}
			response, err := client.DeleteNVMfRemoteController(ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}
