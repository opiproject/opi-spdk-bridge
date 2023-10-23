// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package utils contains utility functions
package utils

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/philippgille/gokv"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

// ServeFunc function to run service job
type ServeFunc func() error

// ShutdownFunc function to perform shutdown of a service
type ShutdownFunc func(ctx context.Context) error

// ShutdownHandler is responsible for running services and perform their shutdowns
// on service error or signals
type ShutdownHandler struct {
	waitSignal         chan os.Signal
	timeoutPerShutdown time.Duration

	mu        sync.Mutex
	serves    []ServeFunc
	shutdowns []ShutdownFunc

	eg    *errgroup.Group
	egCtx context.Context
}

// NewShutdownHandler creates an instance of ShutdownHandler
func NewShutdownHandler(
	timeoutPerShutdown time.Duration,
) *ShutdownHandler {
	eg, egCtx := errgroup.WithContext(context.Background())

	return &ShutdownHandler{
		waitSignal:         make(chan os.Signal, 1),
		timeoutPerShutdown: timeoutPerShutdown,

		mu:        sync.Mutex{},
		serves:    []ServeFunc{},
		shutdowns: []ShutdownFunc{},

		eg:    eg,
		egCtx: egCtx,
	}
}

// AddServe adds a service to run ant corresponding shutdown
func (s *ShutdownHandler) AddServe(serve ServeFunc, shutdown ShutdownFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serves = append(s.serves, serve)
	s.shutdowns = append(s.shutdowns, shutdown)
}

// AddShutdown add a shutdown procedure to execute.
// Shutdowns are executed in backward order
func (s *ShutdownHandler) AddShutdown(shutdown ShutdownFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdowns = append(s.shutdowns, shutdown)
}

// AddGrpcServer adds serve and shutdown procedures for provided gRPC server
func (s *ShutdownHandler) AddGrpcServer(server *grpc.Server, lis net.Listener) {
	s.AddServe(
		func() error {
			log.Printf("gRPC Server listening at %v", lis.Addr())
			return server.Serve(lis)
		},
		func(ctx context.Context) error {
			log.Println("Stopping gRPC Server")
			return s.runWithCtx(ctx, func() error {
				server.GracefulStop()
				return nil
			})
		},
	)
}

// AddHTTPServer adds serve and shutdown procedures for provided HTTP server
func (s *ShutdownHandler) AddHTTPServer(server *http.Server) {
	s.AddServe(
		func() error {
			log.Printf("HTTP Server listening at %v", server.Addr)
			err := server.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}

			return err
		},
		func(ctx context.Context) error {
			log.Println("Stopping HTTP Server")
			err := server.Shutdown(ctx)
			if err != nil {
				cerr := server.Close()
				log.Println("HTTP server close error:", cerr)
			}
			return err
		},
	)
}

// AddGokvStore adds gokv shutdown procedure
func (s *ShutdownHandler) AddGokvStore(store gokv.Store) {
	s.AddShutdown(func(ctx context.Context) error {
		log.Println("Stopping gokv storage")
		return s.runWithCtx(ctx, func() error {
			return store.Close()
		})
	})
}

// AddTraceProvider adds trace provider shutdown procedure
func (s *ShutdownHandler) AddTraceProvider(tp *sdktrace.TracerProvider) {
	s.AddShutdown(func(ctx context.Context) error {
		log.Println("Stopping tracer")
		return tp.Shutdown(ctx)
	})
}

// RunAndWait runs all services and execute shutdowns on a signal received
func (s *ShutdownHandler) RunAndWait() error {
	for i := range s.serves {
		fn := s.serves[i]
		s.eg.Go(func() error {
			return fn()
		})
	}

	s.eg.Go(func() error {
		signal.Notify(s.waitSignal, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-s.waitSignal:
			log.Printf("Got signal: %v", sig)
		case <-s.egCtx.Done():
			// can be reached if any Serve returned an error. Thus, initiating shutdown
			log.Println("A process from errgroup exited with error:", s.egCtx.Err())
		}
		log.Printf("Start graceful shutdown with timeout per shutdown call: %v", s.timeoutPerShutdown)

		s.mu.Lock()
		defer s.mu.Unlock()

		var err error
		for i := len(s.shutdowns) - 1; i >= 0; i-- {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), s.timeoutPerShutdown)
			defer cancel()
			err = errors.Join(err, s.shutdowns[i](timeoutCtx))
		}

		return err
	})

	return s.eg.Wait()
}

func (s *ShutdownHandler) runWithCtx(ctx context.Context, fn func() error) error {
	var err error

	stopped := make(chan struct{}, 1)
	func() {
		err = fn()
		stopped <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-stopped:
	}

	return err
}
