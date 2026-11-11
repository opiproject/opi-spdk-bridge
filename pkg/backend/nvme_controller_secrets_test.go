// SPDX-License-Identifier: Apache-2.0
package backend

import (
	"bytes"
	"testing"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

func TestRemoteControllerRPCSecretRedaction(t *testing.T) {
	env := createTestEnvironment(nil)
	defer env.Close()
	controller := utils.ProtoClone(&testNvmeCtrl)
	controller.Tcp.Psk = []byte("test-only-psk")
	request := &pb.CreateNvmeRemoteControllerRequest{
		NvmeRemoteController: controller, NvmeRemoteControllerId: testNvmeCtrlID,
	}
	check := func(response *pb.NvmeRemoteController, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if len(response.GetTcp().GetPsk()) != 0 {
			t.Fatal("controller response exposed a PSK")
		}
		if !bytes.Equal(env.opiSpdkServer.Volumes.NvmeControllers[testNvmeCtrlName].Tcp.Psk, controller.Tcp.Psk) {
			t.Fatal("response redaction must preserve the PSK used for path setup")
		}
	}
	check(env.client.CreateNvmeRemoteController(env.ctx, request))
	check(env.client.CreateNvmeRemoteController(env.ctx, request))
	check(env.client.GetNvmeRemoteController(env.ctx, &pb.GetNvmeRemoteControllerRequest{Name: testNvmeCtrlName}))
	list, err := env.client.ListNvmeRemoteControllers(env.ctx, &pb.ListNvmeRemoteControllersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.NvmeRemoteControllers) != 1 {
		t.Fatal("expected one controller")
	}
	check(list.NvmeRemoteControllers[0], nil)
	update := utils.ProtoClone(controller)
	update.Name = testNvmeCtrlName
	check(env.client.UpdateNvmeRemoteController(env.ctx, &pb.UpdateNvmeRemoteControllerRequest{NvmeRemoteController: update}))
}
