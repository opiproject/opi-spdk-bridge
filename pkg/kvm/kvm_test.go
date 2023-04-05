// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"
)

var (
	errStub                 = errors.New("stub error")
	alwaysSuccessfulJSONRPC = stubJSONRRPC{nil}
	alwaysFailingJSONRPC    = stubJSONRRPC{errStub}

	testVirtioBlkID            = "virtio-blk-42"
	testCreateVirtioBlkRequest = &pb.CreateVirtioBlkRequest{VirtioBlk: &pb.VirtioBlk{
		Id:       &pc.ObjectKey{Value: testVirtioBlkID},
		PcieId:   &pb.PciEndpoint{PhysicalFunction: 42},
		VolumeId: &pc.ObjectKey{Value: "Malloc42"},
		MaxIoQps: 1,
	}}
	testDeleteVirtioBlkRequest = &pb.DeleteVirtioBlkRequest{Name: testVirtioBlkID}

	genericQmpError = `{"error": {"class": "GenericError", "desc": "some error"}}` + "\n"
	genericQmpOk    = `{"return": {}}` + "\n"

	qmpServerOperationTimeout = 500 * time.Millisecond
	qmplibTimeout             = 250 * time.Millisecond
)

type stubJSONRRPC struct {
	err error
}

func (s stubJSONRRPC) Call(method string, _, result interface{}) error {
	if method == "vhost_create_blk_controller" {
		if s.err == nil {
			resultCreateVirtioBLk, ok := result.(*models.VhostCreateBlkControllerResult)
			if !ok {
				log.Panicf("Unexpected type for virtio-blk device creation result")
			}
			*resultCreateVirtioBLk = models.VhostCreateBlkControllerResult(true)
		}
		return s.err
	} else if method == "vhost_delete_controller" {
		if s.err == nil {
			resultDeleteVirtioBLk, ok := result.(*models.VhostDeleteControllerResult)
			if !ok {
				log.Panicf("Unexpected type for virtio-blk device deletion result")
			}
			*resultDeleteVirtioBLk = models.VhostDeleteControllerResult(true)
		}
		return s.err
	} else if method == "nvmf_subsystem_add_listener" || method == "nvmf_subsystem_remove_listener" {
		if s.err == nil {
			resultCreateNvmeController, ok := result.(*models.NvmfSubsystemAddListenerResult)
			if !ok {
				log.Panicf("Unexpected type for add subsystem listener result")
			}
			*resultCreateNvmeController = models.NvmfSubsystemAddListenerResult(true)
		}
		return s.err
	} else {
		return s.err
	}
}

type mockCall struct {
	response     string
	event        string
	expectedArgs []string
}

type mockQmpServer struct {
	socket     net.Listener
	testDir    string
	socketPath string

	greeting                string
	capabilitiesNegotiation mockCall
	expectedCalls           []mockCall
	callIndex               uint32

	test *testing.T
	mu   sync.Mutex
}

func startMockQmpServer(t *testing.T) *mockQmpServer {
	s := &mockQmpServer{}
	s.greeting =
		`{"QMP":{"version":{"qemu":{"micro":50,"minor":0,"major":7},"package":""},"capabilities":[]}}`
	s.capabilitiesNegotiation = mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"qmp_capabilities"`,
		},
	}

	testDir, err := os.MkdirTemp("", "opi-spdk-kvm-test")
	if err != nil {
		log.Panic(err.Error())
	}
	s.testDir = testDir

	s.socketPath = filepath.Join(s.testDir, "qmp.sock")
	socket, err := net.Listen("unix", s.socketPath)
	if err != nil {
		log.Panic(err.Error())
	}
	s.socket = socket
	s.test = t

	go func() {
		conn, err := s.socket.Accept()
		if err != nil {
			return
		}
		err = conn.SetDeadline(time.Now().Add(qmpServerOperationTimeout))
		if err != nil {
			log.Panicf("Failed to set deadline: %v", err)
		}

		s.write(s.greeting, conn)
		s.handleCall(s.capabilitiesNegotiation, conn)
		for _, call := range s.expectedCalls {
			s.handleExpectedCall(call, conn)
		}
	}()

	return s
}

func (s *mockQmpServer) Stop() {
	if s.socket != nil {
		if err := s.socket.Close(); err != nil {
			log.Panicf("Failed to close socket: %v", err)
		}
	}
	if err := os.RemoveAll(s.testDir); err != nil {
		log.Panicf("Failed to delete test dir: %v", err)
	}
}

func (s *mockQmpServer) ExpectAddChardev(id string) *mockQmpServer {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: `{"return": {"pty": "/tmp/dev/pty/42"}}` + "\n",
		expectedArgs: []string{
			`"execute":"chardev-add"`,
			`"id":"` + id + `"`,
			`"path":"` + filepath.Join(s.testDir, id) + `"`,
		},
	})
	return s
}

func (s *mockQmpServer) ExpectAddVirtioBlk(id string, chardevID string) *mockQmpServer {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"device_add"`,
			`"driver":"vhost-user-blk-pci"`,
			`"id":"` + id + `"`,
			`"chardev":"` + chardevID + `"`,
		},
	})
	return s
}

func (s *mockQmpServer) ExpectAddNvmeController(id, controllersDir string) *mockQmpServer {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"device_add"`,
			`"driver":"vfio-user-pci"`,
			`"id":"` + id + `"`,
			`"socket":"` + filepath.Join(controllersDir, id, "cntrl") + `"`,
		},
	})
	return s
}

func (s *mockQmpServer) ExpectDeleteChardev(id string) *mockQmpServer {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"chardev-remove"`,
			`"id":"` + id + `"`,
		},
	})
	return s
}

func (s *mockQmpServer) ExpectDeleteVirtioBlkWithEvent(id string) *mockQmpServer {
	s.ExpectDeleteVirtioBlk(id)
	s.expectedCalls[len(s.expectedCalls)-1].event =
		`{"event":"DEVICE_DELETED","data":{"path":"/some/path","device":"` +
			id + `"},"timestamp":{"seconds":1,"microseconds":2}}` + "\n"
	return s
}

func (s *mockQmpServer) ExpectDeleteVirtioBlk(id string) *mockQmpServer {
	return s.expectDeleteDevice(id)
}

func (s *mockQmpServer) ExpectDeleteNvmeController(id string) *mockQmpServer {
	return s.expectDeleteDevice(id)
}

func (s *mockQmpServer) ExpectQueryPci(id string) *mockQmpServer {
	response := `{"return":[{"bus":0,"devices":[]}]}` + "\n"
	if id != "" {
		response = `{"return":[{"bus":0,"devices":[{"bus":0,"slot":0,"function":0,"class_info":{"class":0},"id":{"device":0,"vendor":0},"qdev_id":"` +
			id + `","regions":[]}]}]}` + "\n"
	}
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: response,
		expectedArgs: []string{
			`"execute":"query-pci"`,
		},
	})
	return s
}

func (s *mockQmpServer) WithErrorResponse() *mockQmpServer {
	if len(s.expectedCalls) == 0 {
		log.Panicf("No instance to add a QMP error")
	}
	s.expectedCalls[len(s.expectedCalls)-1].response = genericQmpError
	return s
}

func (s *mockQmpServer) WereExpectedCallsPerformed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	numberOfPerformedCalls := s.callIndex
	numberOfExpectedCalls := len(s.expectedCalls)
	ok := int(numberOfPerformedCalls) == numberOfExpectedCalls
	if !ok {
		log.Printf("Not all expected calls are performed. Expected calls %v: %v. Index: %v",
			numberOfPerformedCalls, s.expectedCalls, numberOfPerformedCalls)
	}
	return ok
}

func (s *mockQmpServer) handleExpectedCall(call mockCall, conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handleCall(call, conn)
	s.callIndex++
}

func (s *mockQmpServer) handleCall(call mockCall, conn net.Conn) {
	req := s.read(conn)
	for _, expectedArg := range call.expectedArgs {
		if !strings.Contains(req, expectedArg) {
			s.test.Errorf("Expected to find argument %v in %v", expectedArg, req)
		}
	}
	s.write(call.response, conn)
	if call.event != "" {
		time.Sleep(time.Millisecond * 1)
		s.write(call.event, conn)
	}
}

func (s *mockQmpServer) write(data string, conn net.Conn) {
	log.Println("QMP server send:", data)
	_, err := conn.Write([]byte(data))
	if err != nil {
		log.Panicf("QMP server failed to write: %v", data)
	}
}

func (s *mockQmpServer) read(conn net.Conn) string {
	buf := make([]byte, 512)
	_, err := conn.Read(buf)
	if err != nil {
		log.Panicf("QMP server failed to read")
	}
	data := string(buf)
	log.Println("QMP server got :", data)
	return data
}

func (s *mockQmpServer) expectDeleteDevice(id string) *mockQmpServer {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"device_del"`,
			`"id":"` + id + `"`,
		},
	})
	return s
}
