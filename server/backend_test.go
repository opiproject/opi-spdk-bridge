package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/opiproject/opi-api/storage/v1/gen/go"
)

func dialer() func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)

	s := grpc.NewServer()

	pb.RegisterNVMfRemoteControllerServiceServer(s, &server{})

	go func() {
		if err := s.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func TestBackEnd_NVMfRemoteControllerConnect(t *testing.T) {
	tests := []struct {
		name    string
		Id      int64
		Traddr  string
		Trsvcid int64
		Subnqn  string
		res     *pb.NVMfRemoteControllerConnectResponse
		errCode codes.Code
		errMsg  string
	}{
		{
			"invalid request with no spdk socket",
			8,
			"127.0.0.1",
			4444,
			"nqn.2016-06.io.spdk:cnode1",
			nil,
			codes.InvalidArgument,
			fmt.Sprintf("dial unix /var/tmp/spdk.sock: connect: no such file or directory"),
		},
		{
			"valid request with correct id, adddr, port, nqn",
			8,
			"127.0.0.1",
			4444,
			"nqn.2016-06.io.spdk:cnode1",
			&pb.NVMfRemoteControllerConnectResponse{},
			codes.OK,
			"",
		},
	}

	// TODO: listen to unix domain socket /var/tmp/spdk.sock

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewNVMfRemoteControllerServiceClient(conn)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := pb.NVMfRemoteControllerConnectRequest{Ctrl: &pb.NVMfRemoteController{Id: tt.Id, Traddr: tt.Traddr, Trsvcid: tt.Trsvcid, Subnqn: tt.Subnqn}}
			response, err := client.NVMfRemoteControllerConnect(ctx, &request)
			if response != nil {
				if !reflect.DeepEqual(response, tt.res) {
					t.Error("response: expected", tt.res, "received", response)
				}
			}
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NVMfRemoteControllerDisconnect(t *testing.T) {

}

func TestBackEnd_NVMfRemoteControllerReset(t *testing.T) {

}

func TestBackEnd_NVMfRemoteControllerList(t *testing.T) {

}

func TestBackEnd_NVMfRemoteControllerGet(t *testing.T) {

}

func TestBackEnd_NVMfRemoteControllerStats(t *testing.T) {

}
