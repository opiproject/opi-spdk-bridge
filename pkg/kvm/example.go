// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package kvm implements QMP communication with KVM instance
package kvm

import (
	"log"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
)

// Communicate implements example QMP with KVM
func Communicate() {
	log.Printf("Start")
	mon, _ := qmp.NewSocketMonitor("tcp", "localhost:4444", 2*time.Second)
	// Monitor must be connected before it can be used.
	if err := mon.Connect(); err != nil {
		log.Fatalf("failed to connect monitor: %v", err)
	}
	defer func(mon *qmp.SocketMonitor) {
		err := mon.Disconnect()
		if err != nil {
			log.Fatal(err)
		}
	}(mon)
	commands := []string{`{ "execute": "qmp_capabilities" }`, `{ "execute": "query-commands" }`, `{ "execute": "query-pci" }`}
	for _, s := range commands {
		cmd := []byte(s)
		log.Printf("snd %v", string(cmd))
		raw, _ := mon.Run(cmd)
		log.Printf("got %v", string(raw))
	}
}
