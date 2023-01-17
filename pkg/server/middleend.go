// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// The main package of the storage Server
package server

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

func (s *Server) CreateEncryptedVolume(ctx context.Context, in *pb.CreateEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("CreateEncryptedVolume: Received from client: %v", in)
	params := BdevCryptoCreateParams{
		Name:         in.EncryptedVolume.EncryptedVolumeId.Value,
		BaseBdevName: in.EncryptedVolume.VolumeId.Value,
		CryptoPmd:    "crypto_aesni_mb",
		Key:          string(in.EncryptedVolume.Key),
		Cipher:       "AES_CBC",
	}
	// TODO: use in.EncryptedVolume.Cipher.String()
	var result BdevCryptoCreateResult
	err := call("bdev_crypto_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result == "" {
		msg := fmt.Sprintf("Could not create Crypto: %s", in.EncryptedVolume.EncryptedVolumeId.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := &pb.EncryptedVolume{}
	err = deepcopier.Copy(in.EncryptedVolume).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *Server) DeleteEncryptedVolume(ctx context.Context, in *pb.DeleteEncryptedVolumeRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteEncryptedVolume: Received from client: %v", in)
	params := BdevCryptoDeleteParams{
		Name: in.Name,
	}
	var result BdevCryptoDeleteResult
	err := call("bdev_crypto_delete", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete Crypto: %s", in.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) UpdateEncryptedVolume(ctx context.Context, in *pb.UpdateEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("UpdateEncryptedVolume: Received from client: %v", in)
	params1 := BdevCryptoDeleteParams{
		Name: in.EncryptedVolume.EncryptedVolumeId.Value,
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
		Name:         in.EncryptedVolume.EncryptedVolumeId.Value,
		BaseBdevName: in.EncryptedVolume.VolumeId.Value,
		CryptoPmd:    "crypto_aesni_mb",
		Key:          string(in.EncryptedVolume.Key),
		Cipher:       "AES_CBC",
	}
	// TODO: use in.EncryptedVolume.Cipher.String()
	var result2 BdevCryptoCreateResult
	err2 := call("bdev_crypto_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	response := &pb.EncryptedVolume{}
	err3 := deepcopier.Copy(in.EncryptedVolume).To(response)
	if err3 != nil {
		log.Printf("error: %v", err3)
		return nil, err3
	}
	return response, nil
}

func (s *Server) ListEncryptedVolumes(ctx context.Context, in *pb.ListEncryptedVolumesRequest) (*pb.ListEncryptedVolumesResponse, error) {
	log.Printf("ListEncryptedVolumes: Received from client: %v", in)
	var result []BdevGetBdevsResult
	err := call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.EncryptedVolume, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: r.Name}}
	}
	return &pb.ListEncryptedVolumesResponse{EncryptedVolumes: Blobarray}, nil
}

func (s *Server) GetEncryptedVolume(ctx context.Context, in *pb.GetEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("GetEncryptedVolume: Received from client: %v", in)
	params := BdevGetBdevsParams{
		Name: in.Name,
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
	return &pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: result[0].Name}}, nil
}

func (s *Server) EncryptedVolumeStats(ctx context.Context, in *pb.EncryptedVolumeStatsRequest) (*pb.EncryptedVolumeStatsResponse, error) {
	log.Printf("EncryptedVolumeStats: Received from client: %v", in)
	params := BdevGetIostatParams{
		Name: in.EncryptedVolumeId.Value,
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
	return &pb.EncryptedVolumeStatsResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.Bdevs[0].BytesRead),
		ReadOpsCount:      int32(result.Bdevs[0].NumReadOps),
		WriteBytesCount:   int32(result.Bdevs[0].BytesWritten),
		WriteOpsCount:     int32(result.Bdevs[0].NumWriteOps),
		UnmapBytesCount:   int32(result.Bdevs[0].BytesUnmapped),
		UnmapOpsCount:     int32(result.Bdevs[0].NumUnmapOps),
		ReadLatencyTicks:  int32(result.Bdevs[0].ReadLatencyTicks),
		WriteLatencyTicks: int32(result.Bdevs[0].WriteLatencyTicks),
		UnmapLatencyTicks: int32(result.Bdevs[0].UnmapLatencyTicks),
	}}, nil
}

//////////////////////////////////////////////////////////
