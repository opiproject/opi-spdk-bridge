// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
	qmpraw "github.com/digitalocean/go-qemu/qmp/raw"
)

// TODO: check for device existence to provide idempotence in all methods

type monitor struct {
	rmon *qmpraw.Monitor
	mon  qmp.Monitor

	waitEventTimeout          time.Duration
	pollDevicePresenceTimeout time.Duration
	pollDevicePresenceStep    time.Duration
}

func newMonitor(qmpAddress string, protocol string,
	timeout time.Duration, pollDevicePresenceStep time.Duration) (*monitor, error) {
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
	return &monitor{
		rmon:                      rawMon,
		mon:                       mon,
		waitEventTimeout:          timeout,
		pollDevicePresenceTimeout: timeout,
		pollDevicePresenceStep:    pollDevicePresenceStep}, nil
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
	if err := m.addDevice(qmpCmd); err != nil {
		return err
	}
	return m.waitForDeviceExist(id)
}

func (m *monitor) AddNvmeControllerDevice(id string, ctrlrDir string, location deviceLocation) error {
	socket := filepath.Join(ctrlrDir, "cntrl")
	qmpCmd := struct {
		Driver string  `json:"driver"`
		ID     *string `json:"id,omitempty"`
		Bus    *string `json:"bus,omitempty"`
		Addr   *string `json:"addr,omitempty"`
		Socket *string `json:"socket,omitempty"`
	}{
		Driver: "vfio-user-pci",
		ID:     &id,
		Bus:    location.Bus,
		Addr:   location.Addr,
		Socket: &socket,
	}
	if err := m.addDevice(qmpCmd); err != nil {
		return err
	}
	return m.waitForDeviceExist(id)
}

func (m *monitor) DeleteVirtioBlkDevice(id string) error {
	err := m.rmon.DeviceDel(id)
	if err != nil {
		return fmt.Errorf("couldn't delete device: %w", err)
	}
	return m.waitForEvent("DEVICE_DELETED", "device", id)
}

func (m *monitor) DeleteNvmeControllerDevice(id string) error {
	if err := m.rmon.DeviceDel(id); err != nil {
		return err
	}
	return m.waitForDeviceNotExist(id)
}

func (m *monitor) addDevice(qmpCmd interface{}) error {
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

func (m *monitor) waitForEvent(event string, key string, value string) error {
	stream, err := m.mon.Events(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't get event channel: %v", err)
	}

	timeoutTimer := time.NewTimer(m.waitEventTimeout)
	for {
		select {
		case e := <-stream:
			log.Println("qemu event:", e)
			if e.Event != event {
				continue
			}
			v, ok := e.Data[key]
			if !ok {
				continue
			}
			val, ok := v.(string)
			if !ok {
				continue
			}
			if val != value {
				continue
			}
			log.Println("Event:", event, "found")
			return nil
		case <-timeoutTimer.C:
			log.Println("Event timeout:", event, ", key:", key, "value:", value)
			return fmt.Errorf("qemu event not found: %v", event)
		}
	}
}

func (m *monitor) waitForDeviceExist(id string) error {
	return m.waitForDevicePresence(id, true)
}

func (m *monitor) waitForDeviceNotExist(id string) error {
	return m.waitForDevicePresence(id, false)
}

func (m *monitor) waitForDevicePresence(id string, shouldExist bool) error {
	timeoutTimer := time.NewTimer(m.pollDevicePresenceTimeout)
	devicePresenceTicker := time.NewTicker(m.pollDevicePresenceStep)
	defer devicePresenceTicker.Stop()
	for {
		select {
		case <-timeoutTimer.C:
			return fmt.Errorf("timeout waiting for PCI device %v presence %v", id, shouldExist)
		case <-devicePresenceTicker.C:
			exist, err := m.pciDeviceExist(id)
			if err != nil {
				log.Println("failed to check pci device existence:", err)
				continue
			}
			if exist != shouldExist {
				continue
			}
			return nil
		}
	}
}

func (m *monitor) pciDeviceExist(id string) (bool, error) {
	pci, err := m.rmon.QueryPCI()
	if err != nil {
		return false, err
	}

	for _, pciDev := range pci {
		if m.findDeviceWithID(pciDev.Devices, id) {
			return true, nil
		}
	}
	return false, nil
}

func (m *monitor) findDeviceWithID(devs []qmpraw.PCIDeviceInfo, id string) bool {
	for _, dev := range devs {
		if dev.QdevID == id {
			return true
		}

		if dev.PCIBridge != nil {
			if m.findDeviceWithID(dev.PCIBridge.Devices, id) {
				return true
			}
		}
	}

	return false
}
