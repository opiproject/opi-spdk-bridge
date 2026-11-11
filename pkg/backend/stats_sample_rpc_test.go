// SPDX-License-Identifier: Apache-2.0
package backend

import (
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBackendStatisticsClockContext(t *testing.T) {
	const name = "volumes/metadata"
	for _, test := range []struct {
		name string
		call func(*testEnv) (*pb.StatsSampleMetadata, *pb.VolumeStats, error)
	}{
		{"aio", func(env *testEnv) (*pb.StatsSampleMetadata, *pb.VolumeStats, error) {
			env.opiSpdkServer.Volumes.AioVolumes[name] = &pb.AioVolume{Name: name}
			response, err := env.client.StatsAioVolume(env.ctx, &pb.StatsAioVolumeRequest{Name: name})
			return response.GetSampleMetadata(), response.GetStats(), err
		}},
		{"malloc", func(env *testEnv) (*pb.StatsSampleMetadata, *pb.VolumeStats, error) {
			env.opiSpdkServer.Volumes.MallocVolumes[name] = &pb.MallocVolume{Name: name}
			response, err := env.client.StatsMallocVolume(env.ctx, &pb.StatsMallocVolumeRequest{Name: name})
			return response.GetSampleMetadata(), response.GetStats(), err
		}},
		{"null", func(env *testEnv) (*pb.StatsSampleMetadata, *pb.VolumeStats, error) {
			env.opiSpdkServer.Volumes.NullVolumes[name] = &pb.NullVolume{Name: name}
			response, err := env.client.StatsNullVolume(env.ctx, &pb.StatsNullVolumeRequest{Name: name})
			return response.GetSampleMetadata(), response.GetStats(), err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := createTestEnvironment([]string{
				`{"id":%d,"result":{"tick_rate":1000,"ticks":9007199254740993,"bdevs":[{"name":"metadata","bytes_read":120,"num_read_ops":12,"read_latency_ticks":70}]}}`,
			})
			defer env.Close()
			metadata, stats, err := test.call(env)
			if err != nil {
				t.Fatal(err)
			}
			if metadata == nil || metadata.CollectionTime == nil || metadata.CollectionTime.CheckValid() != nil ||
				metadata.GetTickRateHz() != 1000 || metadata.GetSampleTicks() != 9007199254740993 {
				t.Fatal("RPC response lost actual source clock context or precision")
			}
			if metadata.CounterEpoch != "" || metadata.CounterStartTime != nil {
				t.Fatal("RPC response fabricated an epoch not supplied by SPDK")
			}
			if stats.GetReadBytesCount() != 120 || stats.GetReadOpsCount() != 12 || stats.GetReadLatencyTicks() != 70 {
				t.Fatal("adding metadata must preserve existing counter values")
			}
		})
	}
}

func TestStatisticsMissingAndInvalidClock(t *testing.T) {
	const name = "volumes/metadata"
	for _, test := range []struct {
		name, response string
		code           codes.Code
	}{
		{"missing clock", `{"id":%d,"result":{"bdevs":[{"name":"metadata","num_read_ops":12}]}}`, codes.OK},
		{"invalid reported frequency", `{"id":%d,"result":{"tick_rate":0,"ticks":10,"bdevs":[{"name":"metadata","num_read_ops":12}]}}`, codes.DataLoss},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := createTestEnvironment([]string{test.response})
			defer env.Close()
			env.opiSpdkServer.Volumes.AioVolumes[name] = &pb.AioVolume{Name: name}
			response, err := env.client.StatsAioVolume(env.ctx, &pb.StatsAioVolumeRequest{Name: name})
			if status.Code(err) != test.code {
				t.Fatalf("got %v, want %s", err, test.code)
			}
			if err == nil && (response.GetSampleMetadata().TickRateHz != nil || response.GetSampleMetadata().SampleTicks != nil) {
				t.Fatal("omitted backend clock fields must remain absent over gRPC")
			}
		})
	}
}
