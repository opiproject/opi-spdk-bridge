# OPI storage gRPC to SPDK json-rpc bridge

[![Linters](https://github.com/opiproject/opi-spdk-bridge/actions/workflows/linters.yml/badge.svg)](https://github.com/opiproject/opi-spdk-bridge/actions/workflows/linters.yml)
[![CodeQL](https://github.com/opiproject/opi-spdk-bridge/actions/workflows/codeql.yml/badge.svg)](https://github.com/opiproject/opi-spdk-bridge/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/opiproject/opi-spdk-bridge/badge)](https://securityscorecards.dev/viewer/?platform=github.com&org=opiproject&repo=opi-spdk-bridge)
[![tests](https://github.com/opiproject/opi-spdk-bridge/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/opiproject/opi-spdk-bridge/actions/workflows/docker-publish.yml)
[![License](https://img.shields.io/github/license/opiproject/opi-spdk-bridge?style=flat-square&color=blue&label=License)](https://github.com/opiproject/opi-spdk-bridge/blob/master/LICENSE)
[![codecov](https://codecov.io/gh/opiproject/opi-spdk-bridge/branch/main/graph/badge.svg)](https://codecov.io/gh/opiproject/opi-spdk-bridge)
[![Go Report Card](https://goreportcard.com/badge/github.com/opiproject/opi-spdk-bridge)](https://goreportcard.com/report/github.com/opiproject/opi-spdk-bridge)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/opiproject/opi-spdk-bridge)
[![Last Release](https://img.shields.io/github/v/release/opiproject/opi-spdk-bridge?label=Latest&style=flat-square&logo=go)](https://github.com/opiproject/opi-spdk-bridge/releases)
[![GitHub stars](https://img.shields.io/github/stars/opiproject/opi-spdk-bridge.svg?style=flat-square&label=github%20stars)](https://github.com/opiproject/opi-spdk-bridge)
[![GitHub Contributors](https://img.shields.io/github/contributors/opiproject/opi-spdk-bridge.svg?style=flat-square)](https://github.com/opiproject/opi-spdk-bridge/graphs/contributors)

This is a simple SPDK based storage API PoC.

* SPDK - container with SPDK app that is running on xPU
* Server - container with OPI gRPC storage APIs to SPDK json-rpc APIs bridge
* Client - use [goDPU](https://github.com/opiproject/godpu) for testing of the above server/bridge

## I Want To Contribute

This project welcomes contributions and suggestions.  We are happy to have the Community involved via submission of **Issues and Pull Requests** (with substantive content or even just fixes). We are hoping for the documents, test framework, etc. to become a community process with active engagement.  PRs can be reviewed by by any number of people, and a maintainer may accept.

See [CONTRIBUTING](https://github.com/opiproject/opi/blob/main/CONTRIBUTING.md) and [GitHub Basic Process](https://github.com/opiproject/opi/blob/main/doc-github-rules.md) for more details.

## Docs

* [JSON RPC Proxy](https://spdk.io/doc/jsonrpc_proxy.html)
* [SPDK SMA](https://github.com/spdk/spdk/tree/master/python/spdk/sma)
* [SPDK CSI](https://github.com/spdk/spdk-csi/blob/master/deploy/spdk/Dockerfile)
* [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md)

## OPI-SPDK Bridge Block Diagram

The following is the example architecture we envision for the OPI Storage
SPDK bridge APIs. It utilizes SPDK to handle storage services,
and the configuration is handled by standard JSON-RPC based APIs
see <https://spdk.io/doc/jsonrpc.html>

We recongnise, not all companies use SPDK, so for them only PROTOBUF definitions
are going to be the OPI conumable product. For those that wish to use SPDK, this
is a refernce implementation not intended to use in production.

![OPI Storage SPDK bridge/server](doc/OPI-storage-SPDK-bridge.png)

## OPI-SPDK Bridge Sequence Diagram

The following is the example sequence diagram for OPI-SPDK bridge APIs.
It is just an example and implies SPDK just as example, not mandated by OPI.

![OPI Storage SPDK bridge/server](doc/OPI-Storage-Sequence.png)

## Getting started

* [Setup everything once using ansible](https://github.com/opiproject/opi-poc/tree/main/setup)
* Run `docker-compose up -d`

## QEMU example

* [OPI Storage QEMU SPDK Setup](doc/qemu_spdk_setup.md)
* [SPDK vhost-user Target Overview](doc/vhost_user.md)

## Real DPU/IPU example

on DPU/IPU (i.e. with IP=10.10.10.10) run

```bash
$ docker run --rm -it -v /var/tmp/:/var/tmp/ -p 50051:50051 -p 8082:8082 ghcr.io/opiproject/opi-spdk-bridge:main
2023/09/12 20:29:05 Connection to SPDK will be via: unix detected from /var/tmp/spdk.sock
2023/09/12 20:29:05 gRPC server listening at [::]:50051
2023/09/12 20:29:05 HTTP Server listening at 8082
```

on X86 management VM run

```bash
docker run --network=host --rm -it namely/grpc-cli ls   --json_input --json_output 10.10.10.10:50051 -l
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 CreateNvmeSubsystem "{nvme_subsystem : {spec : {nqn: 'nqn.2022-09.io.spdk:opitest2', serial_number: 'myserial2', model_number: 'mymodel2', max_namespaces: 11} }, nvme_subsystem_id : 'subsystem2' }"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 ListNvmeSubsystems "{parent : 'todo'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 GetNvmeSubsystem "{name : '//storage.opiproject.org/volumes/subsystem2'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 CreateNvmeController "{nvme_controller : {spec : {nvme_controller_id: 2, subsystem_name_ref : '//storage.opiproject.org/volumes/subsystem2', pcie_id : {physical_function : 0, virtual_function : 0, port_id: 0}, max_nsq:5, max_ncq:5 } }, nvme_controller_id : 'controller1'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 ListNvmeControllers "{parent : '//storage.opiproject.org/volumes/subsystem2'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 GetNvmeController "{name : '//storage.opiproject.org/volumes/controller1'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 CreateNvmeNamespace "{nvme_namespace : {spec : {subsystem_name_ref : '//storage.opiproject.org/volumes/subsystem2', volume_name_ref : 'Malloc0', 'host_nsid' : '10', uuid:{value : '1b4e28ba-2fa1-11d2-883f-b9a761bde3fb'}, nguid: '1b4e28ba-2fa1-11d2-883f-b9a761bde3fb', eui64: 1967554867335598546 } }, nvme_namespace_id: 'namespace1'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 ListNvmeNamespaces "{parent : '//storage.opiproject.org/volumes/subsystem2'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 GetNvmeNamespace "{name : '//storage.opiproject.org/volumes/namespace1'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 StatsNvmeNamespace "{name : '//storage.opiproject.org/volumes/namespace1'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 CreateNvmeRemoteController "{nvme_remote_controller : {multipath: 'NVME_MULTIPATH_MULTIPATH'}, nvme_remote_controller_id: 'nvmetcp12'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 ListNvmeRemoteControllers "{parent : 'todo'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 GetNvmeRemoteController "{name: '//storage.opiproject.org/volumes/nvmetcp12'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 CreateNvmePath "{nvme_path : {controller_name_ref: '//storage.opiproject.org/volumes/nvmetcp12', traddr:'11.11.11.2', trtype:'NVME_TRANSPORT_TCP', fabrics:{subnqn:'nqn.2016-06.com.opi.spdk.target0', trsvcid:'4444', adrfam:'NVME_ADRFAM_IPV4', hostnqn:'nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c'}}, nvme_path_id: 'nvmetcp12path0'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 CreateNvmeRemoteController "{nvme_remote_controller : {multipath: 'NVME_MULTIPATH_DISABLE'}, nvme_remote_controller_id: 'nvmepcie13'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 CreateNvmePath "{nvme_path : {controller_name_ref: '//storage.opiproject.org/volumes/nvmepcie13', traddr:'0000:01:00.0', trtype:'NVME_TRANSPORT_PCIE'}, nvme_path_id: 'nvmepcie13path0'}"

docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 ListNvmePaths "{parent : 'todo'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 DeleteNvmePath "{name: '//storage.opiproject.org/volumes/nvmepcie13path0'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 DeleteNvmeRemoteController "{name: '//storage.opiproject.org/volumes/nvmepcie13'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 GetNvmePath "{name: '//storage.opiproject.org/volumes/nvmetcp12path0'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 DeleteNvmePath "{name: '//storage.opiproject.org/volumes/nvmetcp12path0'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 DeleteNvmeRemoteController "{name: '//storage.opiproject.org/volumes/nvmetcp12'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 DeleteNvmeNamespace "{name : '//storage.opiproject.org/volumes/namespace1'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 DeleteNvmeController "{name : '//storage.opiproject.org/volumes/controller1'}"
docker run --network=host --rm -it namely/grpc-cli call --json_input --json_output 10.10.10.10:50051 DeleteNvmeSubsystem "{name : '//storage.opiproject.org/volumes/subsystem2'}"
```

## Test SPDK is up

```bash
curl -k --user spdkuser:spdkpass -X POST -H "Content-Type: application/json" -d '{"id": 1, "method": "bdev_get_bdevs", "params": {"name": "Malloc0"}}' http://127.0.0.1:9009/
```

## gRPC CLI examples

From <https://github.com/grpc/grpc-go/blob/master/Documentation/server-reflection-tutorial.md>

Alias

```bash
alias grpc_cli='docker run --network=opi-spdk-bridge_opi --rm -it namely/grpc-cli'
```

See services

```bash
$ grpc_cli ls opi-spdk-server:50051
grpc.reflection.v1alpha.ServerReflection
opi_api.storage.v1.AioVolumeService
opi_api.storage.v1.FrontendNvmeService
opi_api.storage.v1.FrontendVirtioBlkService
opi_api.storage.v1.FrontendVirtioScsiService
opi_api.storage.v1.MiddleendEncryptionService
opi_api.storage.v1.MiddleendQosVolumeService
opi_api.storage.v1.NvmeRemoteControllerService
opi_api.storage.v1.NullVolumeService
```

See commands

```bash
$ grpc_cli ls opi-spdk-server:50051 opi_api.storage.v1.FrontendNvmeService -l
filename: frontend_nvme_pcie.proto
package: opi_api.storage.v1;
service FrontendNvmeService {
  rpc CreateNvmeSubsystem(opi_api.storage.v1.CreateNvmeSubsystemRequest) returns (opi_api.storage.v1.NvmeSubsystem) {}
  rpc DeleteNvmeSubsystem(opi_api.storage.v1.DeleteNvmeSubsystemRequest) returns (google.protobuf.Empty) {}
  rpc UpdateNvmeSubsystem(opi_api.storage.v1.UpdateNvmeSubsystemRequest) returns (opi_api.storage.v1.NvmeSubsystem) {}
  rpc ListNvmeSubsystem(opi_api.storage.v1.ListNvmeSubsystemRequest) returns (opi_api.storage.v1.ListNvmeSubsystemResponse) {}
  rpc GetNvmeSubsystem(opi_api.storage.v1.GetNvmeSubsystemRequest) returns (opi_api.storage.v1.NvmeSubsystem) {}
  rpc StatsNvmeSubsystem(opi_api.storage.v1.StatsNvmeSubsystemRequest) returns (opi_api.storage.v1.StatsNvmeSubsystemResponse) {}
  rpc CreateNvmeController(opi_api.storage.v1.CreateNvmeControllerRequest) returns (opi_api.storage.v1.NvmeController) {}
  rpc DeleteNvmeController(opi_api.storage.v1.DeleteNvmeControllerRequest) returns (google.protobuf.Empty) {}
  rpc UpdateNvmeController(opi_api.storage.v1.UpdateNvmeControllerRequest) returns (opi_api.storage.v1.NvmeController) {}
  rpc ListNvmeController(opi_api.storage.v1.ListNvmeControllerRequest) returns (opi_api.storage.v1.ListNvmeControllerResponse) {}
  rpc GetNvmeController(opi_api.storage.v1.GetNvmeControllerRequest) returns (opi_api.storage.v1.NvmeController) {}
  rpc StatsNvmeController(opi_api.storage.v1.StatsNvmeControllerRequest) returns (opi_api.storage.v1.StatsNvmeControllerResponse) {}
  rpc CreateNvmeNamespace(opi_api.storage.v1.CreateNvmeNamespaceRequest) returns (opi_api.storage.v1.NvmeNamespace) {}
  rpc DeleteNvmeNamespace(opi_api.storage.v1.DeleteNvmeNamespaceRequest) returns (google.protobuf.Empty) {}
  rpc UpdateNvmeNamespace(opi_api.storage.v1.UpdateNvmeNamespaceRequest) returns (opi_api.storage.v1.NvmeNamespace) {}
  rpc ListNvmeNamespace(opi_api.storage.v1.ListNvmeNamespaceRequest) returns (opi_api.storage.v1.ListNvmeNamespaceResponse) {}
  rpc GetNvmeNamespace(opi_api.storage.v1.GetNvmeNamespaceRequest) returns (opi_api.storage.v1.NvmeNamespace) {}
  rpc StatsNvmeNamespace(opi_api.storage.v1.StatsNvmeNamespaceRequest) returns (opi_api.storage.v1.StatsNvmeNamespaceResponse) {}
}
```

See methods

```bash
$ grpc_cli ls opi-spdk-server:50051 opi_api.storage.v1.FrontendNvmeService.CreateNvmeController -l
  rpc CreateNvmeController(opi_api.storage.v1.CreateNvmeControllerRequest) returns (opi_api.storage.v1.NvmeController) {}
```

See messages

```bash
$ grpc_cli type opi-spdk-server:50051 opi_api.storage.v1.NvmeControllerSpec
message NvmeControllerSpec {
  .opi_api.common.v1.ObjectKey id = 1 [json_name = "id"];
  int32 nvme_controller_id = 2 [json_name = "nvmeControllerId"];
  .opi_api.common.v1.ObjectKey subsystem_id = 3 [json_name = "subsystemId"];
  .opi_api.storage.v1.PciEndpoint pcie_id = 4 [json_name = "pcieId"];
  int32 max_nsq = 5 [json_name = "maxNsq"];
  int32 max_ncq = 6 [json_name = "maxNcq"];
  int32 sqes = 7 [json_name = "sqes"];
  int32 cqes = 8 [json_name = "cqes"];
  int32 max_namespaces = 9 [json_name = "maxNamespaces"];
}

$ grpc_cli type opi-spdk-server:50051 opi_api.storage.v1.PciEndpoint
message PciEndpoint {
  int32 port_id = 1 [json_name = "portId"];
  int32 physical_function = 2 [json_name = "physicalFunction"];
  int32 virtual_function = 3 [json_name = "virtualFunction"];
}
```

Call remote method

```bash
$ grpc_cli call --json_input --json_output opi-spdk-server:50051 DeleteNvmeController "{subsystem_id: 8}"
connecting to opi-spdk-server:50051
{}
Rpc succeeded with OK status
```

Server log

```bash
opi-spdk-server_1  | 2022/08/05 14:31:14 server listening at [::]:50051
opi-spdk-server_1  | 2022/08/05 14:39:40 DeleteNvmeSubsystem: Received from client: id:8
opi-spdk-server_1  | 2022/08/05 14:39:40 Sending to SPDK: {"jsonrpc":"2.0","id":1,"method":"bdev_malloc_delete","params":{"name":"OpiMalloc8"}}
opi-spdk-server_1  | 2022/08/05 14:39:40 Received from SPDK: {1 {-19 No such device} 0xc000029f4e}
opi-spdk-server_1  | 2022/08/05 14:39:40 error: bdev_malloc_delete: json response error: No such device
opi-spdk-server_1  | 2022/08/05 14:39:40 Received from SPDK: false
opi-spdk-server_1  | 2022/08/05 14:39:40 Could not delete: id:8
```

In addition HTTP is supported via grpc gateway, for example:

```bash
curl -kL http://10.10.10.10:8082/v1/inventory/1/inventory/2
```

Another remote call example

```bash
$ grpc_cli call --json_input --json_output opi-spdk-server:50051 ListNvmeSubsystem {}
connecting to opi-spdk-server:50051
{
 "subsystem": [
  {
   "nqn": "nqn.2014-08.org.nvmexpress.discovery"
  },
  {
   "nqn": "nqn.2016-06.io.spdk:cnode1"
  }
 ]
}
Rpc succeeded with OK status
```

Another Server log

```bash
2022/09/21 19:38:26 ListNvmeSubsystem: Received from client:
2022/09/21 19:38:26 Sending to SPDK: {"jsonrpc":"2.0","id":1,"method":"bdev_get_bdevs"}
2022/09/21 19:38:26 Received from SPDK: {1 {0 } 0x40003de660}
2022/09/21 19:38:26 Received from SPDK: [{Malloc0 512 131072 08cd0d67-eb57-41c2-957b-585faed7d81a} {Malloc1 512 131072 78c4b40f-dd16-42c1-b057-f95c11db7aaf}]
```
