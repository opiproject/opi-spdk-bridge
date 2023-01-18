// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/opiproject/opi-spdk-bridge/pkg/client"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr = flag.String("addr", "localhost:50051", "the address to connect to")
)

func main() {
	flag.Parse()
	// Set up a connection to the server.
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Fatalf("did not close connection: %v", err)
		}
	}(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("Test frontend")
	client.DoFrontend(ctx, conn)

	log.Printf("Test backend")
	client.DoBackend(ctx, conn)

	log.Printf("Test middleend")
	client.DoMiddleend(ctx, conn)
}
