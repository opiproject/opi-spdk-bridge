// SPDX-License-Identifier: Apache-2.0
package middleend

import (
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func TestMiddleendStatisticsClockContext(t *testing.T) {
	const name = "volumes/metadata"
	for _, test := range []struct {
		name string
		call func(*testEnv) (*pb.StatsSampleMetadata, *pb.VolumeStats, error)
	}{
		{"encryption", func(env *testEnv) (*pb.StatsSampleMetadata, *pb.VolumeStats, error) {
			env.opiSpdkServer.volumes.encVolumes[name] = &pb.EncryptedVolume{Name: name}
			response, err := env.client.StatsEncryptedVolume(env.ctx, &pb.StatsEncryptedVolumeRequest{Name: name})
			return response.GetSampleMetadata(), response.GetStats(), err
		}},
		{"qos", func(env *testEnv) (*pb.StatsSampleMetadata, *pb.VolumeStats, error) {
			env.opiSpdkServer.volumes.qosVolumes[name] = &pb.QosVolume{Name: name, VolumeNameRef: "metadata"}
			response, err := env.client.StatsQosVolume(env.ctx, &pb.StatsQosVolumeRequest{Name: name})
			return response.GetSampleMetadata(), response.GetStats(), err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := createTestEnvironment([]string{
				`{"id":%d,"result":{"tick_rate":1000,"ticks":2000,"bdevs":[{"name":"metadata","num_read_ops":12,"read_latency_ticks":70}]}}`,
			})
			defer env.Close()
			metadata, stats, err := test.call(env)
			if err != nil {
				t.Fatal(err)
			}
			if metadata.GetTickRateHz() != 1000 || metadata.GetSampleTicks() != 2000 ||
				metadata.CollectionTime == nil || metadata.CollectionTime.CheckValid() != nil ||
				metadata.CounterEpoch != "" || metadata.CounterStartTime != nil {
				t.Fatal("middle-end response lost actual clock context or invented continuity")
			}
			if stats.GetReadOpsCount() != 12 || stats.GetReadLatencyTicks() != 70 {
				t.Fatal("existing measurements must remain unchanged")
			}
		})
	}
}
