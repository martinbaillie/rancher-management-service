// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

package rancher

// This file provides server-side bindings for the HTTP transport.
// It utilizes the transport/http.Server.

import (
	"encoding/json"
	"errors"
	"net/http"

	"context"

	"github.com/gorilla/mux"
	stdopentracing "github.com/opentracing/opentracing-go"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/tracing/opentracing"

	kithttp "github.com/go-kit/kit/transport/http"
)

// HTTPHandlers is a holder for the Rancher package's HTTP handlers.
type HTTPHandlers struct {
	Container  http.Handler
	Containers http.Handler
}

// The requested object was not found in the repository.
// swagger:model notFoundResponse
type notFoundResponse struct {
	httpErrorBody
}

// The requested object could not be retrieved due to a failed dependency.
// swagger:model failedDependencyResponse
type failedDependencyResponse struct {
	httpErrorBody
}

// An internal error has caused the requested service to become unavailable.
// swagger:model serviceUnavailableResponse
type serviceUnavailableResponse struct {
	httpErrorBody
}

// httpErrorBody encapsulates the contents of an HTTP error.
type httpErrorBody struct {
	// required: true
	// min: 1
	Error  string `json:"Error"`
	Status int    `json:"-"`
}

// MakeHTTPHandlers creates a new instance of HTTPHandlers.
// Each handler is decorated with opentracing annotations.
func MakeHTTPHandlers(ctx context.Context, es ServerEndpoints, tracer stdopentracing.Tracer, logger log.Logger) HTTPHandlers {
	options := []kithttp.ServerOption{
		kithttp.ServerErrorLogger(logger),
		kithttp.ServerErrorEncoder(encodeHTTPError),
	}

	return HTTPHandlers{
		// Containers swagger:route GET /containers containers containers
		//
		// Get summaries of all Rancher containers in the environment
		//
		// Produces:
		// - application/json
		//
		// Schemes: http, https
		//
		// Responses:
		//	200: containersResponse
		//	424: body:failedDependencyResponse The upstream Rancher metadata service was unavilable.
		//  500: body:serviceUnavailableResponse An internal error has occurred.
		Containers: kithttp.NewServer(
			ctx,
			es.ContainersEndpoint,
			DecodeHTTPContainersRequest,
			EncodeHTTPGenericResponse,
			append(options, kithttp.ServerBefore(
				opentracing.FromHTTPRequest(tracer, "Containers", logger)))...,
		),

		// Container swagger:route GET /containers/{name} containers container
		//
		// Get a summary for a single Rancher container in the environment
		//
		// Produces:
		// - application/json
		//
		// Schemes: http, https
		//
		// Responses:
		//	200: containerResponse
		//  404: body:notFoundResponse The container was not found in the repository.
		//	424: body:failedDependencyResponse The upstream Rancher metadata service was unavilable.
		//  500: body:serviceUnavailableResponse An internal error has occurred.
		Container: kithttp.NewServer(
			ctx,
			es.ContainerEndpoint,
			DecodeHTTPContainerRequest,
			EncodeHTTPGenericResponse,
			append(options, kithttp.ServerBefore(
				opentracing.FromHTTPRequest(tracer, "Container", logger)))...,
		),
	}
}

// DecodeHTTPContainersRequest JSON decodes the request into a containersRequest
func DecodeHTTPContainersRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req containersRequest

	// Special case while Containers requests are empty
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return req, nil
}

// DecodeHTTPContainerRequest JSON decodes the request into a containerRequest
func DecodeHTTPContainerRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req containerRequest

	req.Name = mux.Vars(r)["name"]
	if req.Name == "" {
		return nil, errors.New("failed to extract container name from URL")
	}

	return req, nil
}

// EncodeHTTPGenericResponse is an EncodeResponseFunc that encodes the response
// as JSON to the response writer, handling any error conditions
func EncodeHTTPGenericResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if e, ok := response.(errorer); ok && e.error() != nil {
		// Business logic error has occurred
		encodeHTTPError(ctx, e.error(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

func encodeHTTPError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Handle the Rancher package's business errors
	var resp httpErrorBody
	resp.Error = err.Error()
	switch err {
	case ErrContainerNotFound, ErrHostNotFound:
		resp.Status = http.StatusNotFound
	case ErrContainerRepoEmpty, ErrHostRepoEmpty:
		resp.Status = http.StatusFailedDependency
	default:
		resp.Status = http.StatusInternalServerError
	}

	w.WriteHeader(resp.Status)
	json.NewEncoder(w).Encode(resp)
}

func decodeMetadataContainersResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response metadataContainersResponse

	if err := json.NewDecoder(resp.Body).Decode(&response.Containers); err != nil {
		return nil, err
	}
	return response, nil
}

func decodeMetadataHostsResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response metadataHostsResponse

	if err := json.NewDecoder(resp.Body).Decode(&response.Hosts); err != nil {
		return nil, err
	}
	return response, nil
}

func encodeMetadataGenericRequest(_ context.Context, r *http.Request, request interface{}) error {
	req := request.(metadataGenericRequest)

	// We want Rancher's metadata service to return JSON rather than plaintext
	r.Header.Set("Accept", "application/json")

	// Set the specific Rancher metadata service subpath we're targeting
	r.URL.Path = r.URL.Path + req.Subpath

	return nil
}
