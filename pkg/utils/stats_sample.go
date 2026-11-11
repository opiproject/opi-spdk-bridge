// SPDX-License-Identifier: Apache-2.0
package utils

import (
	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// IostatSnapshot reuses SPDK's counters while preserving source-clock presence.
type IostatSnapshot struct {
	spdk.BdevGetIostatResult
	TickRate *int64 `json:"tick_rate"`
	Ticks    *int64 `json:"ticks"`
}

// IostatSampleMetadata preserves clock context actually reported by SPDK.
func IostatSampleMetadata(result IostatSnapshot) (*pb.StatsSampleMetadata, error) {
	if (result.TickRate != nil && *result.TickRate <= 0) || (result.Ticks != nil && *result.Ticks < 0) {
		return nil, status.Error(codes.DataLoss, "SPDK returned invalid statistics clock data")
	}
	metadata := &pb.StatsSampleMetadata{CollectionTime: timestamppb.Now()}
	if result.TickRate != nil {
		metadata.TickRateHz = proto.Int64(*result.TickRate)
	}
	if result.Ticks != nil {
		metadata.SampleTicks = proto.Int64(*result.Ticks)
	}
	return metadata, nil
}
