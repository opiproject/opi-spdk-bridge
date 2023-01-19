# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Dell Inc, or its subsidiaries.

FROM docker.io/library/golang:1.19.5-alpine as builder

WORKDIR /app

# Download necessary Go modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# build an app
COPY cmd/server/*.go cmd/server/
COPY pkg/server/*.go pkg/server/
WORKDIR /app/cmd/server
RUN go build -v -o /opi-spdk-bridge && CGO_ENABLED=0 go test -v ./...

# second stage to reduce image size
FROM alpine:3.17
COPY --from=builder /opi-spdk-bridge /
COPY --from=docker.io/fullstorydev/grpcurl:v1.8.7-alpine /bin/grpcurl /usr/local/bin/
EXPOSE 50051
CMD [ "/opi-spdk-bridge", "-port=50051" ]
HEALTHCHECK CMD grpcurl -plaintext localhost:50051 list || exit 1
