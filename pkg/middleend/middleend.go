// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package middleend implements the MiddleEnd APIs (service) of the storage Server
package middleend

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server contains middleend related OPI services
type Server struct {
	pb.UnimplementedMiddleendServiceServer

	rpc        server.JSONRPC
	Pagination map[string]int
}

// NewServer creates initialized instance of MiddleEnd server communicating
// with provided jsonRPC
func NewServer(jsonRPC server.JSONRPC) *Server {
	return &Server{
		rpc:        jsonRPC,
		Pagination: make(map[string]int),
	}
}

// CreateEncryptedVolume creates an encrypted volume
func (s *Server) CreateEncryptedVolume(_ context.Context, in *pb.CreateEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("CreateEncryptedVolume: Received from client: %v", in)
	if err := s.verifyEncryptedVolume(in.EncryptedVolume); err != nil {
		log.Printf("error: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// first create a key
	params1 := s.getAccelCryptoKeyCreateParams(in.EncryptedVolume)
	var result1 models.AccelCryptoKeyCreateResult
	err1 := s.rpc.Call("accel_crypto_key_create", &params1, &result1)
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
	params := models.BdevCryptoCreateParams{
		Name:         in.EncryptedVolume.EncryptedVolumeId.Value,
		BaseBdevName: in.EncryptedVolume.VolumeId.Value,
		KeyName:      in.EncryptedVolume.EncryptedVolumeId.Value,
	}
	var result models.BdevCryptoCreateResult
	err := s.rpc.Call("bdev_crypto_create", &params, &result)
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
func (s *Server) DeleteEncryptedVolume(_ context.Context, in *pb.DeleteEncryptedVolumeRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteEncryptedVolume: Received from client: %v", in)
	bdevCryptoDeleteParams := models.BdevCryptoDeleteParams{
		Name: in.Name,
	}
	var bdevCryptoDeleteResult models.BdevCryptoDeleteResult
	err := s.rpc.Call("bdev_crypto_delete", &bdevCryptoDeleteParams, &bdevCryptoDeleteResult)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", bdevCryptoDeleteResult)
	if !bdevCryptoDeleteResult {
		msg := fmt.Sprintf("Could not delete Crypto: %s", in.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	keyDestroyParams := models.AccelCryptoKeyDestroyParams{
		KeyName: in.Name,
	}
	var keyDestroyResult models.AccelCryptoKeyDestroyResult
	err = s.rpc.Call("accel_crypto_key_destroy", &keyDestroyParams, &keyDestroyResult)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", keyDestroyResult)
	if !keyDestroyResult {
		msg := fmt.Sprintf("Could not destroy Crypto Key: %v", keyDestroyParams.KeyName)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	return &emptypb.Empty{}, nil
}

// UpdateEncryptedVolume updates an encrypted volume
func (s *Server) UpdateEncryptedVolume(_ context.Context, in *pb.UpdateEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("UpdateEncryptedVolume: Received from client: %v", in)
	if err := s.verifyEncryptedVolume(in.EncryptedVolume); err != nil {
		log.Printf("error: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// first delete old bdev
	params1 := models.BdevCryptoDeleteParams{
		Name: in.EncryptedVolume.EncryptedVolumeId.Value,
	}
	var result1 models.BdevCryptoDeleteResult
	err1 := s.rpc.Call("bdev_crypto_delete", &params1, &result1)
	if err1 != nil {
		log.Printf("error: %v", err1)
		return nil, err1
	}
	log.Printf("Received from SPDK: %v", result1)
	if !result1 {
		msg := fmt.Sprintf("Could not delete Crypto: %s", in.EncryptedVolume.EncryptedVolumeId.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	// now delete a key
	params0 := models.AccelCryptoKeyDestroyParams{
		KeyName: in.EncryptedVolume.EncryptedVolumeId.Value,
	}
	var result0 models.AccelCryptoKeyDestroyResult
	err0 := s.rpc.Call("accel_crypto_key_destroy", &params0, &result0)
	if err0 != nil {
		log.Printf("error: %v", err0)
		return nil, err0
	}
	log.Printf("Received from SPDK: %v", result0)
	if !result0 {
		msg := fmt.Sprintf("Could not destroy Crypto Key: %v", params0.KeyName)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	params2 := s.getAccelCryptoKeyCreateParams(in.EncryptedVolume)
	var result2 models.AccelCryptoKeyCreateResult
	err2 := s.rpc.Call("accel_crypto_key_create", &params2, &result2)
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
	params3 := models.BdevCryptoCreateParams{
		Name:         in.EncryptedVolume.EncryptedVolumeId.Value,
		BaseBdevName: in.EncryptedVolume.VolumeId.Value,
		KeyName:      in.EncryptedVolume.EncryptedVolumeId.Value,
	}
	var result3 models.BdevCryptoCreateResult
	err3 := s.rpc.Call("bdev_crypto_create", &params3, &result3)
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
func (s *Server) ListEncryptedVolumes(_ context.Context, in *pb.ListEncryptedVolumesRequest) (*pb.ListEncryptedVolumesResponse, error) {
	log.Printf("ListEncryptedVolumes: Received from client: %v", in)
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
	var result []models.BdevGetBdevsResult
	err := s.rpc.Call("bdev_get_bdevs", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
	result, hasMoreElements := server.LimitPagination(result, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.EncryptedVolume, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.EncryptedVolume{EncryptedVolumeId: &pc.ObjectKey{Value: r.Name}}
	}
	return &pb.ListEncryptedVolumesResponse{EncryptedVolumes: Blobarray, NextPageToken: token}, nil
}

// GetEncryptedVolume gets an encrypted volume
func (s *Server) GetEncryptedVolume(_ context.Context, in *pb.GetEncryptedVolumeRequest) (*pb.EncryptedVolume, error) {
	log.Printf("GetEncryptedVolume: Received from client: %v", in)
	params := models.BdevGetBdevsParams{
		Name: in.Name,
	}
	var result []models.BdevGetBdevsResult
	err := s.rpc.Call("bdev_get_bdevs", &params, &result)
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
func (s *Server) EncryptedVolumeStats(_ context.Context, in *pb.EncryptedVolumeStatsRequest) (*pb.EncryptedVolumeStatsResponse, error) {
	log.Printf("EncryptedVolumeStats: Received from client: %v", in)
	params := models.BdevGetIostatParams{
		Name: in.EncryptedVolumeId.Value,
	}
	// See https://mholt.github.io/json-to-go/
	var result models.BdevGetIostatResult
	err := s.rpc.Call("bdev_get_iostat", &params, &result)
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

func (s *Server) verifyEncryptedVolume(volume *pb.EncryptedVolume) error {
	keyLengthInBits := len(volume.Key) * 8
	expectedKeyLengthInBits := 0
	switch {
	case volume.Cipher == pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_256:
		expectedKeyLengthInBits = 512
	case volume.Cipher == pb.EncryptionType_ENCRYPTION_TYPE_AES_XTS_128:
		expectedKeyLengthInBits = 256
	default:
		return fmt.Errorf("only AES_XTS_256 and AES_XTS_128 are supported")
	}

	if keyLengthInBits != expectedKeyLengthInBits {
		return fmt.Errorf("expected key size %vb, provided size %vb",
			expectedKeyLengthInBits, keyLengthInBits)
	}

	return nil
}

func (s *Server) getAccelCryptoKeyCreateParams(volume *pb.EncryptedVolume) models.AccelCryptoKeyCreateParams {
	var params models.AccelCryptoKeyCreateParams

	params.Cipher = "AES_XTS"
	keyHalf := len(volume.Key) / 2
	params.Key = hex.EncodeToString(volume.Key[:keyHalf])
	params.Key2 = hex.EncodeToString(volume.Key[keyHalf:])
	params.Name = volume.EncryptedVolumeId.Value

	return params
}
