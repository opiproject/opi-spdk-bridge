// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package volume contains volume layer abstractions
package volume

import (
	"reflect"
	"testing"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func TestAddDeviceToRegistry(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})

	err := registry.Add("nullId0", nullVol0)

	if err != nil {
		t.Error("Expected nil error, received", err)
	}
}

func TestAddNewDeviceWithExistingID(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	nullVol1 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null1"}})
	id := "nullId0"
	registry.Add(id, nullVol0)

	err := registry.Add(id, nullVol1)

	if err == nil {
		t.Error("Expected error, received nil")
	}
}

func TestCreateMiddleendVolumeOnlyOnVolumeInRegistry(t *testing.T) {
	registry := NewRegistry()
	id := "nullId0"
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add(id, nullVol0)

	vol, err := nullVol0.CreateQosVolume(&pb.QosVolume{})

	if err != nil {
		t.Error("Expected nil error, received", err)
	}
	if vol == nil {
		t.Error("Expected not nil volume")
	}
}

func TestFailedToCreateMiddleendVolumeOnVolumeNotAddedToRegistry(t *testing.T) {
	registry := NewRegistry()
	id := "nullId0"
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add(id, nullVol0)
	encVol0, _ := nullVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol0"}})

	vol, err := encVol0.CreateQosVolume(&pb.QosVolume{})

	if err == nil {
		t.Error("Expected error, received nil")
	}
	if vol != nil {
		t.Error("Expected nil volume")
	}
}

func TestFailedToAddVolumeToVolumeWithAnotherVolume(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add("nullId0", nullVol0)
	encVol0, _ := nullVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol0"}})
	registry.Add("encrId0", encVol0)

	vol, err := nullVol0.CreateQosVolume(&pb.QosVolume{})

	if err == nil {
		t.Error("Expected error, received nil")
	}
	if vol != nil {
		t.Error("Expected nil volume")
	}
}

func TestFailedToAddVolumeOfAlreadyAddedType(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add("nullId0", nullVol0)
	encVol0, _ := nullVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol0"}})
	registry.Add("encrId0", encVol0)

	vol, err := encVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol1"}})

	if err == nil {
		t.Error("Expected error, received nil")
	}
	if vol != nil {
		t.Error("Expected nil volume")
	}
}

func TestCreateQosVolumeOnBackendVolumeNotAddedToRegistry(t *testing.T) {
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})

	vol, err := nullVol0.CreateQosVolume(&pb.QosVolume{})

	if err == nil {
		t.Error("Expected error, received nil")
	}
	if vol != nil {
		t.Error("Expected nil volume")
	}
}

func TestCreateEncryptedVolumeOnBackendVolumeNotAddedToRegistry(t *testing.T) {
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})

	vol, err := nullVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol0"}})

	if err == nil {
		t.Error("Expected error, received nil")
	}
	if vol != nil {
		t.Error("Expected nil volume")
	}
}

func TestBackendVolumeReturnsItsOwnBdevName(t *testing.T) {
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})

	if nullVol0.BdevName() != "Null0" {
		t.Error("Expected Null0 as bdev name, received", nullVol0.BdevName())
	}
}

func TestQosVolumeReturnsBdevNameOfUnderlyingVolume(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add("nullId0", nullVol0)
	encVol0, _ := nullVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol0"}})
	registry.Add("encrId0", encVol0)

	qosVol0, _ := encVol0.CreateQosVolume(&pb.QosVolume{})

	if qosVol0.BdevName() != "EncVol0" {
		t.Error("Expected EncVol0 as bdev name, received", qosVol0.BdevName())
	}
}

func TestEncryptedVolumeReturnsItsOwnBdevName(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add("nullId0", nullVol0)
	qosVol0, _ := nullVol0.CreateQosVolume(&pb.QosVolume{})
	registry.Add("qosId0", qosVol0)

	encVol0, _ := qosVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol0"}})

	if encVol0.BdevName() != "EncVol0" {
		t.Error("Expected EncVol0 as bdev name, received", encVol0.BdevName())
	}
}

func TestCannotDeleteNotAddedDevice(t *testing.T) {
	registry := NewRegistry()
	id := "nullId0"

	err := registry.Delete(id)

	if err == nil {
		t.Error("Expected error, received nil")
	}
}

func TestDeleteAddedDevice(t *testing.T) {
	registry := NewRegistry()
	id := "nullId0"
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add(id, nullVol0)

	err := registry.Delete(id)

	if err != nil {
		t.Error("Expected nil error, received", err)
	}
}

func TestDeleteVolumeNotFromTopOfStack(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	registry.Add("nullId0", nullVol0)
	encVol0, _ := nullVol0.CreateEncryptedVolume(&pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: "EncVol0"}})
	registry.Add("encrId0", encVol0)
	qosVol0, _ := encVol0.CreateQosVolume(&pb.QosVolume{})
	registry.Add("qosVol0", qosVol0)

	err := registry.Delete("encrId0")

	if err == nil {
		t.Error("Expected error, received nil")
	}
}

func TestAddedDeviceCanBeFound(t *testing.T) {
	registry := NewRegistry()
	nullVol0 := NewNullVolume(&pb.NullDebug{Handle: &pc.ObjectKey{Value: "Null0"}})
	id := "nullId0"
	registry.Add(id, nullVol0)

	foundVol := registry.Find(id)

	if foundVol == nil {
		t.Error("Expected to find volume")
	}
	if !reflect.DeepEqual(nullVol0, foundVol) {
		t.Error("Expected", nullVol0, "found", foundVol)
	}
}

func TestNotAddedDeviceCannotBeFound(t *testing.T) {
	registry := NewRegistry()
	id := "nullId0"

	foundVol := registry.Find(id)

	if foundVol != nil {
		t.Error("Expected volume not found")
	}
}
