// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package middleend implememnts the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"testing"

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
	pb.RegisterMiddleendServiceServer(server, &opiSpdkServer)

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

func TestMiddleEnd_CreateEncryptedVolume(t *testing.T) {
	encryptedVolume := &pb.EncryptedVolume{
		EncryptedVolumeId: &pc.ObjectKey{Value: "crypto-test"},
		VolumeId:          &pc.ObjectKey{Value: "volume-test"},
		Key:               []byte("0123456789abcdef0123456789abcdef"),
		Cipher:            pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128,
	}
	tests := []struct {
		name    string
		in      *pb.EncryptedVolume
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Key: %v", "0123456789abcdef0123456789abcdef"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json: cannot unmarshal string into Go struct field .result of type server.AccelCryptoKeyCreateResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			encryptedVolume,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			encryptedVolume,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			encryptedVolume,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("accel_crypto_key_create: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid key and invalid bdev response",
			encryptedVolume,
			encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":""}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create Crypto Dev: %v", "crypto-test"),
			true,
		},
		{
			"valid request with valid key and invalid marshal bdev response",
			encryptedVolume,
			encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json: cannot unmarshal bool into Go struct field .result of type server.BdevCryptoCreateResult"),
			true,
		},
		{
			"valid request with valid key and error code bdev response",
			encryptedVolume,
			encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid key and ID mismatch bdev response",
			encryptedVolume,
			encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":0,"error":{"code":0,"message":""},"result":""}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_create: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with valid SPDK response",
			encryptedVolume,
			encryptedVolume,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`, `{"id":%d,"error":{"code":0,"message":""},"result":"my_crypto_bdev"}`},
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
	client := pb.NewMiddleendServiceClient(conn)

	// start SPDK mockup Server
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
			request := &pb.CreateEncryptedVolumeRequest{EncryptedVolume: tt.in}
			response, err := client.CreateEncryptedVolume(ctx, request)
			if response != nil {
				if string(response.Key) != string(tt.out.Key) &&
					response.EncryptedVolumeId.Value != tt.out.EncryptedVolumeId.Value &&
					response.VolumeId.Value != tt.out.VolumeId.Value {
					// if !reflect.DeepEqual(response, tt.out) {
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

// func TestMiddleEnd_UpdateEncryptedVolume(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		in      *pb.EncryptedVolume
// 		out     *pb.EncryptedVolume
// 		errCode codes.Code
// 		errMsg  string
// 		start   bool
// 	}{
// 		{
// 			"unimplemented method",
// 			&pb.EncryptedVolume{},
// 			nil,
// 			codes.Unimplemented,
// 			fmt.Sprintf("%v method is not implemented", "UpdateEncryptedVolume"),
// 			false,
// 		},
// 	}

// 	// start GRPC mockup Server
// 	ctx := context.Background()
// 	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer func(conn *grpc.ClientConn) {
// 		err := conn.Close()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 	}(conn)
// 	client := pb.NewMiddleendServiceClient(conn)

// 	// run tests
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			request := &pb.UpdateEncryptedVolumeRequest{EncryptedVolume: tt.in}
// 			response, err := client.UpdateEncryptedVolume(ctx, request)
// 			if response != nil {
// 				t.Error("response: expected", codes.Unimplemented, "received", response)
// 			}

// 			if err != nil {
// 				if er, ok := status.FromError(err); ok {
// 					if er.Code() != tt.errCode {
// 						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
// 					}
// 					if er.Message() != tt.errMsg {
// 						t.Error("error message: expected", tt.errMsg, "received", er.Message())
// 					}
// 				}
// 			}
// 		})
// 	}
// }

func TestMiddleEnd_ListEncryptedVolumes(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     []*pb.EncryptedVolume
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
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go struct field .result of type []server.BdevGetBdevsResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"volume-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"volume-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"volume-test",
			[]*pb.EncryptedVolume{
				{
					EncryptedVolumeId: &pc.ObjectKey{Value: "Malloc0"},
				},
				{
					EncryptedVolumeId: &pc.ObjectKey{Value: "Malloc1"},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}},{"name":"Malloc1","aliases":["88112c76-8c49-4395-955a-0d695b1d2099"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"88112c76-8c49-4395-955a-0d695b1d2099","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
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
	client := pb.NewMiddleendServiceClient(conn)

	// start SPDK mockup Server
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
			request := &pb.ListEncryptedVolumesRequest{Parent: tt.in}
			response, err := client.ListEncryptedVolumes(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.EncryptedVolumes, tt.out) {
					t.Error("response: expected", tt.out, "received", response.EncryptedVolumes)
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

func TestMiddleEnd_GetEncryptedVolume(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.EncryptedVolume
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json: cannot unmarshal bool into Go struct field .result of type []server.BdevGetBdevsResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"crypto-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_bdevs: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"Malloc0",
			&pb.EncryptedVolume{
				EncryptedVolumeId: &pc.ObjectKey{Value: "Malloc0"},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"Malloc0","aliases":["11d3902e-d9bb-49a7-bb27-cd7261ef3217"],"product_name":"Malloc disk","block_size":512,"num_blocks":131072,"uuid":"11d3902e-d9bb-49a7-bb27-cd7261ef3217","assigned_rate_limits":{"rw_ios_per_sec":0,"rw_mbytes_per_sec":0,"r_mbytes_per_sec":0,"w_mbytes_per_sec":0},"claimed":false,"zoned":false,"supported_io_types":{"read":true,"write":true,"unmap":true,"write_zeroes":true,"flush":true,"reset":true,"compare":false,"compare_and_write":false,"abort":true,"nvme_admin":false,"nvme_io":false},"driver_specific":{}}]}`},
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
	client := pb.NewMiddleendServiceClient(conn)

	// start SPDK mockup Server
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
			request := &pb.GetEncryptedVolumeRequest{Name: tt.in}
			response, err := client.GetEncryptedVolume(ctx, request)
			if response != nil {
				if response.EncryptedVolumeId.Value != tt.out.EncryptedVolumeId.Value {
					// if !reflect.DeepEqual(response, tt.out) {
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

func TestMiddleEnd_EncryptedVolumeStats(t *testing.T) {
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
			"valid request with invalid SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json: cannot unmarshal bool into Go struct field .result of type server.BdevGetIostatResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"crypto-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"tick_rate":0,"ticks":0,"bdevs":null}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_get_iostat: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"Malloc0",
			&pb.VolumeStats{
				ReadBytesCount:    1,
				ReadOpsCount:      2,
				WriteBytesCount:   3,
				WriteOpsCount:     4,
				ReadLatencyTicks:  7,
				WriteLatencyTicks: 8,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"tick_rate":2490000000,"ticks":18787040917434338,"bdevs":[{"name":"Malloc0","bytes_read":1,"num_read_ops":2,"bytes_written":3,"num_write_ops":4,"bytes_unmapped":0,"num_unmap_ops":0,"read_latency_ticks":7,"write_latency_ticks":8,"unmap_latency_ticks":0}]}}`},
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
	client := pb.NewMiddleendServiceClient(conn)

	// start SPDK mockup Server
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
			request := &pb.EncryptedVolumeStatsRequest{EncryptedVolumeId: &pc.ObjectKey{Value: tt.in}}
			response, err := client.EncryptedVolumeStats(ctx, request)
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

func TestMiddleEnd_DeleteEncryptedVolume(t *testing.T) {
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
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Crypto: %v", "crypto-test"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json: cannot unmarshal array into Go struct field .result of type server.BdevCryptoDeleteResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"crypto-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"crypto-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_crypto_delete: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"crypto-test",
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
	client := pb.NewMiddleendServiceClient(conn)

	// start SPDK mockup Server
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
			request := &pb.DeleteEncryptedVolumeRequest{Name: tt.in}
			response, err := client.DeleteEncryptedVolume(ctx, request)
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
