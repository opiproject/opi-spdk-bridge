#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
# Copyright (c) 2022 Dell Inc, or its subsidiaries.

set -euxo pipefail

# docker compose plugin
command -v docker-compose || { shopt -s expand_aliases && alias docker-compose='docker compose'; }

docker-compose ps -a
for i in $(seq 1 20)
do
    echo "$i"
    if [[ "$(curl --fail --insecure --user spdkuser:spdkpass -X POST -H 'Content-Type: application/json' -d '{"id": 1, "method": "spdk_get_version"}' http://127.0.0.1:9009)" ]]
    then
        break
    else
        sleep 1
    fi
done
curl --fail --insecure --user spdkuser:spdkpass -X POST -H 'Content-Type: application/json' -d '{"id": 1, "method": "bdev_get_bdevs"}' http://127.0.0.1:9009

# wait for client completes and return exit code
STORAGE_CLIENT_NAME=$(docker-compose ps | grep opi-spdk-client | awk '{print $1}')
STORAGE_CLIENT_RC=$(docker wait "${STORAGE_CLIENT_NAME}")
if [ "${STORAGE_CLIENT_RC}" != "0" ]; then
    echo "opi-spdk-client failed:"
    docker logs "${STORAGE_CLIENT_NAME}"
    exit 1
fi

# Check exported port also works (host network)
docker run --network=host --rm docker.io/namely/grpc-cli ls 127.0.0.1:50051
docker run --network=host --rm docker.io/curlimages/curl:8.3.0 curl -qkL http://127.0.0.1:8082/v1/inventory/1/inventory/2

# Check Jaeger tracing works
curl -s "http://127.0.0.1:16686/api/traces?service=opi-spdk-bridge&lookback=20m&prettyPrint=true&limit=10" | jq .data[].spans[].operationName

# check reflection
grpc_cli=(docker run --network=opi-spdk-bridge_opi --rm docker.io/namely/grpc-cli)
"${grpc_cli[@]}" ls opi-spdk-server:50051
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.AioVolumeService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.FrontendNvmeService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.FrontendVirtioBlkService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.FrontendVirtioScsiService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.MiddleendEncryptionService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.MiddleendQosVolumeService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.NvmeRemoteControllerService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.NullVolumeService -l

# check spdk sanity
docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/spdk:main spdk_nvme_perf     -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:4444 subnqn:nqn.2016-06.io.spdk:cnode1 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c' -c 0x1 -q 1 -o 4096 -w randread -t 10 | tee log.txt
grep "Total" log.txt
echo -n NVMeTLSkey-1:01:MDAxMTIyMzM0NDU1NjY3Nzg4OTlhYWJiY2NkZGVlZmZwJEiQ: > /tmp/opikey.txt
chmod 0600 /tmp/opikey.txt
docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages -v /tmp/opikey.txt:/tmp/opikey.txt ghcr.io/opiproject/spdk:main spdk_nvme_perf     -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:5555 subnqn:nqn.2016-06.io.spdk:cnode1 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c' -c 0x1 -q 1 -o 4096 -w randread -t 10 -S ssl --psk-path /tmp/opikey.txt | tee log.txt
grep "Total" log.txt

# get spdk IP
SPDK_NAME=$(docker-compose ps spdk | awk '/spdk/{print $1}')
SPDK_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${SPDK_NAME}")

# check sanity with real IP
docker run --rm --network=opi-spdk-bridge_opi --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/spdk:main spdk_nvme_perf -r "traddr:${SPDK_IP} trtype:TCP adrfam:IPv4 trsvcid:4444 subnqn:nqn.2016-06.io.spdk:cnode1 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c" -c 0x1 -q 1 -o 4096 -w randread -t 10 | tee log.txt
grep "Total" log.txt

# test nvme
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeSubsystem  "{nvme_subsystem_id:  'subsystem1',  nvme_subsystem  : {spec : {nqn: 'nqn.2022-09.io.spdk:opitest1', serial_number: 'myserial1', model_number: 'mymodel1', max_namespaces: 11} } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeController "{nvme_controller_id: 'controller1', parent: 'nvmeSubsystems/subsystem1', nvme_controller : {spec : {nvme_controller_id: 2, 'fabrics_id':{'traddr': '${SPDK_IP}', trsvcid: '7777', adrfam: 'NVME_ADDRESS_FAMILY_IPV4'}, max_nsq:5, max_ncq:5, 'trtype': 'NVME_TRANSPORT_TYPE_TCP' } } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeNamespace  "{nvme_namespace_id:  'namespace1',  parent: 'nvmeSubsystems/subsystem1', nvme_namespace  : {spec : {volume_name_ref : 'Malloc1', host_nsid : 1 } } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeSubsystem "{name : 'nvmeSubsystems/subsystem1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeController "{name : 'nvmeSubsystems/subsystem1/nvmeControllers/controller1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeNamespace "{name :  'nvmeSubsystems/subsystem1/nvmeNamespaces/namespace1'}"
docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/spdk:main spdk_nvme_identify -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:7777 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c'
docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/spdk:main spdk_nvme_perf     -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:7777 subnqn:nqn.2022-09.io.spdk:opitest1 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c' -c 0x1 -q 1 -o 4096 -w randread -t 10 | tee log.txt
grep "Total" log.txt
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeRemoteController "{nvme_remote_controller : {multipath: 'NVME_MULTIPATH_MULTIPATH', tcp: {hdgst: false, ddgst: false}}, nvme_remote_controller_id: 'nvmetcp12'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmePath "{parent: 'nvmeRemoteControllers/nvmetcp12', nvme_path : {traddr:\"$SPDK_IP\", trtype:'NVME_TRANSPORT_TYPE_TCP', fabrics: { subnqn:'nqn.2022-09.io.spdk:opitest1', trsvcid:'7777', adrfam:'NVME_ADDRESS_FAMILY_IPV4', hostnqn:'nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c'}}, nvme_path_id: 'nvmetcp12path0'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeRemoteController "{name: 'nvmeRemoteControllers/nvmetcp12'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmePath "{name: 'nvmeRemoteControllers/nvmetcp12/nvmePaths/nvmetcp12path0'}"

"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmePath "{name: 'nvmeRemoteControllers/nvmetcp12/nvmePaths/nvmetcp12path0'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeRemoteController "{name: 'nvmeRemoteControllers/nvmetcp12'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeNamespace "{name : 'nvmeSubsystems/subsystem1/nvmeNamespaces/namespace1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeController "{name : 'nvmeSubsystems/subsystem1/nvmeControllers/controller1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeSubsystem "{name : 'nvmeSubsystems/subsystem1'}"

# test nvme with TLS
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeSubsystem  "{nvme_subsystem_id:  'subsystem2',  nvme_subsystem  : {spec : {nqn: 'nqn.2022-09.io.spdk:opitest2', serial_number: 'myserial2', model_number: 'mymodel2', max_namespaces: 22, hostnqn: 'nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c', psk: 'TlZNZVRMU2tleS0xOjAxOk1EQXhNVEl5TXpNME5EVTFOalkzTnpnNE9UbGhZV0ppWTJOa1pHVmxabVp3SkVpUTo='} } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeController "{nvme_controller_id: 'controller2', parent: 'nvmeSubsystems/subsystem2', nvme_controller : {spec : {nvme_controller_id: 22, 'fabrics_id':{'traddr': '${SPDK_IP}', trsvcid: '8888', adrfam: 'NVME_ADDRESS_FAMILY_IPV4'}, max_nsq:5, max_ncq:5, 'trtype': 'NVME_TRANSPORT_TYPE_TCP' } } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeNamespace  "{nvme_namespace_id:  'namespace2',  parent: 'nvmeSubsystems/subsystem2', nvme_namespace  : {spec : {volume_name_ref : 'Malloc1', host_nsid : 1 } } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeSubsystem "{name : 'nvmeSubsystems/subsystem2'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeController "{name : 'nvmeSubsystems/subsystem2/nvmeControllers/controller2'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeNamespace "{name :  'nvmeSubsystems/subsystem2/nvmeNamespaces/namespace2'}"
# docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/spdk:main spdk_nvme_identify -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:8888 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c'
docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages -v /tmp/opikey.txt:/tmp/opikey.txt ghcr.io/opiproject/spdk:main spdk_nvme_perf     -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:8888 subnqn:nqn.2022-09.io.spdk:opitest2 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c' -c 0x1 -q 1 -o 4096 -w randread -t 10 -S ssl --psk-path /tmp/opikey.txt | tee log.txt
grep "Total" log.txt
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmeRemoteController "{nvme_remote_controller : {multipath: 'NVME_MULTIPATH_MULTIPATH', tcp: {hdgst: false, ddgst: false, psk: 'TlZNZVRMU2tleS0xOjAxOk1EQXhNVEl5TXpNME5EVTFOalkzTnpnNE9UbGhZV0ppWTJOa1pHVmxabVp3SkVpUTo='}}, nvme_remote_controller_id: 'nvmetls17'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNvmePath "{parent: 'nvmeRemoteControllers/nvmetls17', nvme_path : {traddr:\"$SPDK_IP\", trtype:'NVME_TRANSPORT_TYPE_TCP', fabrics: { subnqn:'nqn.2022-09.io.spdk:opitest2', trsvcid:'8888', adrfam:'NVME_ADDRESS_FAMILY_IPV4', hostnqn:'nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c'}}, nvme_path_id: 'nvmetls17path0'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmeRemoteController "{name: 'nvmeRemoteControllers/nvmetls17'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNvmePath "{name: 'nvmeRemoteControllers/nvmetls17/nvmePaths/nvmetls17path0'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmePath "{name: 'nvmeRemoteControllers/nvmetls17/nvmePaths/nvmetls17path0'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeRemoteController "{name: 'nvmeRemoteControllers/nvmetls17'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeNamespace "{name : 'nvmeSubsystems/subsystem2/nvmeNamespaces/namespace2'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeController "{name : 'nvmeSubsystems/subsystem2/nvmeControllers/controller2'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNvmeSubsystem "{name : 'nvmeSubsystems/subsystem2'}"



# HTTP gateway to Nvme
# Backend
# create
curl -X POST -f http://127.0.0.1:8082/v1/nvmeRemoteControllers?nvme_remote_controller_id=nvmetcp12 -d '{"multipath": "NVME_MULTIPATH_MULTIPATH"}'
curl -X POST -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12/nvmePaths?nvme_path_id=nvmetcp12path0 -d "{\"traddr\":\"${SPDK_IP}\", \"trtype\":\"NVME_TRANSPORT_TYPE_TCP\", \"fabrics\":{\"subnqn\":\"nqn.2016-06.io.spdk:cnode1\", \"trsvcid\":\"4444\", \"adrfam\":\"NVME_ADDRESS_FAMILY_IPV4\", \"hostnqn\":\"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c\"}}"

# get
curl -X GET -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12
curl -X GET -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12/nvmePaths/nvmetcp12path0
# list
curl -X GET -f http://127.0.0.1:8082/v1/nvmeRemoteControllers | jq '.nvmeRemoteControllers[0].name'
curl -X GET -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12/nvmePaths | jq .nvmePaths[0].name
# stats
curl -X GET -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12:stats
curl -X GET -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12/nvmePaths/nvmetcp12path0:stats
# update
curl -X PATCH -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12 -d '{"multipath": "NVME_MULTIPATH_MULTIPATH"}'
curl -X PATCH -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12/nvmePaths/nvmetcp12path0 -d "{\"traddr\":\"${SPDK_IP}\", \"trtype\":\"NVME_TRANSPORT_TYPE_TCP\", \"fabrics\":{\"subnqn\":\"nqn.2016-06.io.spdk:cnode1\", \"trsvcid\":\"4444\", \"adrfam\":\"NVME_ADDRESS_FAMILY_IPV4\", \"hostnqn\":\"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c\"}}"
# delete
curl -X DELETE -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12/nvmePaths/nvmetcp12path0
curl -X DELETE -f http://127.0.0.1:8082/v1/nvmeRemoteControllers/nvmetcp12

# Frontend
# create
curl -X POST -f http://127.0.0.1:8082/v1/nvmeSubsystems?nvme_subsystem_id=subsys0 -d '{"spec": {"nqn": "nqn.2022-09.io.spdk:opitest1"}}'
curl -X POST -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeNamespaces?nvme_namespace_id=namespace0 -d '{"spec": {"volume_name_ref": "Malloc1", "host_nsid": 10}}'
curl -X POST -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeControllers?nvme_controller_id=ctrl0 -d '{"spec": {"trtype": "NVME_TRANSPORT_TYPE_TCP", "fabrics_id":{"traddr": "127.0.0.1", "trsvcid": "4421", "adrfam": "NVME_ADDRESS_FAMILY_IPV4"}}}'

# get
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeNamespaces/namespace0
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeControllers/ctrl0

# list
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeNamespaces
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeControllers

# stats
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0:stats
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeNamespaces/namespace0:stats
curl -X GET -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeControllers/ctrl0:stats

# update
# update subsys returns not implemented error
#curl -X PATCH -k http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0 -d '{"spec": {"nqn": "nqn.2022-09.io.spdk:opitest1"}}'
curl -X PATCH -k http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeNamespaces/namespace0 -d '{"spec": {"volume_name_ref": "Malloc1", "host_nsid": 10}}'
curl -X PATCH -k http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeControllers/ctrl0 -d '{"spec": {"trtype": "NVME_TRANSPORT_TYPE_TCP", "fabrics_id":{"traddr": "127.0.0.1", "trsvcid": "4421", "adrfam": "NVME_ADDRESS_FAMILY_IPV4"}}}'

# delete
curl -X DELETE -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeControllers/ctrl0
curl -X DELETE -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0/nvmeNamespaces/namespace0
curl -X DELETE -f http://127.0.0.1:8082/v1/nvmeSubsystems/subsys0

# this is last line
docker-compose ps -a
