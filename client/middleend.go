package main

import (
	"context"
	"log"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
)

func doMiddleend(ctx context.Context, conn grpc.ClientConnInterface) {
	log.Printf("Test middleend")

	// Crypto
	c1 := pb.NewMiddleendServiceClient(conn)
	log.Printf("Testing NewCryptoServiceClient")
	rs1, err := c1.CreateCrypto(ctx, &pb.CreateCryptoRequest{
		Volume: &pb.Crypto{
			CryptoId: &pc.ObjectKey{Value: "OpiCrypto3"},
			VolumeId: &pc.ObjectKey{Value: "Malloc1"},
			Key:      []byte("0123456789abcdef0123456789abcdef"),
		},
	})
	if err != nil {
		log.Fatalf("could not create CRYPTO device: %v", err)
	}
	log.Printf("Added: %v", rs1)
	rs3, err := c1.UpdateCrypto(ctx, &pb.UpdateCryptoRequest{
		Volume: &pb.Crypto{
			CryptoId: &pc.ObjectKey{Value: "OpiCrypto3"},
			VolumeId: &pc.ObjectKey{Value: "Malloc1"},
			Key:      []byte("0123456789abcdef0123456789abcdef"),
		},
	})
	if err != nil {
		log.Fatalf("could not update CRYPTO device: %v", err)
	}
	log.Printf("Updated: %v", rs3)
	rs4, err := c1.ListCrypto(ctx, &pb.ListCryptoRequest{})
	if err != nil {
		log.Fatalf("could not list CRYPTO device: %v", err)
	}
	log.Printf("Listed: %v", rs4)
	rs5, err := c1.GetCrypto(ctx, &pb.GetCryptoRequest{CryptoId: &pc.ObjectKey{Value: "OpiCrypto3"}})
	if err != nil {
		log.Fatalf("could not get CRYPTO device: %v", err)
	}
	log.Printf("Got: %s", rs5.CryptoId.Value)
	rs6, err := c1.CryptoStats(ctx, &pb.CryptoStatsRequest{CryptoId: &pc.ObjectKey{Value: "OpiCrypto3"}})
	if err != nil {
		log.Fatalf("could not stats CRYPTO device: %v", err)
	}
	log.Printf("Stats: %s", rs6.Stats)
	rs2, err := c1.DeleteCrypto(ctx, &pb.DeleteCryptoRequest{CryptoId: &pc.ObjectKey{Value: "OpiCrypto3"}})
	if err != nil {
		log.Fatalf("could not delete CRYPTO device: %v", err)
	}
	log.Printf("Deleted: %v", rs2)
}
