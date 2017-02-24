// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

package rancher

import (
	"net/url"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/tracing/opentracing"
	kithttp "github.com/go-kit/kit/transport/http"

	"context"

	stdopentracing "github.com/opentracing/opentracing-go"
)

// Error type used for asserting errors in responses
type errorer interface {
	error() error
}

// ServerEndpoints holds the Rancher package's externally facing endpoints
type ServerEndpoints struct {
	ContainerEndpoint  endpoint.Endpoint
	ContainersEndpoint endpoint.Endpoint
}

// NewServerEndpoints creates an instance of ServerEndpoints.
// Each endpoint is decorated with tracing.
func NewServerEndpoints(s ServerService, t stdopentracing.Tracer) ServerEndpoints {
	return ServerEndpoints{
		ContainerEndpoint:  opentracing.TraceServer(t, "rancher-container-endpoint")(ContainerEndpoint(s)),
		ContainersEndpoint: opentracing.TraceServer(t, "rancher-containers-endpoint")(ContainersEndpoint(s)),
	}
}

// containerRequest A container parameter model.
//
// Used for identifying the name of the container.
//
// swagger:parameters container
type containerRequest struct {
	// The name of the container
	//
	// in: path
	// required: true
	Name string
}

// containerResponse A container response model.
//
// Used for returning a response with a single container.
//
// swagger:response containerResponse
type containerResponse struct {
	// in: body
	Container *Container `json:"Container,omitempty"`
	// in: body
	Err error `json:"Error,omitempty"`
}

func (r containerResponse) error() error { return r.Err }

// ContainerEndpoint implements ServerService.
// This endpoint is used as part of a server interaction.
func ContainerEndpoint(s ServerService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		containerReq := request.(containerRequest)
		c, err := s.Container(ctx, containerReq.Name)
		return containerResponse{
			Container: c,
			Err:       err,
		}, nil
	}
}

// containersRequest A containers parameter model.
//
// Unused. Possibly a filter by partial name in future.
//
// swagger:parameters containers
type containersRequest struct{}

// containersResponse A containers response model.
//
// Used for returning a collection of containers.
//
// swagger:response containersResponse
type containersResponse struct {
	// in: body
	Containers []*Container `json:"Containers,omitempty"`
	// in: body
	Err error `json:"Error,omitempty"`
}

func (r containersResponse) error() error { return r.Err }

// ContainersEndpoint implements ServerService.
// This endpoint is used as part of a server interaction.
func ContainersEndpoint(s ServerService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		// containersReq := request.(containersRequest)
		// NOTE: Unused, see containersRequest struct
		cs, err := s.Containers(ctx)
		return containersResponse{
			Containers: cs,
			Err:        err,
		}, nil
	}
}

// ClientEndpoints holds the Rancher package's internally used endpoints
type ClientEndpoints struct {
	MetadataContainersEndpoint endpoint.Endpoint
	MetadataHostsEndpoint      endpoint.Endpoint
}

// NewClientEndpoints creates an instance of ClientEndpoints.
// Each endpoint is decorated with tracing and circuit breaking.
func NewClientEndpoints(ctx context.Context, metadataServiceURL *url.URL, t stdopentracing.Tracer) ClientEndpoints {
	var mce endpoint.Endpoint
	mce = MetadataContainersEndpoint(ctx, metadataServiceURL)
	mce = opentracing.TraceServer(t, "rancher-metadata-service-containers-endpoint")(mce)
	mce = circuitbreaker.Hystrix("rancher-metadata-service-containers-endpoint")(mce)

	var mhe endpoint.Endpoint
	mhe = MetadataHostsEndpoint(ctx, metadataServiceURL)
	mhe = opentracing.TraceServer(t, "rancher-metadata-service-hosts-endpoint")(mhe)
	mhe = circuitbreaker.Hystrix("rancher-metadata-service-hosts-endpoint")(mhe)

	return ClientEndpoints{
		MetadataContainersEndpoint: mce,
		MetadataHostsEndpoint:      mhe,
	}
}

type metadataGenericRequest struct {
	Subpath string
}

type metadataContainersResponse struct {
	Containers []*Container
}

type metadataHostsResponse struct {
	Hosts []*Host
}

// MetadataContainersEndpoint implements ClientService.
// This endpoint is used as part of a client interaction.
func MetadataContainersEndpoint(ctx context.Context, metadataServiceURL *url.URL) endpoint.Endpoint {
	return kithttp.NewClient(
		"GET", metadataServiceURL,
		encodeMetadataGenericRequest,
		decodeMetadataContainersResponse,
	).Endpoint()
}

// MetadataHostsEndpoint implements ClientService.
// This endpoint is used as part of a client interaction.
func MetadataHostsEndpoint(ctx context.Context, metadataServiceURL *url.URL) endpoint.Endpoint {
	return kithttp.NewClient(
		"GET", metadataServiceURL,
		encodeMetadataGenericRequest,
		decodeMetadataHostsResponse,
	).Endpoint()
}
