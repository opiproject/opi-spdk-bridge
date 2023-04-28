// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package volume contains volume layer abstractions
package volume

import (
	"fmt"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

type volumeType int

const (
	// Backend volumes
	aioVolumeType volumeType = iota
	nullVolumeType
	remoteVolumeType
	// Middleend volumes
	encryptedVolumeType
	qosVolumeType
)

type Volume struct {
	bdevName   string
	kind       volumeType
	stack      *volumeStack
	descriptor Descriptor
}

func NewAioVolume(descr *pb.AioController) *Volume {
	return newBackendVolume(descr.Handle.Value, aioVolumeType, descr)
}

func NewNullVolume(descr *pb.NullDebug) *Volume {
	return newBackendVolume(descr.Handle.Value, nullVolumeType, descr)
}

func newBackendVolume(bdevName string, kind volumeType, descr any) *Volume {
	// check bdevName is not empty
	// check kind
	// nil check
	vol := Volume{
		bdevName:   bdevName,
		kind:       kind,
		stack:      newVolumeStack(),
		descriptor: Descriptor{value: descr},
	}
	return &vol
}

func (v *Volume) CreateEncryptedVolume(descr *pb.EncryptedVolume) (*Volume, error) {
	vol := newMiddleendVolume(descr.EncryptedVolumeId.Value, encryptedVolumeType, descr, v.stack)
	if err := v.canCreate(vol); err != nil {
		return nil, err
	}
	return vol, nil
}

func (v *Volume) CreateQosVolume(descr *pb.QosVolume) (*Volume, error) {
	// QoS in SPDK does not create its own bdev, only applies limits to existing bdev.
	// Copy parent bdev name
	parent := v.stack.top()
	if parent == nil {
		return nil, fmt.Errorf("no underlying volume found to create volume")
	}
	vol := newMiddleendVolume(parent.bdevName, qosVolumeType, descr, v.stack)
	if err := v.canCreate(vol); err != nil {
		return nil, err
	}
	return vol, nil
}

func newMiddleendVolume(bdevName string, kind volumeType, descr any, stack *volumeStack) *Volume {
	// check kind
	// nil check
	vol := Volume{
		bdevName:   bdevName,
		kind:       kind,
		stack:      stack,
		descriptor: Descriptor{value: descr},
	}
	return &vol
}

func (v *Volume) BdevName() string {
	return v.bdevName
}

func (v *Volume) Descriptor() Descriptor {
	return v.descriptor
}

func (v *Volume) canBeDeleted() error {
	topVol := v.stack.top()
	if topVol == nil {
		// should be unreachable if object is used through public API... panic?
		return fmt.Errorf("no volume to delete")
	}
	if !v.equal(topVol) {
		return fmt.Errorf("only volume on top can be deleted")
	}
	return nil
}

func (v *Volume) canCreate(vol *Volume) error {
	topVol := v.stack.top()
	if topVol == nil {
		return fmt.Errorf("no underlying volume found to create volume")
	}
	if !v.equal(topVol) {
		return fmt.Errorf("volume is not on top of stack")
	}
	if vol.isBackend() {
		// should be unreachable if object is created by public API... panic?
		return fmt.Errorf("cannot add backend volume")
	}
	if v.stack.hasType(vol.kind) {
		return fmt.Errorf("volume of that type already exists")
	}

	return nil
}

func (v *Volume) addToStack() {
	v.stack.push(v)
}

func (v *Volume) removeFromStack() {
	v.stack.pop()
}

func (v *Volume) isBackend() bool {
	backendVolumeTypes := map[volumeType]struct{}{
		aioVolumeType:    {},
		nullVolumeType:   {},
		remoteVolumeType: {},
	}
	_, ok := backendVolumeTypes[v.kind]
	return ok
}

func (v *Volume) isMiddleend() bool {
	return !v.isBackend()
}

func (v *Volume) equal(other *Volume) bool {
	return *v == *other
}
