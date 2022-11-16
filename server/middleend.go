// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// The main package of the storage server
package main

import (
	"context"
	"fmt"
	"log"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

//////////////////////////////////////////////////////////

func (s *server) CreateCrypto(ctx context.Context, in *pb.CreateCryptoRequest) (*pb.Crypto, error) {
	log.Printf("CreateCrypto: Received from client: %v", in)
	params := BdevCryptoCreateParams{
		Name:         in.Volume.CryptoId.Value,
		BaseBdevName: in.Volume.VolumeId.Value,
		CryptoPmd:    "crypto_aesni_mb",
		Key:          string(in.Volume.Key),
		Cipher:       "AES_CBC",
	}
	// TODO: use in.Volume.Cipher.String()
	var result BdevCryptoCreateResult
	err := call("bdev_crypto_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	response := &pb.Crypto{}
	err = deepcopier.Copy(in.Volume).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteCrypto(ctx context.Context, in *pb.DeleteCryptoRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteCrypto: Received from client: %v", in)
	params := BdevCryptoDeleteParams{
		Name: in.CryptoId.Value,
	}
	var result BdevCryptoDeleteResult
	err := call("bdev_crypto_delete", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateCrypto(ctx context.Context, in *pb.UpdateCryptoRequest) (*pb.Crypto, error) {
	log.Printf("UpdateCrypto: Received from client: %v", in)
	params1 := BdevCryptoDeleteParams{
		Name: in.Volume.CryptoId.Value,
	}
	var result1 BdevCryptoDeleteResult
	err1 := call("bdev_crypto_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		log.Printf("Could not delete: %v", in)
	}
	params2 := BdevCryptoCreateParams{
		Name:         in.Volume.CryptoId.Value,
		BaseBdevName: in.Volume.VolumeId.Value,
		CryptoPmd:    "crypto_aesni_mb",
		Key:          string(in.Volume.Key),
		Cipher:       "AES_CBC",
	}
	// TODO: use in.Volume.Cipher.String()
	var result2 BdevCryptoCreateResult
	err2 := call("bdev_crypto_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	response := &pb.Crypto{}
	err3 := deepcopier.Copy(in.Volume).To(response)
	if err3 != nil {
		log.Printf("error: %v", err3)
		return nil, err3
	}
	return response, nil
}

func (s *server) ListCrypto(ctx context.Context, in *pb.ListCryptoRequest) (*pb.ListCryptoResponse, error) {
	log.Printf("ListCrypto: Received from client: %v", in)
	var result []BdevGetBdevsResult
	err := call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.Crypto, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.Crypto{CryptoId: &pc.ObjectKey{Value: r.Name}}
	}
	return &pb.ListCryptoResponse{Volumes: Blobarray}, nil
}

func (s *server) GetCrypto(ctx context.Context, in *pb.GetCryptoRequest) (*pb.Crypto, error) {
	log.Printf("GetCrypto: Received from client: %v", in)
	params := BdevGetBdevsParams{
		Name: in.CryptoId.Value,
	}
	var result []BdevGetBdevsResult
	err := call("bdev_get_bdevs", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result))
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.Crypto{CryptoId: &pc.ObjectKey{Value: result[0].Name}}, nil
}

func (s *server) CryptoStats(ctx context.Context, in *pb.CryptoStatsRequest) (*pb.CryptoStatsResponse, error) {
	log.Printf("CryptoStats: Received from client: %v", in)
	params := BdevGetIostatParams{
		Name: in.CryptoId.Value,
	}
	// See https://mholt.github.io/json-to-go/
	var result BdevGetIostatResult
	err := call("bdev_get_iostat", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if len(result.Bdevs) != 1 {
		msg := fmt.Sprintf("expecting exactly 1 result, got %d", len(result.Bdevs))
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.CryptoStatsResponse{Stats: fmt.Sprint(result.Bdevs[0])}, nil
}

//////////////////////////////////////////////////////////
