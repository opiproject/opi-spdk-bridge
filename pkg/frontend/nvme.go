// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	ipv4NvmeTCPProtocol = "ipv4"
	ipv6NvmeTCPProtocol = "ipv6"
)

// TODO: consider using https://pkg.go.dev/net#TCPAddr
type tcpSubsystemListener struct {
	listenAddr net.IP
	listenPort string
	protocol   string
}

func sortNVMeSubsystems(subsystems []*pb.NVMeSubsystem) {
	sort.Slice(subsystems, func(i int, j int) bool {
		return subsystems[i].Spec.Nqn < subsystems[j].Spec.Nqn
	})
}

func sortNVMeControllers(controllers []*pb.NVMeController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Spec.Name < controllers[j].Spec.Name
	})
}

func sortNVMeNamespaces(namespaces []*pb.NVMeNamespace) {
	sort.Slice(namespaces, func(i int, j int) bool {
		return namespaces[i].Spec.HostNsid < namespaces[j].Spec.HostNsid
	})
}

// NewTCPSubsystemListener creates a new instance of tcpSubsystemListener
func NewTCPSubsystemListener(listenAddr string) SubsystemListener {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		log.Panicf("Invalid ip:port tuple: %v", listenAddr)
	}

	parsedAddr := net.ParseIP(host)
	if parsedAddr == nil {
		log.Panicf("Invalid ip address: %v", host)
	}

	var protocol string
	switch {
	case parsedAddr.To4() != nil:
		protocol = ipv4NvmeTCPProtocol
	case parsedAddr.To16() != nil:
		protocol = ipv6NvmeTCPProtocol
	default:
		log.Panicf("Not supported protocol for: %v", listenAddr)
	}

	return &tcpSubsystemListener{
		listenAddr: parsedAddr,
		listenPort: port,
		protocol:   protocol,
	}
}

func (c *tcpSubsystemListener) Params(_ *pb.NVMeController, nqn string) spdk.NvmfSubsystemAddListenerParams {
	result := spdk.NvmfSubsystemAddListenerParams{}
	result.Nqn = nqn
	result.ListenAddress.Trtype = "tcp"
	result.ListenAddress.Traddr = c.listenAddr.String()
	result.ListenAddress.Trsvcid = c.listenPort
	result.ListenAddress.Adrfam = c.protocol

	return result
}

// CreateNVMeSubsystem creates an NVMe Subsystem
func (s *Server) CreateNVMeSubsystem(_ context.Context, in *pb.CreateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("CreateNVMeSubsystem: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	name := uuid.New().String()
	if in.NvMeSubsystemId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMeSubsystemId, in.NvMeSubsystem.Spec.Name)
		name = in.NvMeSubsystemId
	}
	in.NvMeSubsystem.Spec.Name = name
	// idempotent API when called with same key, should return same object
	subsys, ok := s.Nvme.Subsystems[in.NvMeSubsystem.Spec.Name]
	if ok {
		log.Printf("Already existing NVMeSubsystem with id %v", in.NvMeSubsystem.Spec.Name)
		return subsys, nil
	}
	// not found, so create a new one
	params := spdk.NvmfCreateSubsystemParams{
		Nqn:           in.NvMeSubsystem.Spec.Nqn,
		SerialNumber:  in.NvMeSubsystem.Spec.SerialNumber,
		ModelNumber:   in.NvMeSubsystem.Spec.ModelNumber,
		AllowAnyHost:  true,
		MaxNamespaces: int(in.NvMeSubsystem.Spec.MaxNamespaces),
	}
	var result spdk.NvmfCreateSubsystemResult
	err := s.rpc.Call("nvmf_create_subsystem", &params, &result)
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
	var ver spdk.GetVersionResult
	err = s.rpc.Call("spdk_get_version", nil, &ver)
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
	s.Nvme.Subsystems[in.NvMeSubsystem.Spec.Name] = response
	return response, nil
}

// DeleteNVMeSubsystem deletes an NVMe Subsystem
func (s *Server) DeleteNVMeSubsystem(_ context.Context, in *pb.DeleteNVMeSubsystemRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeSubsystem: Received from client: %v", in)
	subsys, ok := s.Nvme.Subsystems[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.NvmfDeleteSubsystemParams{
		Nqn: subsys.Spec.Nqn,
	}
	var result spdk.NvmfDeleteSubsystemResult
	err := s.rpc.Call("nvmf_delete_subsystem", &params, &result)
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
	delete(s.Nvme.Subsystems, subsys.Spec.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeSubsystem updates an NVMe Subsystem
func (s *Server) UpdateNVMeSubsystem(_ context.Context, in *pb.UpdateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("UpdateNVMeSubsystem: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeSubsystem method is not implemented")
}

// ListNVMeSubsystems lists NVMe Subsystems
func (s *Server) ListNVMeSubsystems(_ context.Context, in *pb.ListNVMeSubsystemsRequest) (*pb.ListNVMeSubsystemsResponse, error) {
	log.Printf("ListNVMeSubsystems: Received from client: %v", in)
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
	var result []spdk.NvmfGetSubsystemsResult
	err := s.rpc.Call("nvmf_get_subsystems", nil, &result)
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
	Blobarray := make([]*pb.NVMeSubsystem, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Nqn, SerialNumber: r.SerialNumber, ModelNumber: r.ModelNumber}}
	}
	sortNVMeSubsystems(Blobarray)
	return &pb.ListNVMeSubsystemsResponse{NvMeSubsystems: Blobarray, NextPageToken: token}, nil
}

// GetNVMeSubsystem gets NVMe Subsystems
func (s *Server) GetNVMeSubsystem(_ context.Context, in *pb.GetNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("GetNVMeSubsystem: Received from client: %v", in)
	subsys, ok := s.Nvme.Subsystems[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	var result []spdk.NvmfGetSubsystemsResult
	err := s.rpc.Call("nvmf_get_subsystems", nil, &result)
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
func (s *Server) NVMeSubsystemStats(_ context.Context, in *pb.NVMeSubsystemStatsRequest) (*pb.NVMeSubsystemStatsResponse, error) {
	log.Printf("NVMeSubsystemStats: Received from client: %v", in)
	var result spdk.NvmfGetSubsystemStatsResult
	err := s.rpc.Call("nvmf_get_stats", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NVMeSubsystemStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNVMeController creates an NVMe controller
func (s *Server) CreateNVMeController(_ context.Context, in *pb.CreateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.NvMeController)
	// see https://google.aip.dev/133#user-specified-ids
	name := uuid.New().String()
	if in.NvMeControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMeControllerId, in.NvMeController.Spec.Name)
		name = in.NvMeControllerId
	}
	in.NvMeController.Spec.Name = name
	// idempotent API when called with same key, should return same object
	controller, ok := s.Nvme.Controllers[in.NvMeController.Spec.Name]
	if ok {
		log.Printf("Already existing NVMeController with id %v", in.NvMeController.Spec.Name)
		return controller, nil
	}
	// not found, so create a new one
	subsys, ok := s.Nvme.Subsystems[in.NvMeController.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.NvMeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := s.Nvme.subsysListener.Params(in.NvMeController, subsys.Spec.Nqn)
	var result spdk.NvmfSubsystemAddListenerResult
	err := s.rpc.Call("nvmf_subsystem_add_listener", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		msg := fmt.Sprintf("Could not create CTRL: %s", in.NvMeController.Spec.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Nvme.Controllers[in.NvMeController.Spec.Name] = in.NvMeController
	s.Nvme.Controllers[in.NvMeController.Spec.Name].Spec.NvmeControllerId = -1
	s.Nvme.Controllers[in.NvMeController.Spec.Name].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Name: "TBD"}}
	err = deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// DeleteNVMeController deletes an NVMe controller
func (s *Server) DeleteNVMeController(_ context.Context, in *pb.DeleteNVMeControllerRequest) (*emptypb.Empty, error) {
	log.Printf("Received from client: %v", in.Name)
	controller, ok := s.Nvme.Controllers[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := s.Nvme.Subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := s.Nvme.subsysListener.Params(controller, subsys.Spec.Nqn)
	var result spdk.NvmfSubsystemAddListenerResult
	err := s.rpc.Call("nvmf_subsystem_remove_listener", &params, &result)
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
	delete(s.Nvme.Controllers, controller.Spec.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeController updates an NVMe controller
func (s *Server) UpdateNVMeController(_ context.Context, in *pb.UpdateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("UpdateNVMeController: Received from client: %v", in)
	s.Nvme.Controllers[in.NvMeController.Spec.Name] = in.NvMeController
	s.Nvme.Controllers[in.NvMeController.Spec.Name].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{}
	err := deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// ListNVMeControllers lists NVMe controllers
func (s *Server) ListNVMeControllers(_ context.Context, in *pb.ListNVMeControllersRequest) (*pb.ListNVMeControllersResponse, error) {
	log.Printf("Received from client: %v", in.Parent)
	Blobarray := []*pb.NVMeController{}
	for _, controller := range s.Nvme.Controllers {
		Blobarray = append(Blobarray, controller)
	}
	sortNVMeControllers(Blobarray)
	token := uuid.New().String()
	s.Pagination[token] = int(in.PageSize)
	return &pb.ListNVMeControllersResponse{NvMeControllers: Blobarray, NextPageToken: token}, nil
}

// GetNVMeController gets an NVMe controller
func (s *Server) GetNVMeController(_ context.Context, in *pb.GetNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.Name)
	controller, ok := s.Nvme.Controllers[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	return &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Name: in.Name, NvmeControllerId: controller.Spec.NvmeControllerId}, Status: &pb.NVMeControllerStatus{Active: true}}, nil
}

// NVMeControllerStats gets an NVMe controller stats
func (s *Server) NVMeControllerStats(_ context.Context, in *pb.NVMeControllerStatsRequest) (*pb.NVMeControllerStatsResponse, error) {
	log.Printf("NVMeControllerStats: Received from client: %v", in)
	return &pb.NVMeControllerStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNVMeNamespace creates an NVMe namespace
func (s *Server) CreateNVMeNamespace(_ context.Context, in *pb.CreateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("CreateNVMeNamespace: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	name := uuid.New().String()
	if in.NvMeNamespaceId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMeNamespaceId, in.NvMeNamespace.Spec.Name)
		name = in.NvMeNamespaceId
	}
	in.NvMeNamespace.Spec.Name = name
	// idempotent API when called with same key, should return same object
	namespace, ok := s.Nvme.Namespaces[in.NvMeNamespace.Spec.Name]
	if ok {
		log.Printf("Already existing NVMeNamespace with id %v", in.NvMeNamespace.Spec.Name)
		return namespace, nil
	}
	// not found, so create a new one
	subsys, ok := s.Nvme.Subsystems[in.NvMeNamespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.NvMeNamespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.NvmfSubsystemAddNsParams{
		Nqn: subsys.Spec.Nqn,
	}

	// TODO: using bdev for volume id as a middle end handle for now
	params.Namespace.Nsid = int(in.NvMeNamespace.Spec.HostNsid)
	params.Namespace.BdevName = in.NvMeNamespace.Spec.VolumeId.Value

	var result spdk.NvmfSubsystemAddNsResult
	err := s.rpc.Call("nvmf_subsystem_add_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result < 0 {
		msg := fmt.Sprintf("Could not create NS: %s", in.NvMeNamespace.Spec.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Nvme.Namespaces[in.NvMeNamespace.Spec.Name] = in.NvMeNamespace

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
func (s *Server) DeleteNVMeNamespace(_ context.Context, in *pb.DeleteNVMeNamespaceRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeNamespace: Received from client: %v", in)
	namespace, ok := s.Nvme.Namespaces[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := s.Nvme.Subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.NvmfSubsystemRemoveNsParams{
		Nqn:  subsys.Spec.Nqn,
		Nsid: int(namespace.Spec.HostNsid),
	}
	var result spdk.NvmfSubsystemRemoveNsResult
	err := s.rpc.Call("nvmf_subsystem_remove_ns", &params, &result)
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
	delete(s.Nvme.Namespaces, namespace.Spec.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeNamespace updates an NVMe namespace
func (s *Server) UpdateNVMeNamespace(_ context.Context, in *pb.UpdateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("UpdateNVMeNamespace: Received from client: %v", in)
	s.Nvme.Namespaces[in.NvMeNamespace.Spec.Name] = in.NvMeNamespace
	s.Nvme.Namespaces[in.NvMeNamespace.Spec.Name].Status = &pb.NVMeNamespaceStatus{PciState: 2, PciOperState: 1}

	response := &pb.NVMeNamespace{}
	err := deepcopier.Copy(in.NvMeNamespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// ListNVMeNamespaces lists NVMe namespaces
func (s *Server) ListNVMeNamespaces(_ context.Context, in *pb.ListNVMeNamespacesRequest) (*pb.ListNVMeNamespacesResponse, error) {
	log.Printf("ListNVMeNamespaces: Received from client: %v", in)
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
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
	var result []spdk.NvmfGetSubsystemsResult
	err := s.rpc.Call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	token := ""
	Blobarray := []*pb.NVMeNamespace{}
	for i := range result {
		rr := &result[i]
		if rr.Nqn == nqn || nqn == "" {
			log.Printf("Limiting result len(%d) to [%d:%d]", len(result), offset, size)
			hasMoreElements := false
			rr.Namespaces, hasMoreElements = server.LimitPagination(rr.Namespaces, offset, size)
			if hasMoreElements {
				token = uuid.New().String()
				s.Pagination[token] = offset + size
			}
			for j := range rr.Namespaces {
				r := &rr.Namespaces[j]
				Blobarray = append(Blobarray, &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{HostNsid: int32(r.Nsid)}})
			}
		}
	}
	if len(Blobarray) > 0 {
		sortNVMeNamespaces(Blobarray)
		return &pb.ListNVMeNamespacesResponse{NvMeNamespaces: Blobarray, NextPageToken: token}, nil
	}

	msg := fmt.Sprintf("Could not find any namespaces for NQN: %s", nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// GetNVMeNamespace gets an NVMe namespace
func (s *Server) GetNVMeNamespace(_ context.Context, in *pb.GetNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("GetNVMeNamespace: Received from client: %v", in)
	namespace, ok := s.Nvme.Namespaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
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

	var result []spdk.NvmfGetSubsystemsResult
	err := s.rpc.Call("nvmf_get_subsystems", nil, &result)
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
						Spec:   &pb.NVMeNamespaceSpec{Name: namespace.Spec.Name, HostNsid: namespace.Spec.HostNsid},
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
func (s *Server) NVMeNamespaceStats(_ context.Context, in *pb.NVMeNamespaceStatsRequest) (*pb.NVMeNamespaceStatsResponse, error) {
	log.Printf("NVMeNamespaceStats: Received from client: %v", in)
	return &pb.NVMeNamespaceStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}
