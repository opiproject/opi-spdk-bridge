// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

// Package server is he main package of the storage Server
package server

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateEncryptedVolume creates an encrypted volume
func (s *Server) CreateEncryptedVolume(ctx context.Context, in *pb.CreateEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("CreateEncryptedVolume: Received from client: %v", in)
	// first create a key
	r := regexp.MustCompile("ENCRYPTION_TYPE_([A-Z_]+)_")
	if !r.MatchString(in.EncryptedVolume.Cipher.String()) {
		msg := fmt.Sprintf("Could not parse Crypto Cipher: %s", in.EncryptedVolume.Cipher.String())
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	params1 := AccelCryptoKeyCreateParams{
		Cipher: r.FindStringSubmatch(in.EncryptedVolume.Cipher.String())[1],
		Name:   "super_key",
		Key:    string(in.EncryptedVolume.Key),
		Key2:   strings.Repeat("a", len(in.EncryptedVolume.Key)),
	}
	// TODO: don't use hard-coded key name
	var result1 AccelCryptoKeyCreateResult
	err1 := call("accel_crypto_key_create", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		msg := fmt.Sprintf("Could not create Crypto Key: %s", string(in.EncryptedVolume.Key))
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	// create bdev now
	params := BdevCryptoCreateParams{
		Name:         in.EncryptedVolume.EncryptedVolumeId.Value,
		BaseBdevName: in.EncryptedVolume.VolumeId.Value,
		KeyName:      "super_key",
	}
	var result BdevCryptoCreateResult
	err := call("bdev_crypto_create", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result == "" {
		msg := fmt.Sprintf("Could not create Crypto Dev: %s", in.EncryptedVolume.EncryptedVolumeId.Value)
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

// DeleteEncryptedVolume deletes an encrypted volume
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

// UpdateEncryptedVolume updates an encrypted volume
func (s *Server) UpdateEncryptedVolume(ctx context.Context, in *pb.UpdateEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("UpdateEncryptedVolume: Received from client: %v", in)
	// first delete old bdev
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
	// now delete a key
	params0 := AccelCryptoKeyDestroyParams{
		KeyName: "super_key",
	}
	var result0 AccelCryptoKeyDestroyResult
	err0 := call("accel_crypto_key_destroy", &params0, &result0)
	if err0 != nil {
		log.Printf("error: %v", err0)
		return nil, err0
	}
	log.Printf("Received from SPDK: %v", result0)
	if !result0 {
		log.Printf("Could not destroy Crypto Key: %v", in)
	}
	// now create a new key
	r := regexp.MustCompile("ENCRYPTION_TYPE_([A-Z_]+)_")
	if !r.MatchString(in.EncryptedVolume.Cipher.String()) {
		msg := fmt.Sprintf("Could not parse Crypto Cipher: %s", in.EncryptedVolume.Cipher.String())
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	params2 := AccelCryptoKeyCreateParams{
		Cipher: r.FindStringSubmatch(in.EncryptedVolume.Cipher.String())[1],
		Name:   "super_key",
		Key:    string(in.EncryptedVolume.Key),
		Key2:   strings.Repeat("b", len(in.EncryptedVolume.Key)),
	}
	// TODO: don't use hard-coded key name
	var result2 AccelCryptoKeyCreateResult
	err2 := call("accel_crypto_key_create", &params2, &result2)
	if err2 != nil {
		log.Printf("error: %v", err2)
		return nil, err2
	}
	log.Printf("Received from SPDK: %v", result2)
	if !result2 {
		msg := fmt.Sprintf("Could not create Crypto Key: %s", string(in.EncryptedVolume.Key))
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	// create bdev now
	params3 := BdevCryptoCreateParams{
		Name:         in.EncryptedVolume.EncryptedVolumeId.Value,
		BaseBdevName: in.EncryptedVolume.VolumeId.Value,
		KeyName:      "super_key",
	}
	var result3 BdevCryptoCreateResult
	err3 := call("bdev_crypto_create", &params3, &result3)
	if err3 != nil {
		log.Printf("error: %v", err3)
		return nil, err3
	}
	log.Printf("Received from SPDK: %v", result3)
	if result3 == "" {
		msg := fmt.Sprintf("Could not create Crypto Dev: %s", in.EncryptedVolume.EncryptedVolumeId.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	// return result
	response := &pb.EncryptedVolume{}
	err4 := deepcopier.Copy(in.EncryptedVolume).To(response)
	if err4 != nil {
		log.Printf("error: %v", err4)
		return nil, err4
	}
	return response, nil
}

// ListEncryptedVolumes lists encrypted volumes
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

// GetEncryptedVolume gets an encrypted volume
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

// EncryptedVolumeStats gets an encrypted volume stats
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
