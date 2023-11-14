// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"errors"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

type stubNvmeTransport struct {
	err error
}

func (t *stubNvmeTransport) Params(_ *pb.NvmeController, _ *pb.NvmeSubsystem) (any, error) {
	return spdk.NvmfSubsystemAddListenerParams{}, t.err
}

var (
	alwaysValidNvmeTransport  = &stubNvmeTransport{}
	alwaysValidNvmeTransports = map[pb.NvmeTransportType]NvmeTransport{
		pb.NvmeTransportType_NVME_TRANSPORT_TCP: alwaysValidNvmeTransport,
	}
	alwaysFailedNvmeTransport  = &stubNvmeTransport{errors.New("some transport error")}
	alwaysFailedNvmeTransports = map[pb.NvmeTransportType]NvmeTransport{
		pb.NvmeTransportType_NVME_TRANSPORT_TCP: alwaysFailedNvmeTransport,
	}
)
