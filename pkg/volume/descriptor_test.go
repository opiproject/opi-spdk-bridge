// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package volume contains volume layer abstractions
package volume

import (
	"testing"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func TestNullVolumeReturnsItsOwnDescriptor(t *testing.T) {
	value := &pb.NullDebug{Handle: &pc.ObjectKey{Value: "Handle42"}, BlockSize: 4096}
	nullVol0 := NewNullVolume(value)
	val := nullVol0.Descriptor().ToNullDebug()
	if val != value {
		t.Errorf("Expect %v equal to %v", val, value)
	}
}

func TestQosVolumeReturnsItsOwnDescriptor(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Handle42"}, BlockSize: 4096})
	registry.Add("nullId0", nullVol0)
	value := &pb.QosVolume{QosVolumeId: &pc.ObjectKey{Value: "Qos42"}}
	qosVol0, _ := nullVol0.CreateQosVolume(value)
	val := qosVol0.Descriptor().ToQosVolume()
	if val != value {
		t.Errorf("Expect %v equal to %v", val, value)
	}
}

func TestEncryptedVolumeReturnsItsOwnDescriptor(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Handle42"}, BlockSize: 4096})
	registry.Add("nullId0", nullVol0)
	value := &pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "Encr42"}}
	encrVol0, _ := nullVol0.CreateEncryptedVolume(value)
	val := encrVol0.Descriptor().ToEncryptedVolume()
	if val != value {
		t.Errorf("Expect %v equal to %v", val, value)
	}
}

func TestVolumeDescriptorConvertToInvalidType(t *testing.T) {
	value := &pb.NullDebug{Handle: &pc.ObjectKey{Value: "Handle42"}, BlockSize: 4096}
	nullVol0 := NewNullVolume(value)
	val := nullVol0.Descriptor().ToQosVolume()
	if val != nil {
		t.Errorf("Expect invalid type conversion to Qos Volume")
	}
}
