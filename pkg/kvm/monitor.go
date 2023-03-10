// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
	qmpraw "github.com/digitalocean/go-qemu/qmp/raw"
)

// TODO: check for device existence to provide idempotence in all methods

type monitor struct {
	rmon    *qmpraw.Monitor
	mon     qmp.Monitor
	timeout time.Duration
}

func newMonitor(qmpAddress string, protocol string, timeout time.Duration) (*monitor, error) {
	mon, err := qmp.NewSocketMonitor(protocol, qmpAddress, timeout)
	if err != nil {
		log.Printf("couldn't create QEMU monitor: %v", err)
		return nil, err
	}

	if err := mon.Connect(); err != nil {
		log.Printf("Failed to connect to QEMU: %v", err)
		return nil, err
	}

	rawMon := qmpraw.NewMonitor(mon)
	return &monitor{rawMon, mon, timeout}, nil
}

func (m *monitor) Disconnect() {
	err := m.mon.Disconnect()
	if err != nil {
		log.Fatalf("Failed to disconnect QMP monitor %v", err)
	}
}

func (m *monitor) AddChardev(id string, sockPath string) error {
	server := false
	socketBackend := qmpraw.ChardevBackendSocket{
		Addr: qmpraw.SocketAddressLegacyUnix{
			Path: sockPath},
		Server: &server}
	_, err := m.rmon.ChardevAdd(id, socketBackend)
	return err
}

func (m *monitor) DeleteChardev(id string) error {
	return m.rmon.ChardevRemove(id)
}

func (m *monitor) AddVirtioBlkDevice(id string, chardevID string) error {
	qmpCmd := struct {
		Driver  string  `json:"driver"`
		ID      *string `json:"id,omitempty"`
		Chardev *string `json:"chardev,omitempty"`
	}{
		Driver:  "vhost-user-blk-pci",
		ID:      &id,
		Chardev: &chardevID,
	}
	bs, err := json.Marshal(map[string]interface{}{
		"execute":   "device_add",
		"arguments": qmpCmd,
	})
	if err != nil {
		log.Println("json marshalling error:", err)
		return fmt.Errorf("couldn't create QMP command: %w", err)
	}

	log.Println("QMP command to send: ", string(bs))
	raw, err := m.mon.Run(bs)
	if err != nil {
		log.Println("QMP error:", err)
		return fmt.Errorf("couldn't run QMP command: %w", err)
	}

	response := string(raw)
	log.Println("QMP response:", response)
	if strings.Contains(response, "error") {
		return fmt.Errorf("qemu cmd run error: %v", string(bs))
	}

	return nil
}
