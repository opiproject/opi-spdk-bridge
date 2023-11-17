// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package kvm automates plugging of SPDK devices to a QEMU instance
package kvm

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opiproject/gospdk/spdk"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const qmpID = `"id":"`

var (
	errStub                 = status.Error(codes.Internal, "stub error")
	alwaysSuccessfulJSONRPC = &stubJSONRRPC{err: nil}
	alwaysFailingJSONRPC    = &stubJSONRRPC{err: errStub}

	genericQmpError = `{"error": {"class": "GenericError", "desc": "some error"}}` + "\n"
	genericQmpOk    = `{"return": {}}` + "\n"

	qmpServerOperationTimeout = 500 * time.Millisecond
	qmplibTimeout             = 250 * time.Millisecond

	pathRegexpStr = `\/[a-zA-Z\/\-\_0-9]*\/`

	checkGlobalTestProtoObjectsNotChanged = utils.CheckTestProtoObjectsNotChanged(
		testCreateNvmeControllerRequest,
		testDeleteNvmeControllerRequest,
		&testSubsystem,
		testCreateVirtioBlkRequest,
		testDeleteVirtioBlkRequest,
	)
)

type stubJSONRRPC struct {
	err error
	arg any
}

// build time check that struct implements interface
var _ spdk.JSONRPC = (*stubJSONRRPC)(nil)

func (s *stubJSONRRPC) GetID() uint64 {
	return 0
}

func (s *stubJSONRRPC) StartUnixListener() net.Listener {
	return nil
}

func (s *stubJSONRRPC) GetVersion(_ context.Context) string {
	return ""
}

func (s *stubJSONRRPC) Call(_ context.Context, method string, arg, result interface{}) error {
	if method == "vhost_create_blk_controller" {
		if s.err == nil {
			resultCreateVirtioBLk, ok := result.(*spdk.VhostCreateBlkControllerResult)
			if !ok {
				log.Panicf("Unexpected type for virtio-blk device creation result")
			}
			*resultCreateVirtioBLk = spdk.VhostCreateBlkControllerResult(true)
		}
	} else if method == "vhost_delete_controller" {
		if s.err == nil {
			resultDeleteVirtioBLk, ok := result.(*spdk.VhostDeleteControllerResult)
			if !ok {
				log.Panicf("Unexpected type for virtio-blk device deletion result")
			}
			*resultDeleteVirtioBLk = spdk.VhostDeleteControllerResult(true)
		}
	} else if method == "nvmf_subsystem_add_listener" || method == "nvmf_subsystem_remove_listener" {
		if s.err == nil {
			resultCreateNvmeController, ok := result.(*spdk.NvmfSubsystemAddListenerResult)
			if !ok {
				log.Panicf("Unexpected type for add subsystem listener result")
			}
			*resultCreateNvmeController = spdk.NvmfSubsystemAddListenerResult(true)
		}
	}
	s.arg = arg

	return s.err
}

type mockCall struct {
	response           string
	event              string
	expectedArgs       []string
	expectedRegExpArgs []*regexp.Regexp
}

type mockQmpCalls struct {
	expectedCalls []mockCall
}

func newMockQmpCalls() *mockQmpCalls {
	return &mockQmpCalls{}
}

func (s *mockQmpCalls) ExpectAddChardev(id string) *mockQmpCalls {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: `{"return": {"pty": "/tmp/dev/pty/42"}}` + "\n",
		expectedArgs: []string{
			`"execute":"chardev-add"`,
			qmpID + toQemuID(id) + `"`,
		},
		expectedRegExpArgs: []*regexp.Regexp{
			regexp.MustCompile(`"path":"` + pathRegexpStr + id + `"`),
		},
	})
	return s
}

func (s *mockQmpCalls) ExpectAddVirtioBlk(id string, chardevID string) *mockQmpCalls {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"device_add"`,
			`"driver":"vhost-user-blk-pci"`,
			qmpID + toQemuID(id) + `"`,
			`"chardev":"` + toQemuID(chardevID) + `"`,
		},
	})
	return s
}

func (s *mockQmpCalls) ExpectAddVirtioBlkWithAddress(id string, chardevID string, bus string, pf uint32) *mockQmpCalls {
	s.ExpectAddVirtioBlk(id, chardevID)
	index := len(s.expectedCalls) - 1
	s.expectedCalls[index].expectedArgs =
		append(s.expectedCalls[index].expectedArgs, `"bus":"`+bus+`"`)
	s.expectedCalls[index].expectedArgs =
		append(s.expectedCalls[index].expectedArgs, `"addr":"`+fmt.Sprintf("%#x", pf)+`"`)
	return s
}

func (s *mockQmpCalls) ExpectAddNvmeController(id string, ctrlrDir string) *mockQmpCalls {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"device_add"`,
			`"driver":"vfio-user-pci"`,
			qmpID + toQemuID(id) + `"`,
		},
		expectedRegExpArgs: []*regexp.Regexp{
			regexp.MustCompile(`"socket":"` + pathRegexpStr + ctrlrDir + `/cntrl"`),
		},
	})
	return s
}

func (s *mockQmpCalls) ExpectAddNvmeControllerWithAddress(id string, ctrlDir string, bus string, pf uint32) *mockQmpCalls {
	s.ExpectAddNvmeController(id, ctrlDir)
	index := len(s.expectedCalls) - 1
	s.expectedCalls[index].expectedArgs =
		append(s.expectedCalls[index].expectedArgs, `"bus":"`+bus+`"`)
	s.expectedCalls[index].expectedArgs =
		append(s.expectedCalls[index].expectedArgs, `"addr":"`+fmt.Sprintf("%#x", pf)+`"`)
	return s
}

func (s *mockQmpCalls) ExpectDeleteChardev(id string) *mockQmpCalls {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"chardev-remove"`,
			qmpID + toQemuID(id) + `"`,
		},
	})
	return s
}

func (s *mockQmpCalls) ExpectDeleteVirtioBlkWithEvent(id string) *mockQmpCalls {
	s.ExpectDeleteVirtioBlk(id)
	s.expectedCalls[len(s.expectedCalls)-1].event =
		`{"event":"DEVICE_DELETED","data":{"path":"/some/path","device":"` +
			toQemuID(id) + `"},"timestamp":{"seconds":1,"microseconds":2}}` + "\n"
	return s
}

func (s *mockQmpCalls) ExpectDeleteVirtioBlk(id string) *mockQmpCalls {
	return s.expectDeleteDevice(id)
}

func (s *mockQmpCalls) ExpectDeleteNvmeController(id string) *mockQmpCalls {
	return s.expectDeleteDevice(id)
}

func (s *mockQmpCalls) ExpectQueryPci(id string) *mockQmpCalls {
	response := `{"return":[{"bus":0,"devices":[{"bus":0,"slot":0,"function":0,` +
		`"class_info":{"class":0},"id":{"device":0,"vendor":0},"qdev_id":"` +
		toQemuID(id) + `","regions":[]}]}]}` + "\n"
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: response,
		expectedArgs: []string{
			`"execute":"query-pci"`,
		},
	})
	return s
}

func (s *mockQmpCalls) ExpectNoDeviceQueryPci() *mockQmpCalls {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: `{"return":[{"bus":0,"devices":[]}]}` + "\n",
		expectedArgs: []string{
			`"execute":"query-pci"`,
		},
	})
	return s
}

func (s *mockQmpCalls) WithErrorResponse() *mockQmpCalls {
	if len(s.expectedCalls) == 0 {
		log.Panicf("No instance to add a QMP error")
	}
	s.expectedCalls[len(s.expectedCalls)-1].response = genericQmpError
	return s
}

func (s *mockQmpCalls) expectDeleteDevice(id string) *mockQmpCalls {
	s.expectedCalls = append(s.expectedCalls, mockCall{
		response: genericQmpOk,
		expectedArgs: []string{
			`"execute":"device_del"`,
			qmpID + toQemuID(id) + `"`,
		},
	})
	return s
}

func (s *mockQmpCalls) GetExpectedCalls() []mockCall {
	return s.expectedCalls
}

type mockQmpServer struct {
	socket     net.Listener
	testDir    string
	socketPath string

	greeting                string
	capabilitiesNegotiation mockCall

	expectedCalls []mockCall
	test          *testing.T
	mu            sync.Mutex
	callIndex     uint32
}

func startMockQmpServer(t *testing.T, m *mockQmpCalls) *mockQmpServer {
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
	if m != nil {
		s.expectedCalls = m.GetExpectedCalls()
	}

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
	for _, expectedRegExpArg := range call.expectedRegExpArgs {
		if !expectedRegExpArg.MatchString(req) {
			s.test.Errorf("Expected to find argument matching regexp %v in %v", expectedRegExpArg.String(), req)
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
