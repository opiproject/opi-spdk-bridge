# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.

FROM docker.io/library/golang:1.21.2-alpine as builder

WORKDIR /app

# Download necessary Go modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# build an app
COPY cmd/ cmd/
COPY pkg/ pkg/
RUN go build -v -o /opi-spdk-bridge ./cmd/...

# second stage to reduce image size
FROM alpine:3.18
COPY --from=builder /opi-spdk-bridge /
COPY --from=docker.io/fullstorydev/grpcurl:v1.8.8-alpine /bin/grpcurl /usr/local/bin/
EXPOSE 50051 8082
CMD [ "/opi-spdk-bridge", "-grpc_port=50051", "-http_port=8082" ]
HEALTHCHECK CMD grpcurl -plaintext localhost:50051 list || exit 1
