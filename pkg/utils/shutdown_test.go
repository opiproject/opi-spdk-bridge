// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2023 Intel Corporation

// Package utils contains utility functions
package utils

import (
	"context"
	"errors"
	"log"
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
	servePanic bool, shutdownPanic bool,
) *serveShutdownPair {
	shutdownTrigger := make(chan struct{}, 1)
	s := &serveShutdownPair{
		shutdownTrigger: shutdownTrigger,
		serve: func() error {
			mu.Lock()
			*serveIDs = append(*serveIDs, fnID)
			mu.Unlock()

			if servePanic {
				log.Panic("Panic!")
			}

			if serveErr == nil {
				<-shutdownTrigger
			}

			return serveErr
		},
		shutdown: func(ctx context.Context) error {
			mu.Lock()
			*shutdownIDs = append(*shutdownIDs, fnID)
			mu.Unlock()

			shutdownTrigger <- struct{}{}

			if shutdownPanic {
				log.Panic("Panic!")
			}

			return shutdownErr
		},
	}

	return s
}

func errString(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

func TestRunAndWait(t *testing.T) {
	stubErr := errors.New("stub error")
	tests := map[string]struct {
		giveServeErr       error
		giveServePanic     bool
		giveShutdownErr    error
		giveShutdownPanic  bool
		stoppedByInterrupt bool
		wantErr            string
	}{
		"all services successfully completed": {
			giveServeErr:       nil,
			giveServePanic:     false,
			giveShutdownErr:    nil,
			giveShutdownPanic:  false,
			stoppedByInterrupt: true,
			wantErr:            "",
		},
		"serve failed": {
			giveServeErr:       stubErr,
			giveServePanic:     false,
			giveShutdownErr:    nil,
			stoppedByInterrupt: false,
			giveShutdownPanic:  false,
			wantErr:            stubErr.Error(),
		},
		"shutdown failed": {
			giveServeErr:       nil,
			giveServePanic:     false,
			giveShutdownErr:    stubErr,
			giveShutdownPanic:  false,
			stoppedByInterrupt: true,
			wantErr:            stubErr.Error(),
		},
		"serve panic": {
			giveServeErr:       nil,
			giveServePanic:     true,
			giveShutdownErr:    nil,
			giveShutdownPanic:  false,
			stoppedByInterrupt: false,
			wantErr:            "was panic for serve function, recovered value: Panic!",
		},
		"shutdown panic": {
			giveServeErr:       nil,
			giveServePanic:     false,
			giveShutdownErr:    nil,
			giveShutdownPanic:  true,
			stoppedByInterrupt: true,
			wantErr:            "was panic for shutdown function, recovered value: Panic!",
		},
	}
	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			sh := NewShutdownHandler(1 * time.Millisecond)

			serveFnIDs := &[]int{}
			shutdownFnIDs := &[]int{}
			mu := sync.Mutex{}
			s0 := newServeShutdownPair(0, serveFnIDs, shutdownFnIDs, &mu, nil, nil, false, false)
			s1 := newServeShutdownPair(1, serveFnIDs, shutdownFnIDs, &mu,
				tt.giveServeErr, tt.giveShutdownErr,
				tt.giveServePanic, tt.giveShutdownPanic,
			)
			s2 := newServeShutdownPair(2, serveFnIDs, shutdownFnIDs, &mu, nil, nil, false, false)

			sh.AddServe(s0.serve, s0.shutdown)
			sh.AddServe(s1.serve, s1.shutdown)
			sh.AddServe(s2.serve, s2.shutdown)

			if tt.stoppedByInterrupt {
				sh.waitSignal <- os.Interrupt
			}

			err := sh.RunAndWait()

			if errString(err) != tt.wantErr {
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
