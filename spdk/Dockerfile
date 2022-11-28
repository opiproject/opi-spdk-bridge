# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Dell Inc, or its subsidiaries.

FROM docker.io/library/fedora:37 as build

ARG TAG=v22.09

WORKDIR /root
RUN dnf install -y git rpm-build diffutils procps-ng pip && dnf clean all

# hadolint ignore=DL3003
RUN git clone https://github.com/spdk/spdk --branch ${TAG} --depth 1 && \
    cd spdk && git submodule update --init --depth 1 && scripts/pkgdep.sh --rdma

ENV GRPC_PYTHON_BUILD_EXT_COMPILER_JOBS 8

# hadolint ignore=DL3003
RUN if [ "$(uname -m)" = "aarch64" ]; then \
        ARCH="generic"; \
    elif [ "$(uname -m)" = "x86_64" ]; then \
        ARCH="x86-64-v2"; \
    fi; \
    cd spdk && ./rpmbuild/rpm.sh --target-arch=${ARCH} --without-uring --with-crypto \
    --without-fio --with-raid5f --with-vhost --without-pmdk --without-rbd \
    --with-rdma --with-shared --with-iscsi-initiator --without-vtune

FROM docker.io/library/fedora:37

RUN dnf install -y hostname && dnf clean all

WORKDIR /root
RUN mkdir -p /root/rpmbuild
COPY --from=build /root/rpmbuild/ /root/rpmbuild/
RUN dnf install -y /root/rpmbuild/rpm/x86_64/*.rpm && dnf clean all
EXPOSE 9009
HEALTHCHECK CMD curl --fail --insecure --user spdkuser:spdkpass -X POST -H 'Content-Type: application/json' -d '{\"id\": 1, \"method\": \"bdev_get_bdevs\"}' http://localhost:9009 || exit 1
CMD ["/usr/libexec/spdk/scripts/rpc_http_proxy.py", "0.0.0.0", "9009", "spdkuser", "spdkpass"]
