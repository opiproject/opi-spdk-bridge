// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package volume contains volume layer abstractions
package volume

import pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"

type Descriptor struct {
	value any
}

// return copies??

func (d Descriptor) ToNullDebug() *pb.NullDebug {
	val, ok := d.value.(*pb.NullDebug)
	if ok {
		return val
	}
	return nil
}

func (d Descriptor) ToQosVolume() *pb.QosVolume {
	val, ok := d.value.(*pb.QosVolume)
	if ok {
		return val
	}
	return nil
}

func (d Descriptor) ToEncryptedVolume() *pb.EncryptedVolume {
	val, ok := d.value.(*pb.EncryptedVolume)
	if ok {
		return val
	}
	return nil
}
