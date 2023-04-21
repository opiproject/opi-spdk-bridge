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

STORAGE_CLIENT_NAME=$(docker-compose ps | grep opi-spdk-client | awk '{print $1}')
STORAGE_CLIENT_RC=$(docker wait "${STORAGE_CLIENT_NAME}")
if [ "${STORAGE_CLIENT_RC}" != "0" ]; then
    echo "opi-spdk-client failed:"
    docker logs "${STORAGE_CLIENT_NAME}"
    exit 1
fi

# Check exported port also works (host network)
docker run --network=host --rm docker.io/namely/grpc-cli ls 127.0.0.1:50051

# check reflection
grpc_cli=(docker run --network=opi-spdk-bridge_opi --rm docker.io/namely/grpc-cli)
"${grpc_cli[@]}" ls opi-spdk-server:50051
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.AioControllerService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.FrontendNvmeService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.FrontendVirtioBlkService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.FrontendVirtioScsiService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.NVMfRemoteControllerService -l
"${grpc_cli[@]}" ls opi-spdk-server:50051 opi_api.storage.v1.NullDebugService -l

# test nvme
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNVMeSubsystem "{nv_me_subsystem : {spec : {id : {value : 'subsystem1'}, nqn: 'nqn.2022-09.io.spdk:opitest1', serial_number: 'myserial1', model_number: 'mymodel1', max_namespaces: 11} } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNVMeController "{nv_me_controller : {spec : {id : {value : 'controller1'}, nvme_controller_id: 2, subsystem_id : { value : 'subsystem1' }, pcie_id : {physical_function : 0}, max_nsq:5, max_ncq:5 } } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNVMeNamespace "{'nv_me_namespace' : { 'spec' : {'id' : {'value' : 'namespace1'}, 'subsystem_id' : { 'value' : 'subsystem1' }, 'volume_id' : { 'value' : 'Malloc1' }, 'host_nsid' : '1' } } }"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNVMeSubsystem "{name : 'subsystem1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNVMeController "{name : 'controller1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNVMeNamespace "{name :  'namespace1'}"
docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/spdk:main spdk_nvme_identify -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:4444'
docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/spdk:main spdk_nvme_perf     -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:4444 subnqn:nqn.2022-09.io.spdk:opitest1 hostnqn:nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c' -c 0x1 -q 1 -o 4096 -w randread -t 10 | tee log.txt
grep "Total" log.txt
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNVMeNamespace "{name : 'namespace1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNVMeController "{name : 'controller1'}"
"${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNVMeSubsystem "{name : 'subsystem1'}"

# this is last line
docker-compose ps -a
