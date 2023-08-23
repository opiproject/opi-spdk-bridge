// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package utils contains utility functions
package utils

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/exp/slices"
)

type serveShutdownPair struct {
	shutdownTrigger chan struct{}
	serve           ServeFunc
	shutdown        ShutdownFunc
}

func newServeShutdownPair(
	fnID int,
	serveIDs, shutdownIDs *[]int,
	mu *sync.Mutex,
	serveErr, shutdownErr error,
) *serveShutdownPair {
	shutdownTrigger := make(chan struct{}, 1)
	s := &serveShutdownPair{
		shutdownTrigger: shutdownTrigger,
		serve: func() error {
			if serveErr == nil {
				<-shutdownTrigger
			}

			mu.Lock()
			*serveIDs = append(*serveIDs, fnID)
			mu.Unlock()

			return serveErr
		},
		shutdown: func(ctx context.Context) error {
			mu.Lock()
			*shutdownIDs = append(*shutdownIDs, fnID)
			mu.Unlock()

			shutdownTrigger <- struct{}{}

			return shutdownErr
		},
	}

	return s
}

func TestRunAndWait(t *testing.T) {
	stubErr := errors.New("stub error")
	tests := map[string]struct {
		giveServeErr       error
		giveShutdownErr    error
		stoppedByInterrupt bool
		wantErr            error
	}{
		"all services successfully completed": {
			giveServeErr:       nil,
			giveShutdownErr:    nil,
			stoppedByInterrupt: true,
			wantErr:            nil,
		},
		"serve failed": {
			giveServeErr:       stubErr,
			giveShutdownErr:    nil,
			stoppedByInterrupt: false,
			wantErr:            stubErr,
		},
		"shutdown failed": {
			giveServeErr:       nil,
			giveShutdownErr:    stubErr,
			stoppedByInterrupt: true,
			wantErr:            stubErr,
		},
	}
	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			sh := NewShutdownHandler(1 * time.Millisecond)

			serveFnIDs := &[]int{}
			shutdownFnIDs := &[]int{}
			mu := sync.Mutex{}
			s0 := newServeShutdownPair(0, serveFnIDs, shutdownFnIDs, &mu, nil, nil)
			s1 := newServeShutdownPair(1, serveFnIDs, shutdownFnIDs, &mu, tt.giveServeErr, tt.giveShutdownErr)
			s2 := newServeShutdownPair(2, serveFnIDs, shutdownFnIDs, &mu, nil, nil)

			sh.AddServe(s0.serve, s0.shutdown)
			sh.AddServe(s1.serve, s1.shutdown)
			sh.AddServe(s2.serve, s2.shutdown)

			if tt.stoppedByInterrupt {
				sh.waitSignal <- os.Interrupt
			}

			err := sh.RunAndWait()

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Expected error: %v, received: %v", tt.wantErr, err)
			}

			if !slices.Equal(*shutdownFnIDs, []int{2, 1, 0}) {
				t.Errorf("Expected shutdown functions are called in order, instead %v", shutdownFnIDs)
			}

			slices.Sort(*serveFnIDs)
			if !slices.Equal(*serveFnIDs, []int{0, 1, 2}) {
				t.Errorf("Expected all serve functions are called, instead %v", serveFnIDs)
			}
		})
	}
}
