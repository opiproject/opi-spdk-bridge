// The main package of the storage client
package main

import (
	"context"
	"log"
	"net"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
)

func doBackend(ctx context.Context, conn grpc.ClientConnInterface) {
	// NVMfRemoteController
	c4 := pb.NewNVMfRemoteControllerServiceClient(conn)
	addr, err := net.LookupIP("spdk")
	if err != nil {
		log.Fatalf("could not find SPDK IP address")
	}
	rr0, err := c4.NVMfRemoteControllerConnect(ctx, &pb.NVMfRemoteControllerConnectRequest{Ctrl: &pb.NVMfRemoteController{Id: 8, Traddr: addr[0].String(), Trsvcid: 4444, Subnqn: "nqn.2016-06.io.spdk:cnode1"}})
	if err != nil {
		log.Fatalf("could not connect to Remote NVMf controller: %v", err)
	}
	log.Printf("Connected: %v", rr0)
	rr2, err := c4.NVMfRemoteControllerReset(ctx, &pb.NVMfRemoteControllerResetRequest{Id: 8})
	if err != nil {
		log.Fatalf("could not reset Remote NVMf controller: %v", err)
	}
	log.Printf("Reset: %v", rr2)
	rr3, err := c4.NVMfRemoteControllerList(ctx, &pb.NVMfRemoteControllerListRequest{Id: 8})
	if err != nil {
		log.Fatalf("could not list Remote NVMf controllerd: %v", err)
	}
	log.Printf("List: %v", rr3)
	rr4, err := c4.NVMfRemoteControllerGet(ctx, &pb.NVMfRemoteControllerGetRequest{Id: 8})
	if err != nil {
		log.Fatalf("could not get Remote NVMf controller: %v", err)
	}
	log.Printf("Got: %v", rr4)
	rr5, err := c4.NVMfRemoteControllerStats(ctx, &pb.NVMfRemoteControllerStatsRequest{Id: 8})
	if err != nil {
		log.Fatalf("could not stats from Remote NVMf controller: %v", err)
	}
	log.Printf("Stats: %v", rr5)
	rr1, err := c4.NVMfRemoteControllerDisconnect(ctx, &pb.NVMfRemoteControllerDisconnectRequest{Id: 8})
	if err != nil {
		log.Fatalf("could not disconnect from Remote NVMf controller: %v", err)
	}
	log.Printf("Disconnected: %v", rr1)

	// NullDebug
	c1 := pb.NewNullDebugServiceClient(conn)
	log.Printf("Testing NewNullDebugServiceClient")
	rs1, err := c1.NullDebugCreate(ctx, &pb.NullDebugCreateRequest{Device: &pb.NullDebug{Handle: &pc.ObjectKey{Value: "OpiNull9"}}})
	if err != nil {
		log.Fatalf("could not create NULL device: %v", err)
	}
	log.Printf("Added: %v", rs1)
	rs3, err := c1.NullDebugUpdate(ctx, &pb.NullDebugUpdateRequest{Device: &pb.NullDebug{Handle: &pc.ObjectKey{Value: "OpiNull9"}}})
	if err != nil {
		log.Fatalf("could not update NULL device: %v", err)
	}
	log.Printf("Updated: %v", rs3)
	rs4, err := c1.NullDebugList(ctx, &pb.NullDebugListRequest{})
	if err != nil {
		log.Fatalf("could not list NULL device: %v", err)
	}
	log.Printf("Listed: %v", rs4)
	rs5, err := c1.NullDebugGet(ctx, &pb.NullDebugGetRequest{Handle: &pc.ObjectKey{Value: "OpiNull9"}})
	if err != nil {
		log.Fatalf("could not get NULL device: %v", err)
	}
	log.Printf("Got: %s", rs5.Handle.Value)
	rs6, err := c1.NullDebugStats(ctx, &pb.NullDebugStatsRequest{Handle: &pc.ObjectKey{Value: "OpiNull9"}})
	if err != nil {
		log.Fatalf("could not stats NULL device: %v", err)
	}
	log.Printf("Stats: %s", rs6.Stats)
	rs2, err := c1.NullDebugDelete(ctx, &pb.NullDebugDeleteRequest{Handle: &pc.ObjectKey{Value: "OpiNull9"}})
	if err != nil {
		log.Fatalf("could not delete NULL device: %v", err)
	}
	log.Printf("Deleted: %v", rs2)

	// Aio
	c2 := pb.NewAioControllerServiceClient(conn)
	log.Printf("Testing NewAioControllerServiceClient")
	ra1, err := c2.AioControllerCreate(ctx, &pb.AioControllerCreateRequest{Device: &pb.AioController{Handle: &pc.ObjectKey{Value: "OpiAio4"}, Filename: "/tmp/aio_bdev_file"}})
	if err != nil {
		log.Fatalf("could not create Aio device: %v", err)
	}
	log.Printf("Added: %v", ra1)
	ra3, err := c2.AioControllerUpdate(ctx, &pb.AioControllerUpdateRequest{Device: &pb.AioController{Handle: &pc.ObjectKey{Value: "OpiAio4"}, Filename: "/tmp/aio_bdev_file"}})
	if err != nil {
		log.Fatalf("could not update Aio device: %v", err)
	}
	log.Printf("Updated: %v", ra3)
	ra4, err := c2.AioControllerGetList(ctx, &pb.AioControllerGetListRequest{})
	if err != nil {
		log.Fatalf("could not list Aio device: %v", err)
	}
	log.Printf("Listed: %v", ra4)
	ra5, err := c2.AioControllerGet(ctx, &pb.AioControllerGetRequest{Handle: &pc.ObjectKey{Value: "OpiAio4"}})
	if err != nil {
		log.Fatalf("could not get Aio device: %v", err)
	}
	log.Printf("Got: %s", ra5.Handle.Value)
	ra6, err := c2.AioControllerGetStats(ctx, &pb.AioControllerGetStatsRequest{Handle: &pc.ObjectKey{Value: "OpiAio4"}})
	if err != nil {
		log.Fatalf("could not stats Aio device: %v", err)
	}
	log.Printf("Stats: %s", ra6.Stats)
	ra2, err := c2.AioControllerDelete(ctx, &pb.AioControllerDeleteRequest{Handle: &pc.ObjectKey{Value: "OpiAio4"}})
	if err != nil {
		log.Fatalf("could not delete Aio device: %v", err)
	}
	log.Printf("Deleted: %v", ra2)
}
