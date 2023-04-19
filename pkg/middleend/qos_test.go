// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/opiproject/gospdk/spdk"
	_go "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	testQosVolume = &pb.QosVolume{
		QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
		VolumeId:    &_go.ObjectKey{Value: "volume-42"},
		LimitMax:    &pb.QosLimit{RwBandwidthMbs: 1},
	}
)

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
		"limit_min is not supported": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMin: &pb.QosLimit{
					RdIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_min is not supported",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"limit_max rd_iops_kiops is not supported": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax: &pb.QosLimit{
					RdIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_max rd_iops_kiops is not supported",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"limit_max wr_iops_kiops is not supported": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax: &pb.QosLimit{
					WrIopsKiops: 100000,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_max wr_iops_kiops is not supported",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"limit_max rw_iops_kiops is negative": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax: &pb.QosLimit{
					RwIopsKiops: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_max rw_iops_kiops cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"limit_max rd_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax: &pb.QosLimit{
					RdBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_max rd_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"limit_max wr_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax: &pb.QosLimit{
					WrBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_max wr_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"limit_max rw_bandwidth_kiops is negative": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax: &pb.QosLimit{
					RwBandwidthMbs: -1,
				},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_max rw_bandwidth_mbs cannot be negative",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"limit_max with all zero limits": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax:    &pb.QosLimit{},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "QoS volume limit_max should set limit",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"qos_volume_id is nil": {
			in: &pb.QosVolume{
				QosVolumeId: nil,
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax:    &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "qos_volume_id cannot be empty",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"qos_volume_id is empty": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: ""},
				VolumeId:    &_go.ObjectKey{Value: "volume-42"},
				LimitMax:    &pb.QosLimit{RwBandwidthMbs: 1},
			},
			out:         nil,
			spdk:        []string{},
			errCode:     codes.InvalidArgument,
			errMsg:      "qos_volume_id cannot be empty",
			start:       false,
			existBefore: false,
			existAfter:  false,
		},
		"volume_id is nil": {
			in: &pb.QosVolume{
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    nil,
				LimitMax:    &pb.QosLimit{RwBandwidthMbs: 1},
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
				QosVolumeId: &_go.ObjectKey{Value: "qos-volume-42"},
				VolumeId:    &_go.ObjectKey{Value: ""},
				LimitMax:    &pb.QosLimit{RwBandwidthMbs: 1},
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
			request := &pb.CreateQosVolumeRequest{QosVolume: tt.in}
			if tt.existBefore {
				testEnv.opiSpdkServer.volumes.qosVolumes[tt.in.QosVolumeId.Value] = tt.in
			}

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

			vol, ok := testEnv.opiSpdkServer.volumes.qosVolumes[testQosVolume.QosVolumeId.Value]
			if tt.existAfter != ok {
				t.Error("expect QoS volume exist", tt.existAfter, "received", ok)
			}
			if tt.existAfter && !proto.Equal(tt.in, vol) {
				t.Error("expect QoS volume ", vol, "is equal to", tt.in)
			}
		})
	}
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
			in:          testQosVolume.QosVolumeId.Value,
			spdk:        []string{},
			errCode:     codes.NotFound,
			errMsg:      fmt.Sprintf("unable to find key %s", testQosVolume.QosVolumeId.Value),
			start:       false,
			existBefore: false,
			existAfter:  false,
			missing:     false,
		},
		"qos volume does not exist, with allow_missing": {
			in:          testQosVolume.QosVolumeId.Value,
			spdk:        []string{},
			errCode:     codes.OK,
			errMsg:      "",
			start:       false,
			existBefore: false,
			existAfter:  false,
			missing:     true,
		},
		"SPDK call failed": {
			in:          testQosVolume.QosVolumeId.Value,
			spdk:        []string{`{"id":%d,"error":{"code":1,"message":"some internal error"},"result":true}`},
			errCode:     status.Convert(spdk.ErrFailedSpdkCall).Code(),
			errMsg:      status.Convert(spdk.ErrFailedSpdkCall).Message(),
			start:       true,
			existBefore: true,
			existAfter:  true,
			missing:     false,
		},
		"SPDK call result false": {
			in:          testQosVolume.QosVolumeId.Value,
			spdk:        []string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			errCode:     status.Convert(spdk.ErrUnexpectedSpdkCallResult).Code(),
			errMsg:      status.Convert(spdk.ErrUnexpectedSpdkCallResult).Message(),
			start:       true,
			existBefore: true,
			existAfter:  true,
			missing:     false,
		},
		"successful deletion": {
			in:          testQosVolume.QosVolumeId.Value,
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
			request := &pb.DeleteQosVolumeRequest{Name: tt.in}
			if tt.existBefore {
				testEnv.opiSpdkServer.volumes.qosVolumes[testQosVolume.QosVolumeId.Value] = testQosVolume
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

			_, ok := testEnv.opiSpdkServer.volumes.qosVolumes[tt.in]
			if tt.existAfter != ok {
				t.Error("expect QoS volume exist", tt.existAfter, "received", ok)
			}
		})
	}
}
