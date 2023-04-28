// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package volume contains volume layer abstractions
package volume

type volumeStack struct {
	volumes []*Volume
}

func newVolumeStack() *volumeStack {
	return &volumeStack{}
}

func (s *volumeStack) top() *Volume {
	i := len(s.volumes) - 1
	if i < 0 {
		return nil
	}
	val := s.volumes[i]
	return val
}

func (s *volumeStack) push(vol *Volume) {
	s.volumes = append(s.volumes, vol)
}

func (s *volumeStack) pop() *Volume {
	vol := s.top()
	if vol == nil {
		return nil
	}
	s.volumes = s.volumes[:len(s.volumes)-1]
	return vol
}

func (s *volumeStack) hasType(kind volumeType) bool {
	for _, v := range s.volumes {
		if v.kind == kind {
			return true
		}
	}
	return false
}
