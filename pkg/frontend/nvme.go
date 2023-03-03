// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"context"
	"fmt"
	"log"
	"net"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/models"

	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateNVMeSubsystem creates an NVMe Subsystem
func (s *Server) CreateNVMeSubsystem(ctx context.Context, in *pb.CreateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("CreateNVMeSubsystem: Received from client: %v", in)
	params := models.NvmfCreateSubsystemParams{
		Nqn:           in.NvMeSubsystem.Spec.Nqn,
		SerialNumber:  in.NvMeSubsystem.Spec.SerialNumber,
		ModelNumber:   in.NvMeSubsystem.Spec.ModelNumber,
		AllowAnyHost:  true,
		MaxNamespaces: int(in.NvMeSubsystem.Spec.MaxNamespaces),
	}
	var result models.NvmfCreateSubsystemResult
	err := s.RPC.Call("nvmf_create_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not create NQN: %s", in.NvMeSubsystem.Spec.Nqn)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	var ver models.GetVersionResult
	err = s.RPC.Call("spdk_get_version", nil, &ver)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", ver)
	response := &pb.NVMeSubsystem{}
	err = deepcopier.Copy(in.NvMeSubsystem).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	response.Status = &pb.NVMeSubsystemStatus{FirmwareRevision: ver.Version}
	s.Nvme.Subsystems[in.NvMeSubsystem.Spec.Id.Value] = response
	return response, nil
}

// DeleteNVMeSubsystem deletes an NVMe Subsystem
func (s *Server) DeleteNVMeSubsystem(ctx context.Context, in *pb.DeleteNVMeSubsystemRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeSubsystem: Received from client: %v", in)
	subsys, ok := s.Nvme.Subsystems[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.NvmfDeleteSubsystemParams{
		Nqn: subsys.Spec.Nqn,
	}
	var result models.NvmfDeleteSubsystemResult
	err := s.RPC.Call("nvmf_delete_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete NQN: %s", subsys.Spec.Nqn)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Nvme.Subsystems, subsys.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeSubsystem updates an NVMe Subsystem
func (s *Server) UpdateNVMeSubsystem(ctx context.Context, in *pb.UpdateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("UpdateNVMeSubsystem: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeSubsystem method is not implemented")
}

// ListNVMeSubsystems lists NVMe Subsystems
func (s *Server) ListNVMeSubsystems(ctx context.Context, in *pb.ListNVMeSubsystemsRequest) (*pb.ListNVMeSubsystemsResponse, error) {
	log.Printf("ListNVMeSubsystems: Received from client: %v", in)
	var result []models.NvmfGetSubsystemsResult
	err := s.RPC.Call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.NVMeSubsystem, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Nqn, SerialNumber: r.SerialNumber, ModelNumber: r.ModelNumber}}
	}
	return &pb.ListNVMeSubsystemsResponse{NvMeSubsystems: Blobarray}, nil
}

// GetNVMeSubsystem gets NVMe Subsystems
func (s *Server) GetNVMeSubsystem(ctx context.Context, in *pb.GetNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("GetNVMeSubsystem: Received from client: %v", in)
	subsys, ok := s.Nvme.Subsystems[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	var result []models.NvmfGetSubsystemsResult
	err := s.RPC.Call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	for i := range result {
		r := &result[i]
		if r.Nqn == subsys.Spec.Nqn {
			return &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Nqn, SerialNumber: r.SerialNumber, ModelNumber: r.ModelNumber}, Status: &pb.NVMeSubsystemStatus{FirmwareRevision: "TBD"}}, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Spec.Nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// NVMeSubsystemStats gets NVMe Subsystem stats
func (s *Server) NVMeSubsystemStats(ctx context.Context, in *pb.NVMeSubsystemStatsRequest) (*pb.NVMeSubsystemStatsResponse, error) {
	log.Printf("NVMeSubsystemStats: Received from client: %v", in)
	var result models.NvmfGetSubsystemStatsResult
	err := s.RPC.Call("nvmf_get_stats", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NVMeSubsystemStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNVMeController creates an NVMe controller
func (s *Server) CreateNVMeController(ctx context.Context, in *pb.CreateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.NvMeController)
	subsys, ok := s.Nvme.Subsystems[in.NvMeController.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.NvMeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// get SPDK local IP
	addrs, err := net.LookupIP("spdk")
	if err != nil {
		log.Printf("error: %v", err)
		// assume localhost
		addrs = []net.IP{net.ParseIP("127.0.0.1")}
		// return nil, err
	}
	params := models.NvmfSubsystemAddListenerParams{
		Nqn: subsys.Spec.Nqn,
	}
	params.ListenAddress.Trtype = "tcp"
	params.ListenAddress.Traddr = addrs[0].String()
	params.ListenAddress.Trsvcid = "4444"
	params.ListenAddress.Adrfam = "ipv4"

	var result models.NvmfSubsystemAddListenerResult
	err = s.RPC.Call("nvmf_subsystem_add_listener", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not create CTRL: %s", in.NvMeController.Spec.Id.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Nvme.Controllers[in.NvMeController.Spec.Id.Value] = in.NvMeController
	s.Nvme.Controllers[in.NvMeController.Spec.Id.Value].Spec.NvmeControllerId = -1
	s.Nvme.Controllers[in.NvMeController.Spec.Id.Value].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: &pc.ObjectKey{Value: "TBD"}}}
	err = deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// DeleteNVMeController deletes an NVMe controller
func (s *Server) DeleteNVMeController(ctx context.Context, in *pb.DeleteNVMeControllerRequest) (*emptypb.Empty, error) {
	log.Printf("Received from client: %v", in.Name)
	controller, ok := s.Nvme.Controllers[in.Name]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsys, ok := s.Nvme.Subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	// get SPDK local IP
	addrs, err := net.LookupIP("spdk")
	if err != nil {
		log.Printf("error: %v", err)
		// assume localhost
		addrs = []net.IP{net.ParseIP("127.0.0.1")}
		// return nil, err
	}
	params := models.NvmfSubsystemAddListenerParams{
		Nqn: subsys.Spec.Nqn,
	}
	params.ListenAddress.Trtype = "tcp"
	params.ListenAddress.Traddr = addrs[0].String()
	params.ListenAddress.Trsvcid = "4444"
	params.ListenAddress.Adrfam = "ipv4"

	var result models.NvmfSubsystemAddListenerResult
	err = s.RPC.Call("nvmf_subsystem_remove_listener", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete NQN:ID %s:%d", subsys.Spec.Nqn, controller.Spec.NvmeControllerId)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Nvme.Controllers, controller.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeController updates an NVMe controller
func (s *Server) UpdateNVMeController(ctx context.Context, in *pb.UpdateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("UpdateNVMeController: Received from client: %v", in)
	s.Nvme.Controllers[in.NvMeController.Spec.Id.Value] = in.NvMeController
	s.Nvme.Controllers[in.NvMeController.Spec.Id.Value].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{}
	err := deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// ListNVMeControllers lists NVMe controllers
func (s *Server) ListNVMeControllers(ctx context.Context, in *pb.ListNVMeControllersRequest) (*pb.ListNVMeControllersResponse, error) {
	log.Printf("Received from client: %v", in.Parent)
	Blobarray := []*pb.NVMeController{}
	for _, controller := range s.Nvme.Controllers {
		Blobarray = append(Blobarray, controller)
	}
	return &pb.ListNVMeControllersResponse{NvMeControllers: Blobarray}, nil
}

// GetNVMeController gets an NVMe controller
func (s *Server) GetNVMeController(ctx context.Context, in *pb.GetNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.Name)
	controller, ok := s.Nvme.Controllers[in.Name]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	return &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: &pc.ObjectKey{Value: in.Name}, NvmeControllerId: controller.Spec.NvmeControllerId}, Status: &pb.NVMeControllerStatus{Active: true}}, nil
}

// NVMeControllerStats gets an NVMe controller stats
func (s *Server) NVMeControllerStats(ctx context.Context, in *pb.NVMeControllerStatsRequest) (*pb.NVMeControllerStatsResponse, error) {
	log.Printf("NVMeControllerStats: Received from client: %v", in)
	return &pb.NVMeControllerStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNVMeNamespace creates an NVMe namespace
func (s *Server) CreateNVMeNamespace(ctx context.Context, in *pb.CreateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("CreateNVMeNamespace: Received from client: %v", in)
	subsys, ok := s.Nvme.Subsystems[in.NvMeNamespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.NvMeNamespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.NvmfSubsystemAddNsParams{
		Nqn: subsys.Spec.Nqn,
	}

	// TODO: using bdev for volume id as a middle end handle for now
	params.Namespace.Nsid = int(in.NvMeNamespace.Spec.HostNsid)
	params.Namespace.BdevName = in.NvMeNamespace.Spec.VolumeId.Value

	var result models.NvmfSubsystemAddNsResult
	err := s.RPC.Call("nvmf_subsystem_add_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result < 0 {
		msg := fmt.Sprintf("Could not create NS: %s", in.NvMeNamespace.Spec.Id.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Nvme.Namespaces[in.NvMeNamespace.Spec.Id.Value] = in.NvMeNamespace

	response := &pb.NVMeNamespace{}
	err = deepcopier.Copy(in.NvMeNamespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	response.Status = &pb.NVMeNamespaceStatus{PciState: 2, PciOperState: 1}
	return response, nil
}

// DeleteNVMeNamespace deletes an NVMe namespace
func (s *Server) DeleteNVMeNamespace(ctx context.Context, in *pb.DeleteNVMeNamespaceRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeNamespace: Received from client: %v", in)
	namespace, ok := s.Nvme.Namespaces[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := s.Nvme.Subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.NvmfSubsystemRemoveNsParams{
		Nqn:  subsys.Spec.Nqn,
		Nsid: int(namespace.Spec.HostNsid),
	}
	var result models.NvmfSubsystemRemoveNsResult
	err := s.RPC.Call("nvmf_subsystem_remove_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not delete NS: %s", in.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Nvme.Namespaces, namespace.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeNamespace updates an NVMe namespace
func (s *Server) UpdateNVMeNamespace(ctx context.Context, in *pb.UpdateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("UpdateNVMeNamespace: Received from client: %v", in)
	s.Nvme.Namespaces[in.NvMeNamespace.Spec.Id.Value] = in.NvMeNamespace
	s.Nvme.Namespaces[in.NvMeNamespace.Spec.Id.Value].Status = &pb.NVMeNamespaceStatus{PciState: 2, PciOperState: 1}

	response := &pb.NVMeNamespace{}
	err := deepcopier.Copy(in.NvMeNamespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// ListNVMeNamespaces lists NVMe namespaces
func (s *Server) ListNVMeNamespaces(ctx context.Context, in *pb.ListNVMeNamespacesRequest) (*pb.ListNVMeNamespacesResponse, error) {
	log.Printf("ListNVMeNamespaces: Received from client: %v", in)

	nqn := ""
	if in.Parent != "" {
		subsys, ok := s.Nvme.Subsystems[in.Parent]
		if !ok {
			err := fmt.Errorf("unable to find subsystem %s", in.Parent)
			log.Printf("error: %v", err)
			return nil, err
		}
		nqn = subsys.Spec.Nqn
	}
	var result []models.NvmfGetSubsystemsResult
	err := s.RPC.Call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	Blobarray := []*pb.NVMeNamespace{}
	for i := range result {
		rr := &result[i]
		if rr.Nqn == nqn || nqn == "" {
			for j := range rr.Namespaces {
				r := &rr.Namespaces[j]
				Blobarray = append(Blobarray, &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{HostNsid: int32(r.Nsid)}})
			}
		}
	}
	if len(Blobarray) > 0 {
		return &pb.ListNVMeNamespacesResponse{NvMeNamespaces: Blobarray}, nil
	}

	msg := fmt.Sprintf("Could not find any namespaces for NQN: %s", nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// GetNVMeNamespace gets an NVMe namespace
func (s *Server) GetNVMeNamespace(ctx context.Context, in *pb.GetNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("GetNVMeNamespace: Received from client: %v", in)
	namespace, ok := s.Nvme.Namespaces[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: do we even query SPDK to confirm if namespace is present?
	// return namespace, nil

	// fetch subsystems -> namespaces from Server, match the nsid to find the corresponding namespace
	subsys, ok := s.Nvme.Subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	var result []models.NvmfGetSubsystemsResult
	err := s.RPC.Call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	for i := range result {
		rr := &result[i]
		if rr.Nqn == subsys.Spec.Nqn {
			for j := range rr.Namespaces {
				r := &rr.Namespaces[j]
				if int32(r.Nsid) == namespace.Spec.HostNsid {
					return &pb.NVMeNamespace{
						Spec:   &pb.NVMeNamespaceSpec{Id: namespace.Spec.Id, HostNsid: namespace.Spec.HostNsid},
						Status: &pb.NVMeNamespaceStatus{PciState: 2, PciOperState: 1},
					}, nil
				}
			}
			msg := fmt.Sprintf("Could not find NSID: %d", namespace.Spec.HostNsid)
			log.Print(msg)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Spec.Nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// NVMeNamespaceStats gets an NVMe namespace stats
func (s *Server) NVMeNamespaceStats(ctx context.Context, in *pb.NVMeNamespaceStatsRequest) (*pb.NVMeNamespaceStatsResponse, error) {
	log.Printf("NVMeNamespaceStats: Received from client: %v", in)
	return &pb.NVMeNamespaceStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}
