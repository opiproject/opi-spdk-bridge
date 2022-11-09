// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

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

// ////////////////////////////////////////////////////////
var subsystems = map[string]*pb.NVMeSubsystem{}

func (s *server) NVMeSubsystemCreate(ctx context.Context, in *pb.NVMeSubsystemCreateRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("NVMeSubsystemCreate: Received from client: %v", in)
	params := NvmfCreateSubsystemParams{
		Nqn:          in.Subsystem.Spec.Nqn,
		SerialNumber: "SPDK0",
		AllowAnyHost: true,
	}
	var result NvmfCreateSubsystemResult
	err := call("nvmf_create_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	subsystems[in.Subsystem.Spec.Id.Value] = in.Subsystem
	log.Printf("Received from SPDK: %v", result)
	response := &pb.NVMeSubsystem{}
	err = deepcopier.Copy(in.Subsystem).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeSubsystemDelete(ctx context.Context, in *pb.NVMeSubsystemDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("NVMeSubsystemDelete: Received from client: %v", in)
	subsys, ok := subsystems[in.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.SubsystemId)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := NvmfDeleteSubsystemParams{
		Nqn: subsys.Spec.Nqn,
	}
	var result NvmfDeleteSubsystemResult
	err := call("nvmf_delete_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	delete(subsystems, subsys.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) NVMeSubsystemUpdate(ctx context.Context, in *pb.NVMeSubsystemUpdateRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("NVMeSubsystemUpdate: Received from client: %v", in)
	subsystems[in.Subsystem.Spec.Id.Value] = in.Subsystem
	response := &pb.NVMeSubsystem{}
	err := deepcopier.Copy(in.Subsystem).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeSubsystemList(ctx context.Context, in *pb.NVMeSubsystemListRequest) (*pb.NVMeSubsystemListResponse, error) {
	log.Printf("NVMeSubsystemList: Received from client: %v", in)
	var result []NvmfGetSubsystemsResult
	err := call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.NVMeSubsystem, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NVMeSubsystem{Spec : &pb.NVMeSubsystemSpec{Nqn: r.Nqn}}
	}
	return &pb.NVMeSubsystemListResponse{Subsystem: Blobarray}, nil
}

func (s *server) NVMeSubsystemGet(ctx context.Context, in *pb.NVMeSubsystemGetRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("NVMeSubsystemGet: Received from client: %v", in)
	subsys, ok := subsystems[in.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	var result []NvmfGetSubsystemsResult
	err := call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	for i := range result {
		r := &result[i]
		if r.Nqn == subsys.Spec.Nqn {
			return &pb.NVMeSubsystem{ Spec: &pb.NVMeSubsystemSpec{Nqn: r.Nqn}}, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Spec.Nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

func (s *server) NVMeSubsystemStats(ctx context.Context, in *pb.NVMeSubsystemStatsRequest) (*pb.NVMeSubsystemStatsResponse, error) {
	log.Printf("NVMeSubsystemStats: Received from client: %v", in)
	var result NvmfGetSubsystemStatsResult
	err := call("nvmf_get_stats", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NVMeSubsystemStatsResponse{Stats: fmt.Sprint(result.TickRate)}, nil
}

// ////////////////////////////////////////////////////////
var controllers = map[string]*pb.NVMeController{}

func (s *server) NVMeControllerCreate(ctx context.Context, in *pb.NVMeControllerCreateRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.Controller)
	controllers[in.Controller.Spec.Id.Value] = in.Controller
	response := &pb.NVMeController{}
	err := deepcopier.Copy(in.Controller).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeControllerDelete(ctx context.Context, in *pb.NVMeControllerDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("Received from client: %v", in.ControllerId)
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	delete(controllers, controller.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) NVMeControllerUpdate(ctx context.Context, in *pb.NVMeControllerUpdateRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.Controller)
	controllers[in.Controller.Spec.Id.Value] = in.Controller
	response := &pb.NVMeController{}
	err := deepcopier.Copy(in.Controller).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeControllerList(ctx context.Context, in *pb.NVMeControllerListRequest) (*pb.NVMeControllerListResponse, error) {
	log.Printf("Received from client: %v", in.SubsystemId)
	Blobarray := []*pb.NVMeController{}
	for _, controller := range controllers {
		Blobarray = append(Blobarray, controller)
	}
	return &pb.NVMeControllerListResponse{Controller: Blobarray}, nil
}

func (s *server) NVMeControllerGet(ctx context.Context, in *pb.NVMeControllerGetRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.ControllerId)
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	return &pb.NVMeController{Spec: &pb.NVMeControllerSpec {Id: in.ControllerId, NvmeControllerId: controller.Spec.NvmeControllerId}}, nil
}

func (s *server) NVMeControllerStats(ctx context.Context, in *pb.NVMeControllerStatsRequest) (*pb.NVMeControllerStatsResponse, error) {
	log.Printf("Received from client: %v", in.Id)
	return &pb.NVMeControllerStatsResponse{}, nil
}

// ////////////////////////////////////////////////////////
var namespaces = map[string]*pb.NVMeNamespace{}

func (s *server) NVMeNamespaceCreate(ctx context.Context, in *pb.NVMeNamespaceCreateRequest) (*pb.NVMeNamespace, error) {
	log.Printf("NVMeNamespaceCreate: Received from client: %v", in)
	subsys, ok := subsystems[in.Namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", in.Namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		// TODO: temp workaround
		subsys = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: in.Namespace.Spec.SubsystemId.Value}}
		// return nil, err
	}

	params := NvmfSubsystemAddNsParams{
		Nqn: subsys.Spec.Nqn,
	}

	// TODO: using bdev for volume id as a middle end handle for now
	params.Namespace.BdevName = in.Namespace.Spec.VolumeId.Value

	var result NvmfSubsystemAddNsResult
	err := call("nvmf_subsystem_add_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	namespaces[in.Namespace.Spec.Id.Value] = in.Namespace

	response := &pb.NVMeNamespace{}
	err = deepcopier.Copy(in.Namespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeNamespaceDelete(ctx context.Context, in *pb.NVMeNamespaceDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("NVMeNamespaceDelete: Received from client: %v", in)
	namespace, ok := namespaces[in.NamespaceId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NamespaceId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		// TODO: temp workaround
		subsys = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec { Nqn: namespace.Spec.SubsystemId.Value}}
		// return nil, err
	}

	params := NvmfSubsystemRemoveNsParams{
		Nqn:  subsys.Spec.Nqn,
		Nsid: int(namespace.Spec.HostNsid),
	}
	var result NvmfSubsystemRemoveNsResult
	err := call("nvmf_subsystem_remove_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	delete(namespaces, namespace.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) NVMeNamespaceUpdate(ctx context.Context, in *pb.NVMeNamespaceUpdateRequest) (*pb.NVMeNamespace, error) {
	log.Printf("Received from client: %v", in.Namespace)
	namespaces[in.Namespace.Spec.Id.Value] = in.Namespace
	response := &pb.NVMeNamespace{}
	err := deepcopier.Copy(in.Namespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeNamespaceList(ctx context.Context, in *pb.NVMeNamespaceListRequest) (*pb.NVMeNamespaceListResponse, error) {
	log.Printf("NVMeNamespaceList: Received from client: %v", in)

	nqn := ""
	if in.SubsystemId != nil {
		subsys, ok := subsystems[in.SubsystemId.Value]
		if !ok {
			err := fmt.Errorf("unable to find subsystem %s", in.SubsystemId.Value)
			log.Printf("error: %v", err)
			return nil, err
		}
		nqn = subsys.Spec.Nqn
	}
	var result []NvmfGetSubsystemsResult
	err := call("nvmf_get_subsystems", nil, &result)
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
				Blobarray = append(Blobarray, &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec {HostNsid: int32(r.Nsid)}})
			}
		}
	}
	if len(Blobarray) > 0 {
		return &pb.NVMeNamespaceListResponse{Namespace: Blobarray}, nil
	}

	msg := fmt.Sprintf("Could not find any namespaces for NQN: %s", nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

func (s *server) NVMeNamespaceGet(ctx context.Context, in *pb.NVMeNamespaceGetRequest) (*pb.NVMeNamespace, error) {
	log.Printf("NVMeNamespaceGet: Received from client: %v", in)
	namespace, ok := namespaces[in.NamespaceId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NamespaceId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: do we even query SPDK to confirm if namespace is present?
	// return namespace, nil

	// fetch subsystems -> namespaces from server, match the nsid to find the corresponding namespace
	subsys, ok := subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		// TODO: temp workaround
		subsys = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec {Nqn: namespace.Spec.SubsystemId.Value}}
		// return nil, err
	}

	var result []NvmfGetSubsystemsResult
	err := call("nvmf_get_subsystems", nil, &result)
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
					return &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{
						Id: namespace.Spec.Id, HostNsid: namespace.Spec.HostNsid}}, nil
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

func (s *server) NVMeNamespaceStats(ctx context.Context, in *pb.NVMeNamespaceStatsRequest) (*pb.NVMeNamespaceStatsResponse, error) {
	log.Printf("Received from client: %v", in.NamespaceId)
	return &pb.NVMeNamespaceStatsResponse{}, nil
}

//////////////////////////////////////////////////////////

func (s *server) VirtioBlkCreate(ctx context.Context, in *pb.VirtioBlkCreateRequest) (*pb.VirtioBlk, error) {
	log.Printf("VirtioBlkCreate: Received from client: %v", in)
	params := VhostCreateBlkControllerParams{
		Ctrlr:   in.GetController().GetId().GetValue(),
		DevName: in.GetController().GetBdev(),
	}
	var result VhostCreateBlkControllerResult
	err := call("vhost_create_blk_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
	}
	return &pb.VirtioBlk{}, nil
}

func (s *server) VirtioBlkDelete(ctx context.Context, in *pb.VirtioBlkDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("VirtioBlkDelete: Received from client: %v", in)
	params := VhostDeleteControllerParams{
		Ctrlr: in.GetControllerId().GetValue(),
	}
	var result VhostDeleteControllerResult
	err := call("vhost_delete_controller", &params, &result)
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

func (s *server) VirtioBlkUpdate(ctx context.Context, in *pb.VirtioBlkUpdateRequest) (*pb.VirtioBlk, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioBlk{}, nil
}

func (s *server) VirtioBlkList(ctx context.Context, in *pb.VirtioBlkListRequest) (*pb.VirtioBlkListResponse, error) {
	log.Printf("VirtioBlkList: Received from client: %v", in)
	var result []VhostGetControllersResult
	err := call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.VirtioBlk, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioBlk{Id: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.VirtioBlkListResponse{Controller: Blobarray}, nil
}

func (s *server) VirtioBlkGet(ctx context.Context, in *pb.VirtioBlkGetRequest) (*pb.VirtioBlk, error) {
	log.Printf("VirtioBlkGet: Received from client: %v", in)
	params := VhostGetControllersParams{
		Name: in.GetControllerId().GetValue(),
	}
	var result []VhostGetControllersResult
	err := call("vhost_get_controllers", &params, &result)
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
	return &pb.VirtioBlk{Id: &pc.ObjectKey{Value: result[0].Ctrlr}}, nil
}

func (s *server) VirtioBlkStats(ctx context.Context, in *pb.VirtioBlkStatsRequest) (*pb.VirtioBlkStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioBlkStatsResponse{}, nil
}

//////////////////////////////////////////////////////////

func (s *server) VirtioScsiControllerCreate(ctx context.Context, in *pb.VirtioScsiControllerCreateRequest) (*pb.VirtioScsiController, error) {
	log.Printf("VirtioScsiControllerCreate: Received from client: %v", in)
	params := VhostCreateScsiControllerParams{
		Ctrlr: in.GetController().GetId().GetValue(),
	}
	var result VhostCreateScsiControllerResult
	err := call("vhost_create_scsi_controller", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not create: %v", in)
	}
	return &pb.VirtioScsiController{}, nil
}

func (s *server) VirtioScsiControllerDelete(ctx context.Context, in *pb.VirtioScsiControllerDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("VirtioScsiControllerDelete: Received from client: %v", in)
	params := VhostDeleteControllerParams{
		Ctrlr: in.GetControllerId().GetValue(),
	}
	var result VhostDeleteControllerResult
	err := call("vhost_delete_controller", &params, &result)
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

func (s *server) VirtioScsiControllerUpdate(ctx context.Context, in *pb.VirtioScsiControllerUpdateRequest) (*pb.VirtioScsiController, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiController{}, nil
}

func (s *server) VirtioScsiControllerList(ctx context.Context, in *pb.VirtioScsiControllerListRequest) (*pb.VirtioScsiControllerListResponse, error) {
	log.Printf("VirtioScsiControllerList: Received from client: %v", in)
	var result []VhostGetControllersResult
	err := call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.VirtioScsiController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiController{Id: &pc.ObjectKey{Value: r.Ctrlr}}
	}
	return &pb.VirtioScsiControllerListResponse{Controller: Blobarray}, nil
}

func (s *server) VirtioScsiControllerGet(ctx context.Context, in *pb.VirtioScsiControllerGetRequest) (*pb.VirtioScsiController, error) {
	log.Printf("VirtioScsiControllerGet: Received from client: %v", in)
	params := VhostGetControllersParams{
		Name: in.GetControllerId().GetValue(),
	}
	var result []VhostGetControllersResult
	err := call("vhost_get_controllers", &params, &result)
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
	return &pb.VirtioScsiController{Id: &pc.ObjectKey{Value: result[0].Ctrlr}}, nil
}

func (s *server) VirtioScsiControllerStats(ctx context.Context, in *pb.VirtioScsiControllerStatsRequest) (*pb.VirtioScsiControllerStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiControllerStatsResponse{}, nil
}

//////////////////////////////////////////////////////////

func (s *server) VirtioScsiLunCreate(ctx context.Context, in *pb.VirtioScsiLunCreateRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("VirtioScsiLunCreate: Received from client: %v", in)
	params := struct {
		Name string `json:"ctrlr"`
		Num  int    `json:"scsi_target_num"`
		Bdev string `json:"bdev_name"`
	}{
		Name: in.GetLun().GetControllerId().GetValue(),
		Num:  5,
		Bdev: in.GetLun().GetBdev(),
	}
	var result int
	err := call("vhost_scsi_controller_add_target", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.VirtioScsiLun{}, nil
}

func (s *server) VirtioScsiLunDelete(ctx context.Context, in *pb.VirtioScsiLunDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("VirtioScsiLunDelete: Received from client: %v", in)
	params := struct {
		Name string `json:"ctrlr"`
		Num  int    `json:"scsi_target_num"`
	}{
		Name: in.GetControllerId().GetValue(),
		Num:  5,
	}
	var result bool
	err := call("vhost_scsi_controller_remove_target", &params, &result)
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

func (s *server) VirtioScsiLunUpdate(ctx context.Context, in *pb.VirtioScsiLunUpdateRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiLun{}, nil
}

func (s *server) VirtioScsiLunList(ctx context.Context, in *pb.VirtioScsiLunListRequest) (*pb.VirtioScsiLunListResponse, error) {
	log.Printf("VirtioScsiLunList: Received from client: %v", in)
	var result []VhostGetControllersResult
	err := call("vhost_get_controllers", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.VirtioScsiLun, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.VirtioScsiLun{Bdev: r.Ctrlr}
	}
	return &pb.VirtioScsiLunListResponse{Lun: Blobarray}, nil
}

func (s *server) VirtioScsiLunGet(ctx context.Context, in *pb.VirtioScsiLunGetRequest) (*pb.VirtioScsiLun, error) {
	log.Printf("VirtioScsiLunGet: Received from client: %v", in)
	params := VhostGetControllersParams{
		Name: in.GetControllerId().GetValue(),
	}
	var result []VhostGetControllersResult
	err := call("vhost_get_controllers", &params, &result)
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
	return &pb.VirtioScsiLun{Bdev: result[0].Ctrlr}, nil
}

func (s *server) VirtioScsiLunStats(ctx context.Context, in *pb.VirtioScsiLunStatsRequest) (*pb.VirtioScsiLunStatsResponse, error) {
	log.Printf("Received from client: %v", in)
	return &pb.VirtioScsiLunStatsResponse{}, nil
}
