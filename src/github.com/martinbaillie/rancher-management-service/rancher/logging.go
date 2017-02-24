// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

package rancher

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"context"

	"github.com/go-kit/kit/log"
	level "github.com/go-kit/kit/log/experimental_level"
)

// NewServerServiceLogger returns a new instance of a ServerService logging wrapper.
func NewServerServiceLogger(l log.Logger, s ServerService) ServerService {
	return &serverServiceLogger{
		logger:  l,
		service: s,
	}
}

type serverServiceLogger struct {
	logger  log.Logger
	service ServerService
}

// Log is a helper wrapper for printing details common to all log statements
func Log(logger log.Logger, begin time.Time, err error, additionalKVs ...interface{}) {
	pc, _, _, _ := runtime.Caller(1)
	caller := strings.Split(runtime.FuncForPC(pc).Name(), ".")
	defaultKVs := []interface{}{
		"method", caller[len(caller)-2],
		"took", time.Since(begin),
		"success", fmt.Sprint(err == nil),
	}

	if err != nil {
		defaultKVs = append(defaultKVs, "err")
		defaultKVs = append(defaultKVs, err)
		level.Error(logger).Log(defaultKVs...)
	} else {
		level.Info(logger).Log(append(defaultKVs, additionalKVs...)...)
	}
}

// Containers decorates the wrapped ServerService method with useful structured logging.
func (s *serverServiceLogger) Containers(ctx context.Context) (cs []*Container, err error) {
	defer func(begin time.Time) {
		Log(s.logger, begin, err, "container_count", len(cs))
	}(time.Now())
	return s.service.Containers(ctx)
}

// Container decorates the wrapped ServerService method with useful structured logging.
func (s *serverServiceLogger) Container(ctx context.Context, name string) (c *Container, err error) {
	defer func(begin time.Time) {
		Log(s.logger, begin, err, "container_name", name)
	}(time.Now())
	return s.service.Container(ctx, name)
}

// NewClientServiceLogger returns a new instance of a ClientService logging wrapper.
func NewClientServiceLogger(l log.Logger, s ClientService) ClientService {
	return &clientServiceLogger{
		logger:  l,
		service: s,
	}
}

type clientServiceLogger struct {
	logger  log.Logger
	service ClientService
}

// MetadataContainers decorates the wrapped ClientService method with useful structured logging.
func (s *clientServiceLogger) MetadataContainers() (cs []*Container, err error) {
	defer func(begin time.Time) {
		Log(s.logger, begin, err, "container_count", len(cs))
	}(time.Now())
	return s.service.MetadataContainers()
}

// MetadataHosts decorates the wrapped ClientService method with useful structured logging.
func (s *clientServiceLogger) MetadataHosts() (hs []*Host, err error) {
	defer func(begin time.Time) {
		Log(s.logger, begin, err, "host_count", len(hs))
	}(time.Now())
	return s.service.MetadataHosts()
}
