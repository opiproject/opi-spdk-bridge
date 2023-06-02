// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"bytes"
	"fmt"
	"net"
	"testing"

	"github.com/opiproject/gospdk/spdk"
	_go "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	testQosVolumeID = "qos-volume-42"
	testQosVolume   = &pb.QosVolume{
		VolumeId: &_go.ObjectKey{Value: "volume-42"},
		MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
	}
)

type stubJSONRRPC struct {
	params []any
}

func (s *stubJSONRRPC) GetID() uint64 {
	return 0
}

func (s *stubJSONRRPC) StartUnixListener() net.Listener {
	return nil
}

func (s *stubJSONRRPC) GetVersion() string {
	return ""
}

func (s *stubJSONRRPC) Call(_ string, param interface{}, _ interface{}) error {
	s.params = append(s.params, param)
	return nil
}

func TestMiddleEnd_CreateQosVolume(t *testing.T) {
	tests := map[string]struct {
		in          *pb.QosVolume
		out         *pb.QosVolume
		spdk        []string
		errCode     codes.Code
		errMsg      string
		start       bool
		existBefore bool
		existAfter  bool
	}{
		"min_limit is not supported": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MinLimit: &pb.QosLimit{
					RdIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume min_limit is not supported",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"max_limit rd_iops_kiops is not supported": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RdIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rd_iops_kiops is not supported",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"max_limit wr_iops_kiops is not supported": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					WrIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit wr_iops_kiops is not supported",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"max_limit rw_iops_kiops is negative": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RwIopsKiops: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rw_iops_kiops cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"max_limit rd_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RdBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rd_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"max_limit wr_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					WrBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit wr_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"max_limit rw_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RwBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rw_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"max_limit with all zero limits": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit should set limit",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"qos_volume_id is ignored": {
			in: &pb.QosVolume{
				Name:     "ignored-id",
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         testQosVolume,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode:     codes.OK,
			errMsg:      "",
			start:       true,
			existBefore: false,
			existAfter:  true,
		},
		"volume_id is nil": {
			in: &pb.QosVolume{
				VolumeId: nil,
				MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "volume_id cannot be empty",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"volume_id is empty": {
			in: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: ""},
				MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "volume_id cannot be empty",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"qos volume already exists": {
			in:          testQosVolume,
			out:         testQosVolume,
			spdk:        []string{},
			errCode:     codes.OK,
			errMsg:      "",
			start:       false,
			existBefore: true,
			existAfter:  true,
		},
		"SPDK call failed": {
			in:          testQosVolume,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":1,"message":"some internal error"},"result":true}`},
			errCode:     status.Convert(spdk.ErrFailedSpdkCall).Code(),
			errMsg:      status.Convert(spdk.ErrFailedSpdkCall).Message(),
			start:       true,
			existBefore: false,
			existAfter:  false,
		},
		"SPDK call result false": {
			in:          testQosVolume,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode:     status.Convert(spdk.ErrUnexpectedSpdkCallResult).Code(),
			errMsg:      status.Convert(spdk.ErrUnexpectedSpdkCallResult).Message(),
			start:       true,
			existBefore: false,
			existAfter:  false,
		},
		"successful creation": {
			in:          testQosVolume,
			out:         testQosVolume,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode:     codes.OK,
			errMsg:      "",
			start:       true,
			existBefore: false,
			existAfter:  true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			fullname := fmt.Sprintf("//storage.opiproject.org/volumes/%s", testQosVolumeID)
			if tt.existBefore {
				testEnv.opiSpdkServer.volumes.qosVolumes[fullname] = tt.in
			}
			if tt.out != nil {
				tt.out.Name = fullname
			}

			request := &pb.CreateQosVolumeRequest{QosVolume: tt.in, QosVolumeId: testQosVolumeID}
			response, err := testEnv.client.CreateQosVolume(testEnv.ctx, request)

			marshalledOut, _ := proto.Marshal(tt.out)
			marshalledResponse, _ := proto.Marshal(response)
			if !bytes.Equal(marshalledOut, marshalledResponse) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expect grpc error status")
			}

			vol, ok := testEnv.opiSpdkServer.volumes.qosVolumes[fullname]
			if tt.existAfter != ok {
				t.Error("expect QoS volume exist", tt.existAfter, "received", ok)
			}
			if tt.existAfter && !proto.Equal(tt.out, vol) {
				t.Error("expect QoS volume ", vol, "is equal to", tt.in)
			}
		})
	}

	t.Run("valid values are sent to SPDK", func(t *testing.T) {
		testEnv := createTestEnvironment(false, []string{})
		defer testEnv.Close()
		stubRPC := &stubJSONRRPC{}
		testEnv.opiSpdkServer.rpc = stubRPC

		_, _ = testEnv.client.CreateQosVolume(testEnv.ctx, &pb.CreateQosVolumeRequest{
			QosVolumeId: testQosVolumeID,
			QosVolume: &pb.QosVolume{
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RwIopsKiops:    1,
					RdBandwidthMbs: 2,
					WrBandwidthMbs: 3,
					RwBandwidthMbs: 4,
				},
			},
		})
		if len(stubRPC.params) != 1 {
			t.Fatalf("Expect only one call to SPDK, received %v", stubRPC.params)
		}
		qosParams := stubRPC.params[0].(*spdk.BdevQoSParams)
		expectedParams := spdk.BdevQoSParams{
			Name:           "volume-42",
			RwIosPerSec:    1000,
			RMbytesPerSec:  2,
			WMbytesPerSec:  3,
			RwMbytesPerSec: 4,
		}
		if *qosParams != expectedParams {
			t.Errorf("Expected qos params to be sent: %v, received %v", expectedParams, *qosParams)
		}
	})
}

func TestMiddleEnd_DeleteQosVolume(t *testing.T) {
	tests := map[string]struct {
		in          string
		spdk        []string
		errCode     codes.Code
		errMsg      string
		start       bool
		existBefore bool
		existAfter  bool
		missing     bool
	}{
		"qos volume does not exist": {
			in:          testQosVolumeID,
			spdk:        []string{},
			errCode:     codes.NotFound,
			errMsg:      fmt.Sprintf("unable to find key //storage.opiproject.org/volumes/%s", testQosVolumeID),
			start:       false,
			existBefore: false,
			existAfter:  false,
			missing:     false,
		},
		"qos volume does not exist, with allow_missing": {
			in:          testQosVolumeID,
			spdk:        []string{},
			errCode:     codes.OK,
			errMsg:      "",
			start:       false,
			existBefore: false,
			existAfter:  false,
			missing:     true,
		},
		"SPDK call failed": {
			in:          testQosVolumeID,
			spdk:        []string{`{"id":%d,"error":{"code":1,"message":"some internal error"},"result":true}`},
			errCode:     status.Convert(spdk.ErrFailedSpdkCall).Code(),
			errMsg:      status.Convert(spdk.ErrFailedSpdkCall).Message(),
			start:       true,
			existBefore: true,
			existAfter:  true,
			missing:     false,
		},
		"SPDK call result false": {
			in:          testQosVolumeID,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode:     status.Convert(spdk.ErrUnexpectedSpdkCallResult).Code(),
			errMsg:      status.Convert(spdk.ErrUnexpectedSpdkCallResult).Message(),
			start:       true,
			existBefore: true,
			existAfter:  true,
			missing:     false,
		},
		"successful deletion": {
			in:          testQosVolumeID,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode:     codes.OK,
			errMsg:      "",
			start:       true,
			existBefore: true,
			existAfter:  false,
			missing:     false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			fname1 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", tt.in)
			fname2 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", testQosVolumeID)

			request := &pb.DeleteQosVolumeRequest{Name: fname1}
			if tt.existBefore {
				testEnv.opiSpdkServer.volumes.qosVolumes[fname2] = testQosVolume
			}
			if tt.missing {
				request.AllowMissing = true
			}

			_, err := testEnv.client.DeleteQosVolume(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expect grpc error status")
			}

			_, ok := testEnv.opiSpdkServer.volumes.qosVolumes[fname1]
			if tt.existAfter != ok {
				t.Error("expect QoS volume exist", tt.existAfter, "received", ok)
			}
		})
	}
}

func TestMiddleEnd_UpdateQosVolume(t *testing.T) {
	fullname := fmt.Sprintf("//storage.opiproject.org/volumes/%s", testQosVolumeID)
	originalQosVolume := &pb.QosVolume{
		Name:     fullname,
		VolumeId: testQosVolume.VolumeId,
		MaxLimit: &pb.QosLimit{RdBandwidthMbs: 1221},
	}
	tests := map[string]struct {
		in          *pb.QosVolume
		out         *pb.QosVolume
		spdk        []string
		errCode     codes.Code
		errMsg      string
		start       bool
		existBefore bool
	}{
		"min_limit is not supported": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MinLimit: &pb.QosLimit{
					RdIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume min_limit is not supported",
			start:       false,
			existBefore: true,
		},
		"max_limit rd_iops_kiops is not supported": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RdIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rd_iops_kiops is not supported",
			start:       false,
			existBefore: true,
		},
		"max_limit wr_iops_kiops is not supported": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					WrIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit wr_iops_kiops is not supported",
			start:       false,
			existBefore: true,
		},
		"max_limit rw_iops_kiops is negative": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RwIopsKiops: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rw_iops_kiops cannot be negative",
			start:       false,
			existBefore: true,
		},
		"max_limit rd_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RdBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rd_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: true,
		},
		"max_limit wr_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					WrBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit wr_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: true,
		},
		"max_limit rw_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{
					RwBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit rw_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: true,
		},
		"max_limit with all zero limits": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume max_limit should set limit",
			start:       false,
			existBefore: true,
		},
		"qos_volume_id is empty": {
			in: &pb.QosVolume{
				Name:     "",
				VolumeId: &_go.ObjectKey{Value: "volume-42"},
				MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "qos_volume_id cannot be empty",
			start:       false,
			existBefore: true,
		},
		"volume_id is nil": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: nil,
				MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "volume_id cannot be empty",
			start:       false,
			existBefore: true,
		},
		"volume_id is empty": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: ""},
				MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "volume_id cannot be empty",
			start:       false,
			existBefore: true,
		},
		"qos volume does not exist": {
			in:          testQosVolume,
			out:         nil,
			spdk:        []string{},
			errCode:     codes.NotFound,
			errMsg:      fmt.Sprintf("volume_id %v does not exist", fullname),
			start:       false,
			existBefore: false,
		},
		"change underlying volume": {
			in: &pb.QosVolume{
				Name:     fullname,
				VolumeId: &_go.ObjectKey{Value: "new-underlying-volume-id"},
				MaxLimit: &pb.QosLimit{RdBandwidthMbs: 1},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg: fmt.Sprintf("Change of underlying volume %v to a new one %v is forbidden",
				originalQosVolume.VolumeId.Value, "new-underlying-volume-id"),
			start:       false,
			existBefore: true,
		},
		"SPDK call failed": {
			in:          testQosVolume,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":1,"message":"some internal error"},"result":true}`},
			errCode:     status.Convert(spdk.ErrFailedSpdkCall).Code(),
			errMsg:      status.Convert(spdk.ErrFailedSpdkCall).Message(),
			start:       true,
			existBefore: true,
		},
		"SPDK call result false": {
			in:          testQosVolume,
			out:         nil,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode:     status.Convert(spdk.ErrUnexpectedSpdkCallResult).Code(),
			errMsg:      status.Convert(spdk.ErrUnexpectedSpdkCallResult).Message(),
			start:       true,
			existBefore: true,
		},
		"successful update": {
			in:          testQosVolume,
			out:         testQosVolume,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode:     codes.OK,
			errMsg:      "",
			start:       true,
			existBefore: true,
		},
		"update with the same limit values": {
			in:          originalQosVolume,
			out:         originalQosVolume,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			errCode:     codes.OK,
			errMsg:      "",
			start:       true,
			existBefore: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			if tt.existBefore {
				testEnv.opiSpdkServer.volumes.qosVolumes[originalQosVolume.Name] = originalQosVolume
			}

			request := &pb.UpdateQosVolumeRequest{QosVolume: tt.in}
			response, err := testEnv.client.UpdateQosVolume(testEnv.ctx, request)

			marshalledOut, _ := proto.Marshal(tt.out)
			marshalledResponse, _ := proto.Marshal(response)
			if !bytes.Equal(marshalledOut, marshalledResponse) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expect grpc error status")
			}

			vol := testEnv.opiSpdkServer.volumes.qosVolumes[fullname]
			if tt.errCode == codes.OK {
				if !proto.Equal(tt.in, vol) {
					t.Error("expect QoS volume", vol, "is equal to", tt.in)
				}
			} else if tt.existBefore {
				if !proto.Equal(originalQosVolume, vol) {
					t.Error("expect QoS volume", originalQosVolume, "is preserved, received", vol)
				}
			}
		})
	}
}

func TestMiddleEnd_ListQosVolume(t *testing.T) {
	qosVolume0 := &pb.QosVolume{
		Name:     "qos-volume-41",
		VolumeId: &_go.ObjectKey{Value: "volume-41"},
		MaxLimit: &pb.QosLimit{RwBandwidthMbs: 1},
	}
	qosVolume1 := &pb.QosVolume{
		Name:     "qos-volume-45",
		VolumeId: &_go.ObjectKey{Value: "volume-45"},
		MaxLimit: &pb.QosLimit{RwBandwidthMbs: 5},
	}
	existingToken := "existing-pagination-token"
	tests := map[string]struct {
		out             []*pb.QosVolume
		existingVolumes map[string]*pb.QosVolume
		errCode         codes.Code
		errMsg          string
		size            int32
		token           string
	}{
		"no qos volumes were created": {
			out:             []*pb.QosVolume{},
			existingVolumes: map[string]*pb.QosVolume{},
			errCode:         codes.OK,
			errMsg:          "",
			size:            0,
			token:           "",
		},
		"qos volumes were created": {
			out: []*pb.QosVolume{qosVolume0, qosVolume1},
			existingVolumes: map[string]*pb.QosVolume{
				qosVolume0.Name: qosVolume0,
				qosVolume1.Name: qosVolume1,
			},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"pagination": {
			out: []*pb.QosVolume{qosVolume0},
			existingVolumes: map[string]*pb.QosVolume{
				qosVolume0.Name: qosVolume0,
				qosVolume1.Name: qosVolume1,
			},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
		},
		"pagination offset": {
			out: []*pb.QosVolume{qosVolume1},
			existingVolumes: map[string]*pb.QosVolume{
				qosVolume0.Name: qosVolume0,
				qosVolume1.Name: qosVolume1,
			},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   existingToken,
		},
		"pagination negative": {
			out: nil,
			existingVolumes: map[string]*pb.QosVolume{
				qosVolume0.Name: qosVolume0,
				qosVolume1.Name: qosVolume1,
			},
			errCode: codes.InvalidArgument,
			errMsg:  "negative PageSize is not allowed",
			size:    -10,
			token:   "",
		},
		"pagination error": {
			out: nil,
			existingVolumes: map[string]*pb.QosVolume{
				qosVolume0.Name: qosVolume0,
				qosVolume1.Name: qosVolume1,
			},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			size:    0,
			token:   "unknown-pagination-token",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()
			testEnv.opiSpdkServer.volumes.qosVolumes = tt.existingVolumes
			request := &pb.ListQosVolumesRequest{}
			request.PageSize = tt.size
			request.PageToken = tt.token
			testEnv.opiSpdkServer.Pagination[existingToken] = 1

			response, err := testEnv.client.ListQosVolumes(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expect grpc error status")
			}

			if response != nil {
				if len(tt.out) != len(response.QosVolumes) {
					t.Error("Expected", tt.out, "received", response.QosVolumes)
				} else {
					for _, expectVol := range tt.out {
						found := false
						for _, receivedVol := range response.QosVolumes {
							if proto.Equal(expectVol, receivedVol) {
								found = true
							}
						}
						if !found {
							t.Error("expect ", expectVol, "received in", response.QosVolumes)
						}
					}
				}
			}
		})
	}
}

func TestMiddleEnd_GetQosVolume(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.QosVolume
		errCode codes.Code
		errMsg  string
	}{
		"unknown QoS volume name": {
			in:      "unknown-qos-volume-id",
			out:     nil,
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %s", "//storage.opiproject.org/volumes/unknown-qos-volume-id"),
		},
		"existing QoS volume": {
			in:      testQosVolumeID,
			out:     testQosVolume,
			errCode: codes.OK,
			errMsg:  "",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(false, []string{})
			defer testEnv.Close()

			fname1 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", tt.in)
			fname2 := fmt.Sprintf("//storage.opiproject.org/volumes/%s", testQosVolumeID)
			testEnv.opiSpdkServer.volumes.qosVolumes[fname2] = testQosVolume

			request := &pb.GetQosVolumeRequest{Name: fname1}
			response, err := testEnv.client.GetQosVolume(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expect grpc error status")
			}

			if tt.errCode == codes.OK && !proto.Equal(tt.out, response) {
				t.Error("expect QoS volume", tt.out, "is equal to", response)
			}
		})
	}
}

func TestMiddleEnd_QosVolumeStats(t *testing.T) {
	fullname := fmt.Sprintf("//storage.opiproject.org/volumes/%s", testQosVolumeID)
	tests := map[string]struct {
		in      *_go.ObjectKey
		out     *pb.QosVolumeStatsResponse
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"empty QoS volume id is not allowed ": {
			in:      nil,
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "volume_id cannot be empty",
			start:   false,
		},
		"unknown QoS volume Id": {
			in:      &_go.ObjectKey{Value: "unknown-qos-volume-id"},
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %s", "unknown-qos-volume-id"),
			start:   false,
		},
		"SPDK call failed": {
			in:      &_go.ObjectKey{Value: fullname},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"some internal error"}}`},
			errCode: status.Convert(spdk.ErrFailedSpdkCall).Code(),
			errMsg:  status.Convert(spdk.ErrFailedSpdkCall).Message(),
			start:   true,
		},
		"SPDK call result false": {
			in:      &_go.ObjectKey{Value: fullname},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate": 3300000000,"ticks": 5,"bdevs":[]}}`},
			errCode: status.Convert(spdk.ErrUnexpectedSpdkCallResult).Code(),
			errMsg:  status.Convert(spdk.ErrUnexpectedSpdkCallResult).Message(),
			start:   true,
		},
		"successful QoS volume stats": {
			in: &_go.ObjectKey{Value: fullname},
			out: &pb.QosVolumeStatsResponse{
				Stats: &pb.VolumeStats{
					ReadBytesCount: 36864,
				},
				Id: &_go.ObjectKey{Value: fullname},
			},
			spdk: []string{
				`{"id":%d,"error":{"code":0,"message":""},"result":{"tick_rate": 3300000000,"ticks": 5,` +
					`"bdevs":[{"name":"` + testQosVolume.VolumeId.Value + `", "bytes_read": 36864}]}}`,
			},
			errCode: codes.OK,
			errMsg:  "",
			start:   true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.volumes.qosVolumes[fullname] = testQosVolume

			request := &pb.QosVolumeStatsRequest{VolumeId: tt.in}
			response, err := testEnv.client.QosVolumeStats(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Errorf("expect grpc error status")
			}

			if tt.errCode == codes.OK && !proto.Equal(tt.out, response) {
				t.Error("expect QoS volume stats", tt.out, "is equal to", response)
			}
		})
	}
}
