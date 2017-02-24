// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

package rancher

import "context"

// The Rancher package's servicing functionality is split into Server services
// and Client services, where:
// - ServerService encapsulates services that are ultimately called by the end
// user as part of e.g. HTTP or gRPC transports.
// - ClientService encapsulates services used internally by the Rancher package
// to integrate to external 3rd party services e.g. the Rancher metadata service.

// ServerService encapsulates services that are ultimately called by the end
// user as part of e.g. HTTP or gRPC transports.
type ServerService interface {
	Container(ctx context.Context, name string) (*Container, error)
	Containers(ctx context.Context) ([]*Container, error)
}

type serverService struct {
	repository Repository
}

// NewServerService creates a new instance of ServerService.
func NewServerService(r Repository) ServerService {
	return &serverService{
		repository: r,
	}
}

// Container implements ServerService.
// It calls into the configured Repository implementation of ContainersByName.
func (s serverService) Container(_ context.Context, name string) (*Container, error) {
	c, err := s.repository.ContainerByName(name)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Containers implements ServerService.
// It calls into the configured Repository implementation of Containers.
func (s serverService) Containers(_ context.Context) ([]*Container, error) {
	cs, err := s.repository.Containers()
	if err != nil {
		return nil, err
	}
	return cs, nil
}

// ClientService encapsulates services used internally by the Rancher package
// to integrate to external 3rd party services e.g. the Rancher metadata service.
type ClientService interface {
	MetadataContainers() ([]*Container, error)
	MetadataHosts() ([]*Host, error)
}

type clientService struct {
	context.Context
	ClientEndpoints
}

// NewClientService creates a new instance of ClientService.
func NewClientService(ctx context.Context, ces ClientEndpoints) ClientService {
	return &clientService{
		Context:         ctx,
		ClientEndpoints: ces,
	}
}

// MetadataContainers implements ClientService.
// It calls the configured MetadataContainersEndpoint, i.e.:
// <metadata scheme>://<metadata URL>/<metadata version>/containers
func (cs clientService) MetadataContainers() ([]*Container, error) {
	res, err := cs.MetadataContainersEndpoint(cs.Context, metadataGenericRequest{Subpath: "/containers"})
	if err != nil {
		return nil, err
	}
	return res.(metadataContainersResponse).Containers, nil
}

// MetadataHosts implements ClientService.
// It calls the configured MetadataHostsEndpoint, i.e.:
// <metadata scheme>://<metadata URL>/<metadata version>/hosts
func (cs clientService) MetadataHosts() ([]*Host, error) {
	res, err := cs.MetadataHostsEndpoint(cs.Context, metadataGenericRequest{Subpath: "/hosts"})
	if err != nil {
		return nil, err
	}
	return res.(metadataHostsResponse).Hosts, nil
}
