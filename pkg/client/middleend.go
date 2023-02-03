// Package client implements the storage client
package client

import (
	"context"
	"log"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
)

// DoMiddleend executes the middle end code
func DoMiddleend(ctx context.Context, conn grpc.ClientConnInterface) {
	// EncryptedVolume
	c1 := pb.NewMiddleendServiceClient(conn)
	log.Printf("==============================================================================")
	log.Printf("Testing NewMiddleendServiceClient")
	rs1, err := c1.CreateEncryptedVolume(ctx, &pb.CreateEncryptedVolumeRequest{
		EncryptedVolume: &pb.EncryptedVolume{
			EncryptedVolumeId: &pc.ObjectKey{Value: "OpiEncryptedVolume3"},
			VolumeId:          &pc.ObjectKey{Value: "Malloc1"},
			Key:               []byte("0123456789abcdef0123456789abcdef"),
		},
	})
	if err != nil {
		log.Fatalf("could not create CRYPTO device: %v", err)
	}
	log.Printf("Added: %v", rs1)
	rs3, err := c1.UpdateEncryptedVolume(ctx, &pb.UpdateEncryptedVolumeRequest{
		EncryptedVolume: &pb.EncryptedVolume{
			EncryptedVolumeId: &pc.ObjectKey{Value: "OpiEncryptedVolume3"},
			VolumeId:          &pc.ObjectKey{Value: "Malloc1"},
			Key:               []byte("0123456789abcdef0123456789abcdef"),
		},
	})
	if err != nil {
		log.Fatalf("could not update CRYPTO device: %v", err)
	}
	log.Printf("Updated: %v", rs3)
	rs4, err := c1.ListEncryptedVolumes(ctx, &pb.ListEncryptedVolumesRequest{})
	if err != nil {
		log.Fatalf("could not list CRYPTO device: %v", err)
	}
	log.Printf("Listed: %v", rs4)
	rs5, err := c1.GetEncryptedVolume(ctx, &pb.GetEncryptedVolumeRequest{Name: "OpiEncryptedVolume3"})
	if err != nil {
		log.Fatalf("could not get CRYPTO device: %v", err)
	}
	log.Printf("Got: %s", rs5.EncryptedVolumeId.Value)
	rs6, err := c1.EncryptedVolumeStats(ctx, &pb.EncryptedVolumeStatsRequest{EncryptedVolumeId: &pc.ObjectKey{Value: "OpiEncryptedVolume3"}})
	if err != nil {
		log.Fatalf("could not stats CRYPTO device: %v", err)
	}
	log.Printf("Stats: %s", rs6.Stats)
	rs2, err := c1.DeleteEncryptedVolume(ctx, &pb.DeleteEncryptedVolumeRequest{Name: "OpiEncryptedVolume3"})
	if err != nil {
		log.Fatalf("could not delete CRYPTO device: %v", err)
	}
	log.Printf("Deleted: %v", rs2)
}
