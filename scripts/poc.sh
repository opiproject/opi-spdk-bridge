#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
# Copyright (c) 2022 Dell Inc, or its subsidiaries.

set -euxo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# docker compose plugin
command -v docker-compose || { shopt -s expand_aliases && alias docker-compose='docker compose'; }

usage() {
    echo "Usage: poc.sh [start|stop|tests|logs]"
}

tests_poc() {
    if [ "$(uname -m)" == "x86_64" ]; then
        # Show x86-64-v level.
        "${SCRIPT_DIR}"/x86v.sh
    fi
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
    STORAGE_CLIENT_RC=$(docker inspect --format '{{.State.ExitCode}}' "${STORAGE_CLIENT_NAME}")
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
    "${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 CreateNVMeNamespace "{'namespace' : { 'spec' : {'id' : {'value' : 'namespace1'}, 'subsystem_id' : { 'value' : 'nqn.2016-06.io.spdk:cnode1' }, 'volume_id' : { 'value' : 'Malloc1' }, 'host_nsid' : '1' } } }"
    "${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 GetNVMeNamespace "{'namespace_id' : {'value' : 'namespace1'} }"
    docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/opi-storage-spdk:main spdk_nvme_identify -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:4444'
    docker run --rm --network=host --privileged -v /dev/hugepages:/dev/hugepages ghcr.io/opiproject/opi-storage-spdk:main spdk_nvme_perf     -r 'traddr:127.0.0.1 trtype:TCP adrfam:IPv4 trsvcid:4444' -c 0x1 -q 1 -o 4096 -w randread -t 10
    "${grpc_cli[@]}" call --json_input --json_output opi-spdk-server:50051 DeleteNVMeNamespace "{'namespace_id' : {'value' : 'namespace1'} }"

    # this is last line
    docker-compose ps -a
}

stop_poc() {
    docker-compose down --volumes
    docker network prune --force
}

start_poc() {
    docker-compose up --detach
}

logs_poc() {
    docker-compose ps -a
    docker-compose logs || true
    docker inspect "$(docker-compose ps -q)" || true
    netstat -an || true
    ifconfig -a || true
}

stop_containers() {
    docker-compose down --volumes
}

if [ $# -eq 0 ]
  then
	usage
	exit 1
fi

case ${1} in
    start)
        echo "Starting PoC"
        stop_poc
        start_poc
        ;;
    stop)
        echo "Stopping PoC"
        stop_poc
        ;;
    tests)
        echo "Testing PoC"
        tests_poc
        ;;
    logs)
        echo "Logs PoC"
        logs_poc
        ;;
    *)
        usage
        ;;
esac
