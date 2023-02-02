# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Dell Inc, or its subsidiaries.

FROM docker.io/library/golang:1.20.0-alpine as builder

WORKDIR /app

# Download necessary Go modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# build an app
COPY cmd/client/*.go cmd/client/
COPY pkg/client/*.go pkg/client/
WORKDIR /app/cmd/client
RUN go build -v -o /opi-storage-client && CGO_ENABLED=0 go test -v /app/...

# second stage to reduce image size
FROM alpine:3.17
COPY --from=builder /opi-storage-client /
EXPOSE 50051
CMD [ "/opi-storage-client"]
