// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

package rancher

import (
	"time"

	"context"

	"github.com/go-kit/kit/metrics"
)

// NewServerServiceInstrumenter returns an instance of an instrumenting ServerService.
func NewServerServiceInstrumenter(rc metrics.Counter, rl metrics.Histogram, s ServerService) ServerService {
	return &serverServiceInstrumenter{
		requestCount:   rc,
		requestLatency: rl,
		service:        s,
	}
}

type serverServiceInstrumenter struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
	service        ServerService
}

// Containers decorates the wrapped ServerService method with useful Prometheus instrumentation.
func (s *serverServiceInstrumenter) Containers(ctx context.Context) (cs []*Container, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("method", "Containers").Add(1)
		s.requestLatency.With("method", "Containers").Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.service.Containers(ctx)
}

// Container decorates the wrapped ServerService method with useful Prometheus instrumentation.
func (s *serverServiceInstrumenter) Container(ctx context.Context, name string) (c *Container, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("method", "Container").Add(1)
		s.requestLatency.With("method", "Container").Observe(time.Since(begin).Seconds())
	}(time.Now())
	return s.service.Container(ctx, name)
}

// NewClientServiceInstrumenter returns an instance of an instrumenting ClientService.
func NewClientServiceInstrumenter(rc metrics.Counter, rl metrics.Histogram, cs metrics.Gauge, hs metrics.Gauge, s ClientService) ClientService {
	return &clientServiceInstrumenter{
		requestCount:   rc,
		requestLatency: rl,
		containers:     cs,
		hosts:          hs,
		service:        s,
	}
}

type clientServiceInstrumenter struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
	containers     metrics.Gauge
	hosts          metrics.Gauge
	service        ClientService
}

// MetadataContainers decorates the wrapped ClientService method with useful Prometheus instrumentation.
func (s *clientServiceInstrumenter) MetadataContainers() (cs []*Container, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("method", "MetadataContainers").Add(1)
		s.requestLatency.With("method", "MetadataContainers").Observe(time.Since(begin).Seconds())
		s.containers.With("method", "MetadataContainers").Set(float64(len(cs)))
	}(time.Now())
	return s.service.MetadataContainers()
}

// MetadataHosts decorates the wrapped ClientService method with useful Prometheus instrumentation.
func (s *clientServiceInstrumenter) MetadataHosts() (hs []*Host, err error) {
	defer func(begin time.Time) {
		s.requestCount.With("method", "MetadataHosts").Add(1)
		s.requestLatency.With("method", "MetadataHosts").Observe(time.Since(begin).Seconds())
		s.hosts.With("method", "MetadataHosts").Set(float64(len(hs)))
	}(time.Now())
	return s.service.MetadataHosts()
}
