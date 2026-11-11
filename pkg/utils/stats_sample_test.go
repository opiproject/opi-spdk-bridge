// SPDX-License-Identifier: Apache-2.0
package utils

import (
	"encoding/json"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestIostatSampleMetadata(t *testing.T) {
	for _, test := range []struct {
		name        string
		rate        *int64
		ticks       *int64
		wantErrCode codes.Code
	}{
		{"reported clock", proto.Int64(2400000000), proto.Int64(9007199254740993), codes.OK},
		{"valid tick zero", proto.Int64(1000), proto.Int64(0), codes.OK},
		{"legacy clock omitted", nil, nil, codes.OK},
		{"unknown frequency", nil, proto.Int64(1000), codes.OK},
		{"unknown sample ticks", proto.Int64(1000), nil, codes.OK},
		{"invalid frequency", proto.Int64(-1), proto.Int64(1000), codes.DataLoss},
		{"zero frequency is not omission", proto.Int64(0), proto.Int64(1000), codes.DataLoss},
		{"invalid ticks", proto.Int64(1000), proto.Int64(-1), codes.DataLoss},
	} {
		t.Run(test.name, func(t *testing.T) {
			metadata, err := IostatSampleMetadata(IostatSnapshot{TickRate: test.rate, Ticks: test.ticks})
			if status.Code(err) != test.wantErrCode {
				t.Fatalf("got %v, want %s", err, test.wantErrCode)
			}
			if err != nil {
				if metadata != nil {
					t.Fatal("invalid source data must not produce success-shaped metadata")
				}
				return
			}
			if metadata.CollectionTime == nil || metadata.CollectionTime.CheckValid() != nil {
				t.Fatal("a received snapshot needs valid observation time")
			}
			if (metadata.TickRateHz != nil) != (test.rate != nil) || (metadata.SampleTicks != nil) != (test.ticks != nil) {
				t.Fatal("unknown source-clock values must remain absent")
			}
			if test.rate != nil && metadata.GetTickRateHz() != *test.rate {
				t.Fatal("source frequency lost its original value")
			}
			if test.ticks != nil && metadata.GetSampleTicks() != *test.ticks {
				t.Fatal("source clock lost its original value or precision")
			}
			if metadata.CounterEpoch != "" || metadata.CounterStartTime != nil {
				t.Fatal("SPDK iostat does not establish a counter epoch or start time")
			}
		})
	}
}

func TestIostatJSONClockPresence(t *testing.T) {
	for _, test := range []struct {
		payload string
		rate    bool
		ticks   bool
	}{
		{`{"tick_rate":1000,"bdevs":[{"name":"v","num_read_ops":12}]}`, true, false},
		{`{"ticks":0,"bdevs":[{"name":"v","num_read_ops":12}]}`, false, true},
		{`{"tick_rate":1000,"ticks":9007199254740993,"bdevs":[{"name":"v","num_read_ops":12}]}`, true, true},
	} {
		var snapshot IostatSnapshot
		if err := json.Unmarshal([]byte(test.payload), &snapshot); err != nil {
			t.Fatal(err)
		}
		metadata, err := IostatSampleMetadata(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if (metadata.TickRateHz != nil) != test.rate || (metadata.SampleTicks != nil) != test.ticks ||
			len(snapshot.Bdevs) != 1 || snapshot.Bdevs[0].NumReadOps != 12 {
			t.Fatal("clock presence or reused SPDK counters were lost")
		}
	}
}
