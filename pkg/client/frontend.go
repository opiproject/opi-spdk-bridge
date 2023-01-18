// Package client implements the storage client
package client

import (
	"context"
	"log"
	"time"

	pbc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
)

// DoFrontend executes the front end code
func DoFrontend(ctx context.Context, conn grpc.ClientConnInterface) {
	err := executeNVMeSubsystem(ctx, conn)
	if err != nil {
		log.Fatalf("Error executeNVMeSubsystem: %v", err)
		return
	}

	err = executeNVMeController(ctx, conn)
	if err != nil {
		log.Fatalf("Error executeNVMeController: %v", err)
		return
	}

	err = executeNVMeNamespace(ctx, conn)
	if err != nil {
		log.Fatalf("Error executeNVMeNamespace: %v", err)
		return
	}

	err = executeVirtioBlk(ctx, conn)
	if err != nil {
		log.Fatalf("Error executeVirtioBlk: %v", err)
		return
	}

	c5, err := executeVirtioScsiController(ctx, conn)
	if err != nil {
		log.Fatalf("Error executeVirtioScsiController: %v", err)
		return
	}

	err = executeVirtioScsiLun(ctx, conn, c5)
	if err != nil {
		log.Fatalf("Error executeVirtioScsiLun: %v", err)
		return
	}
}

func executeVirtioScsiLun(ctx context.Context, conn grpc.ClientConnInterface, c5 pb.FrontendVirtioScsiServiceClient) error {
	// VirtioScsiLun
	c6 := pb.NewFrontendVirtioScsiServiceClient(conn)
	log.Printf("Testing NewFrontendVirtioScsiServiceClient")
	rl1, err := c6.CreateVirtioScsiLun(ctx, &pb.CreateVirtioScsiLunRequest{VirtioScsiLun: &pb.VirtioScsiLun{TargetId: &pbc.ObjectKey{Value: "OPI-VirtioScsi8"}, VolumeId: &pbc.ObjectKey{Value: "Malloc1"}}})
	if err != nil {
		log.Fatalf("could not create VirtioScsi subsystem: %v", err)
	}
	log.Printf("Added: %v", rl1)
	rl3, err := c6.UpdateVirtioScsiLun(ctx, &pb.UpdateVirtioScsiLunRequest{VirtioScsiLun: &pb.VirtioScsiLun{TargetId: &pbc.ObjectKey{Value: "OPI-VirtioScsi8"}, VolumeId: &pbc.ObjectKey{Value: "Malloc1"}}})
	if err != nil {
		log.Fatalf("could not update VirtioScsi subsystem: %v", err)
	}
	log.Printf("Updated: %v", rl3)
	rl4, err := c6.ListVirtioScsiLuns(ctx, &pb.ListVirtioScsiLunsRequest{Parent: "OPI-VirtioScsi8"})
	if err != nil {
		log.Fatalf("could not list VirtioScsi subsystem: %v", err)
	}
	log.Printf("Listed: %v", rl4)
	rl5, err := c6.GetVirtioScsiLun(ctx, &pb.GetVirtioScsiLunRequest{Name: "OPI-VirtioScsi8"})
	if err != nil {
		log.Fatalf("could not get VirtioScsi subsystem: %v", err)
	}
	log.Printf("Got: %v", rl5.VolumeId.Value)
	rl6, err := c6.VirtioScsiLunStats(ctx, &pb.VirtioScsiLunStatsRequest{ControllerId: &pbc.ObjectKey{Value: "OPI-VirtioScsi8"}})
	if err != nil {
		log.Fatalf("could not stats VirtioScsi subsystem: %v", err)
	}
	log.Printf("Stats: %v", rl6.Stats)
	rl2, err := c6.DeleteVirtioScsiLun(ctx, &pb.DeleteVirtioScsiLunRequest{Name: "OPI-VirtioScsi8"})
	if err != nil {
		log.Fatalf("could not delete VirtioScsi subsystem: %v", err)
	}
	log.Printf("Deleted: %v", rl2)

	rss2, err := c5.DeleteVirtioScsiController(ctx, &pb.DeleteVirtioScsiControllerRequest{Name: "OPI-VirtioScsi8"})
	if err != nil {
		log.Fatalf("could not delete VirtioScsi subsystem: %v", err)
	}
	log.Printf("Deleted: %v", rss2)
	return err
}

func executeVirtioScsiController(ctx context.Context, conn grpc.ClientConnInterface) (pb.FrontendVirtioScsiServiceClient, error) {
	// VirtioScsiController
	c5 := pb.NewFrontendVirtioScsiServiceClient(conn)
	log.Printf("Testing NewFrontendVirtioScsiServiceClient")
	rss1, err := c5.CreateVirtioScsiController(ctx, &pb.CreateVirtioScsiControllerRequest{VirtioScsiController: &pb.VirtioScsiController{Id: &pbc.ObjectKey{Value: "OPI-VirtioScsi8"}}})
	if err != nil {
		log.Fatalf("could not create VirtioScsi subsystem: %v", err)
	}
	log.Printf("Added: %v", rss1)
	rss3, err := c5.UpdateVirtioScsiController(ctx, &pb.UpdateVirtioScsiControllerRequest{VirtioScsiController: &pb.VirtioScsiController{Id: &pbc.ObjectKey{Value: "OPI-VirtioScsi8"}}})
	if err != nil {
		log.Fatalf("could not update VirtioScsi subsystem: %v", err)
	}
	log.Printf("Updated: %v", rss3)
	rss4, err := c5.ListVirtioScsiControllers(ctx, &pb.ListVirtioScsiControllersRequest{})
	if err != nil {
		log.Fatalf("could not list VirtioScsi subsystem: %v", err)
	}
	log.Printf("Listed: %s", rss4)
	rss5, err := c5.GetVirtioScsiController(ctx, &pb.GetVirtioScsiControllerRequest{Name: "OPI-VirtioScsi8"})
	if err != nil {
		log.Fatalf("could not get VirtioScsi subsystem: %v", err)
	}
	log.Printf("Got: %s", rss5.Id.Value)
	rss6, err := c5.VirtioScsiControllerStats(ctx, &pb.VirtioScsiControllerStatsRequest{ControllerId: &pbc.ObjectKey{Value: "OPI-VirtioScsi8"}})
	if err != nil {
		log.Fatalf("could not stats VirtioScsi subsystem: %v", err)
	}
	log.Printf("Stats: %s", rss6.Stats)
	return c5, err
}

func executeVirtioBlk(ctx context.Context, conn grpc.ClientConnInterface) error {
	// VirtioBlk
	c4 := pb.NewFrontendVirtioBlkServiceClient(conn)
	log.Printf("Testing NewFrontendVirtioBlkServiceClient")
	rv1, err := c4.CreateVirtioBlk(ctx, &pb.CreateVirtioBlkRequest{VirtioBlk: &pb.VirtioBlk{Id: &pbc.ObjectKey{Value: "VirtioBlk8"}, VolumeId: &pbc.ObjectKey{Value: "Malloc1"}}})
	if err != nil {
		log.Fatalf("could not create VirtioBlk Controller: %v", err)
	}
	log.Printf("Added: %v", rv1)
	rv3, err := c4.UpdateVirtioBlk(ctx, &pb.UpdateVirtioBlkRequest{VirtioBlk: &pb.VirtioBlk{Id: &pbc.ObjectKey{Value: "VirtioBlk8"}}})
	if err != nil {
		log.Fatalf("could not update VirtioBlk Controller: %v", err)
	}
	log.Printf("Updated: %v", rv3)
	rv4, err := c4.ListVirtioBlks(ctx, &pb.ListVirtioBlksRequest{})
	if err != nil {
		log.Fatalf("could not list VirtioBlk Controller: %v", err)
	}
	log.Printf("Listed: %v", rv4)
	rv5, err := c4.GetVirtioBlk(ctx, &pb.GetVirtioBlkRequest{Name: "VirtioBlk8"})
	if err != nil {
		log.Fatalf("could not get VirtioBlk Controller: %v", err)
	}
	log.Printf("Got: %v", rv5.Id.Value)
	rv6, err := c4.VirtioBlkStats(ctx, &pb.VirtioBlkStatsRequest{ControllerId: &pbc.ObjectKey{Value: "VirtioBlk8"}})
	if err != nil {
		log.Fatalf("could not stats VirtioBlk Controller: %v", err)
	}
	log.Printf("Stats: %v", rv6.Stats)
	rv2, err := c4.DeleteVirtioBlk(ctx, &pb.DeleteVirtioBlkRequest{Name: "VirtioBlk8"})
	if err != nil {
		log.Fatalf("could not delete VirtioBlk Controller: %v", err)
	}
	log.Printf("Deleted: %v", rv2)

	return err
}

func executeNVMeNamespace(ctx context.Context, conn grpc.ClientConnInterface) error {
	// pre create: subsystem and controller
	c1 := pb.NewFrontendNvmeServiceClient(conn)
	rs1, err := c1.CreateNVMeSubsystem(ctx, &pb.CreateNVMeSubsystemRequest{
		NvMeSubsystem: &pb.NVMeSubsystem{
			Spec: &pb.NVMeSubsystemSpec{
				Id:  &pbc.ObjectKey{Value: "namespace-test-ss"},
				Nqn: "nqn.2022-09.io.spdk:opi1"}}})
	if err != nil {
		log.Fatalf("could not create NVMe subsystem: %v", err)
	}
	log.Printf("Added subsystem: %v", rs1)
	c2 := pb.NewFrontendNvmeServiceClient(conn)
	rc1, err := c2.CreateNVMeController(ctx, &pb.CreateNVMeControllerRequest{
		NvMeController: &pb.NVMeController{
			Spec: &pb.NVMeControllerSpec{
				Id:               &pbc.ObjectKey{Value: "namespace-test-ctrler"},
				SubsystemId:      &pbc.ObjectKey{Value: "namespace-test-ss"},
				NvmeControllerId: 1}}})
	if err != nil {
		log.Fatalf("could not create NVMe controller: %v", err)
	}
	log.Printf("Added controller: %v", rc1)

	// wait for some time for the backend to created above objects
	time.Sleep(time.Second)

	// NVMeNamespace
	c3 := pb.NewFrontendNvmeServiceClient(conn)
	log.Printf("Testing NewFrontendNvmeServiceClient")
	rn1, err := c3.CreateNVMeNamespace(ctx, &pb.CreateNVMeNamespaceRequest{
		NvMeNamespace: &pb.NVMeNamespace{
			Spec: &pb.NVMeNamespaceSpec{
				Id:          &pbc.ObjectKey{Value: "namespace-test"},
				SubsystemId: &pbc.ObjectKey{Value: "namespace-test-ss"},
				VolumeId:    &pbc.ObjectKey{Value: "Malloc1"},
				HostNsid:    1}}})
	if err != nil {
		log.Fatalf("could not create NVMe namespace: %v", err)
	}
	log.Printf("Added: %v", rn1)
	rn3, err := c3.UpdateNVMeNamespace(ctx, &pb.UpdateNVMeNamespaceRequest{
		NvMeNamespace: &pb.NVMeNamespace{
			Spec: &pb.NVMeNamespaceSpec{
				Id:          &pbc.ObjectKey{Value: "namespace-test"},
				SubsystemId: &pbc.ObjectKey{Value: "namespace-test-ss"},
				HostNsid:    1}}})
	if err != nil {
		log.Fatalf("could not update NVMe namespace: %v", err)
	}
	log.Printf("Updated: %v", rn3)
	rn4, err := c3.ListNVMeNamespaces(ctx, &pb.ListNVMeNamespacesRequest{Parent: "namespace-test-ss"})
	if err != nil {
		log.Fatalf("could not list NVMe namespace: %v", err)
	}
	log.Printf("Listed: %v", rn4)
	rn5, err := c3.GetNVMeNamespace(ctx, &pb.GetNVMeNamespaceRequest{Name: "namespace-test"})
	if err != nil {
		log.Fatalf("could not get NVMe namespace: %v", err)
	}
	log.Printf("Got: %v", rn5.Spec.Id.Value)
	rn6, err := c3.NVMeNamespaceStats(ctx, &pb.NVMeNamespaceStatsRequest{NamespaceId: &pbc.ObjectKey{Value: "namespace-test"}})
	if err != nil {
		log.Fatalf("could not stats NVMe namespace: %v", err)
	}
	log.Printf("Stats: %v", rn6.Stats)
	rn2, err := c3.DeleteNVMeNamespace(ctx, &pb.DeleteNVMeNamespaceRequest{Name: "namespace-test"})
	if err != nil {
		log.Fatalf("could not delete NVMe namespace: %v", err)
	}
	log.Printf("Deleted: %v", rn2)

	// post cleanup: controller and subsystem
	rc2, err := c2.DeleteNVMeController(ctx, &pb.DeleteNVMeControllerRequest{Name: "namespace-test-ctrler"})
	if err != nil {
		log.Fatalf("could not delete NVMe controller: %v", err)
	}
	log.Printf("Deleted: %v", rc2)

	rs2, err := c1.DeleteNVMeSubsystem(ctx, &pb.DeleteNVMeSubsystemRequest{Name: "namespace-test-ss"})
	if err != nil {
		log.Fatalf("could not delete NVMe subsystem: %v", err)
	}
	log.Printf("Deleted: %v", rs2)
	return err
}

func executeNVMeController(ctx context.Context, conn grpc.ClientConnInterface) error {
	// pre create: subsystem
	c1 := pb.NewFrontendNvmeServiceClient(conn)
	rs1, err := c1.CreateNVMeSubsystem(ctx, &pb.CreateNVMeSubsystemRequest{
		NvMeSubsystem: &pb.NVMeSubsystem{
			Spec: &pb.NVMeSubsystemSpec{
				Id:  &pbc.ObjectKey{Value: "controller-test-ss"},
				Nqn: "nqn.2022-09.io.spdk:opi2"}}})
	if err != nil {
		log.Fatalf("could not create NVMe subsystem: %v", err)
	}
	log.Printf("Added subsystem: %v", rs1)

	// NVMeController
	c2 := pb.NewFrontendNvmeServiceClient(conn)
	log.Printf("Testing NewFrontendNvmeServiceClient")
	rc1, err := c2.CreateNVMeController(ctx, &pb.CreateNVMeControllerRequest{
		NvMeController: &pb.NVMeController{
			Spec: &pb.NVMeControllerSpec{
				Id:               &pbc.ObjectKey{Value: "controller-test"},
				SubsystemId:      &pbc.ObjectKey{Value: "controller-test-ss"},
				NvmeControllerId: 1}}})
	if err != nil {
		log.Fatalf("could not create NVMe controller: %v", err)
	}
	log.Printf("Added: %v", rc1)

	rc3, err := c2.UpdateNVMeController(ctx, &pb.UpdateNVMeControllerRequest{
		NvMeController: &pb.NVMeController{
			Spec: &pb.NVMeControllerSpec{
				Id:               &pbc.ObjectKey{Value: "controller-test"},
				SubsystemId:      &pbc.ObjectKey{Value: "controller-test-ss"},
				NvmeControllerId: 2}}})
	if err != nil {
		log.Fatalf("could not update NVMe controller: %v", err)
	}
	log.Printf("Updated: %v", rc3)

	rc4, err := c2.ListNVMeControllers(ctx, &pb.ListNVMeControllersRequest{Parent: "controller-test-ss"})
	if err != nil {
		log.Fatalf("could not list NVMe controller: %v", err)
	}
	log.Printf("Listed: %s", rc4)

	rc5, err := c2.GetNVMeController(ctx, &pb.GetNVMeControllerRequest{Name: "controller-test"})
	if err != nil {
		log.Fatalf("could not get NVMe controller: %v", err)
	}
	log.Printf("Got: %s", rc5.Spec.Id.Value)

	rc6, err := c2.NVMeControllerStats(ctx, &pb.NVMeControllerStatsRequest{Id: &pbc.ObjectKey{Value: "controller-test"}})
	if err != nil {
		log.Fatalf("could not stats NVMe controller: %v", err)
	}
	log.Printf("Stats: %s", rc6.Stats)

	rc2, err := c2.DeleteNVMeController(ctx, &pb.DeleteNVMeControllerRequest{Name: "controller-test"})
	if err != nil {
		log.Fatalf("could not delete NVMe controller: %v", err)
	}
	log.Printf("Deleted: %v", rc2)

	// post cleanup: subsystem
	rs2, err := c1.DeleteNVMeSubsystem(ctx, &pb.DeleteNVMeSubsystemRequest{Name: "controller-test-ss"})
	if err != nil {
		log.Fatalf("could not delete NVMe subsystem: %v", err)
	}
	log.Printf("Deleted: %v", rs2)
	return err
}

func executeNVMeSubsystem(ctx context.Context, conn grpc.ClientConnInterface) error {
	// NVMeSubsystem
	c1 := pb.NewFrontendNvmeServiceClient(conn)
	log.Printf("Testing NewFrontendNvmeServiceClient")
	rs1, err := c1.CreateNVMeSubsystem(ctx, &pb.CreateNVMeSubsystemRequest{
		NvMeSubsystem: &pb.NVMeSubsystem{
			Spec: &pb.NVMeSubsystemSpec{
				Id:  &pbc.ObjectKey{Value: "subsystem-test"},
				Nqn: "nqn.2022-09.io.spdk:opi3"}}})
	if err != nil {
		log.Fatalf("could not create NVMe subsystem: %v", err)
	}
	log.Printf("Added: %v", rs1)
	rs3, err := c1.UpdateNVMeSubsystem(ctx, &pb.UpdateNVMeSubsystemRequest{
		NvMeSubsystem: &pb.NVMeSubsystem{
			Spec: &pb.NVMeSubsystemSpec{
				Id:  &pbc.ObjectKey{Value: "subsystem-test"},
				Nqn: "nqn.2022-09.io.spdk:opi3"}}})
	if err != nil {
		log.Fatalf("could not update NVMe subsystem: %v", err)
	}
	log.Printf("Updated: %v", rs3)
	rs4, err := c1.ListNVMeSubsystems(ctx, &pb.ListNVMeSubsystemsRequest{})
	if err != nil {
		log.Fatalf("could not list NVMe subsystem: %v", err)
	}
	log.Printf("Listed: %v", rs4)
	rs5, err := c1.GetNVMeSubsystem(ctx, &pb.GetNVMeSubsystemRequest{Name: "subsystem-test"})
	if err != nil {
		log.Fatalf("could not get NVMe subsystem: %v", err)
	}
	log.Printf("Got: %s", rs5.Spec.Nqn)
	rs6, err := c1.NVMeSubsystemStats(ctx, &pb.NVMeSubsystemStatsRequest{
		SubsystemId: &pbc.ObjectKey{Value: "subsystem-test"}})
	if err != nil {
		log.Fatalf("could not stats NVMe subsystem: %v", err)
	}
	log.Printf("Stats: %s", rs6.Stats)
	rs2, err := c1.DeleteNVMeSubsystem(ctx, &pb.DeleteNVMeSubsystemRequest{Name: "subsystem-test"})
	if err != nil {
		log.Fatalf("could not delete NVMe subsystem: %v", err)
	}
	log.Printf("Deleted: %v", rs2)
	return err
}
