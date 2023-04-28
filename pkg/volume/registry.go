// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package volume contains volume layer abstractions
package volume

import "fmt"

type Registry struct {
	volumes map[string]*Volume
}

func NewRegistry() *Registry {
	return &Registry{
		volumes: make(map[string]*Volume),
	}
}

func (r *Registry) Find(ID string) *Volume {
	vol, ok := r.volumes[ID]
	if !ok {
		return nil
	}
	return vol
}

func (r *Registry) Add(ID string, volume *Volume) error {
	_, ok := r.volumes[ID]
	if ok {
		return fmt.Errorf("Volume %v already exists", ID)
	}
	volume.addToStack()
	r.volumes[ID] = volume
	return nil
}

func (r *Registry) Delete(ID string) error {
	vol, ok := r.volumes[ID]
	if !ok {
		return fmt.Errorf("Volume %v not found", ID)
	}
	if err := vol.canBeDeleted(); err != nil {
		return err
	}

	vol.removeFromStack()
	delete(r.volumes, ID)
	return nil
}
