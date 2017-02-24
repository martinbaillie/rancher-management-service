// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause
package rancher

import (
	"io/ioutil"
	"net/url"
	"testing"
	"time"

	stdopentracing "github.com/opentracing/opentracing-go"

	"context"

	"gopkg.in/jarcoal/httpmock.v1"

	"github.com/stretchr/testify/assert"
)

const (
	metadataURLStr   = "http://rancher-metadata/latest"
	containersURLStr = metadataURLStr + "/containers"
	hostsURLStr      = metadataURLStr + "/hosts"
	cacheInterval    = time.Duration(300) * time.Second
)

var (
	rcs                          ClientService
	defaultContainerResponder    httpmock.Responder
	defaultHostResponder         httpmock.Responder
	defaultContainers            []*Container
	defaultContainersNoHostNames []*Container
	defaultHosts                 []*Host
)

type ContainersMethodTestAssertion struct {
	containersResponder httpmock.Responder
	hostsResponder      httpmock.Responder
	expectedContainers  []*Container
	description         string
	expectedError       error
}

type HostsMethodTestAssertion struct {
	containersResponder httpmock.Responder
	hostsResponder      httpmock.Responder
	expectedHosts       []*Host
	description         string
	expectedError       error
}

func init() {
	// Mock the remote calls to the rancher-metadata service
	httpmock.Activate()

	containersResponse, _ := ioutil.ReadFile("testdata/rancher_containers.json")
	hostsResponse, _ := ioutil.ReadFile("testdata/rancher_hosts.json")

	defaultContainerResponder = httpmock.NewStringResponder(200, string(containersResponse))
	defaultHostResponder = httpmock.NewStringResponder(200, string(hostsResponse))

	httpmock.RegisterResponder("GET", containersURLStr, defaultContainerResponder)
	httpmock.RegisterResponder("GET", hostsURLStr, defaultHostResponder)

	// Set up a thin client service (i.e. no tracing, logging, instrumentation decorations)
	ctx := context.Background()
	tracer := stdopentracing.GlobalTracer()
	metadataURL, _ := url.Parse(metadataURLStr)
	rcses := NewClientEndpoints(ctx, metadataURL, tracer)
	rcs = NewClientService(ctx, rcses)

	// Default slices for when nothing has gone wrong
	defaultContainers = []*Container{
		&Container{
			Name:         "web_gossman_2",
			State:        "running",
			PrivateIP:    "10.42.250.129",
			ServiceIndex: 2,
			Host: Host{
				UUID: "e966be1e-6543-4310-9a4a-5016f86b0eb1",
				Name: "host-4.corp",
			},
		},
		&Container{
			Name:         "web_service-web_1",
			State:        "running",
			PrivateIP:    "10.42.118.210",
			ServiceIndex: 1,
			Host: Host{
				UUID: "bfa1363f-8f2a-44de-afb6-a1bb7db1d614",
				Name: "host-2.corp",
			},
		},
		&Container{
			Name:         "web_web-self-service_1",
			State:        "running",
			PrivateIP:    "10.42.97.176",
			ServiceIndex: 1,
			Host: Host{
				UUID: "e966be1e-6543-4310-9a4a-5016f86b0eb1",
				Name: "host-4.corp",
			},
		},
		&Container{
			Name:         "web_web-self-service_2",
			State:        "running",
			PrivateIP:    "10.42.171.148",
			ServiceIndex: 2,
			Host: Host{
				UUID: "e966be1e-6543-4310-9a4a-5016f86b0eb1",
				Name: "host-4.corp",
			},
		},
		&Container{
			Name:         "web_web-deployment_1",
			State:        "stopped",
			PrivateIP:    "10.42.156.227",
			ServiceIndex: 1,
			Host: Host{
				UUID: "259466fc-2c68-4701-8fe5-0ca5be5d354f",
				Name: "host-1.corp",
			},
		},
		&Container{
			Name:         "web_web-deployment_2",
			State:        "stopped",
			PrivateIP:    "10.42.203.121",
			ServiceIndex: 2,
			Host: Host{
				UUID: "e966be1e-6543-4310-9a4a-5016f86b0eb1",
				Name: "host-4.corp",
			},
		},
	}

	defaultContainersNoHostNames = make([]*Container, len(defaultContainers))
	for i, c := range defaultContainers {
		cNoHostName := *c
		cNoHostName.Host.Name = ""
		defaultContainersNoHostNames[i] = &cNoHostName
	}

	defaultHosts = []*Host{
		&Host{
			UUID: "259466fc-2c68-4701-8fe5-0ca5be5d354f",
			Name: "host-1.corp",
		},
		&Host{
			UUID: "bfa1363f-8f2a-44de-afb6-a1bb7db1d614",
			Name: "host-2.corp",
		},
		&Host{
			UUID: "e966be1e-6543-4310-9a4a-5016f86b0eb1",
			Name: "host-4.corp",
		},
	}
}

func ContainersTestRunner(t *testing.T, ts *[]ContainersMethodTestAssertion) {
	assert := assert.New(t)
	httpmock.Activate()
	defer httpmock.Deactivate()

	for _, tc := range *ts {
		httpmock.RegisterResponder("GET", containersURLStr, tc.containersResponder)
		httpmock.RegisterResponder("GET", hostsURLStr, tc.hostsResponder)

		repository := NewMetadataCachingRepository(rcs, cacheInterval)
		repository.refreshContainers()
		res, err := repository.Containers()

		assert.Equal(tc.expectedContainers, res, tc.description)
		assert.Equal(tc.expectedError, err, tc.description)

		httpmock.Reset()
	}
}

func HostsTestRunner(t *testing.T, ts *[]HostsMethodTestAssertion) {
	assert := assert.New(t)
	httpmock.Activate()
	defer httpmock.Deactivate()

	for _, tc := range *ts {
		httpmock.RegisterResponder("GET", containersURLStr, tc.containersResponder)
		httpmock.RegisterResponder("GET", hostsURLStr, tc.hostsResponder)

		repository := NewMetadataCachingRepository(rcs, cacheInterval)
		repository.refreshHosts()
		res, err := repository.Hosts()

		assert.Equal(tc.expectedHosts, res, tc.description)
		assert.Equal(tc.expectedError, err, tc.description)

		httpmock.Reset()
	}
}

func TestContainersMethod(t *testing.T) {
	ContainersTestRunner(t,
		&[]ContainersMethodTestAssertion{
			{
				description:         "Containers() success",
				containersResponder: defaultContainerResponder,
				hostsResponder:      defaultHostResponder,
				expectedContainers:  defaultContainers,
				expectedError:       nil,
			},
			{
				description:         "Containers() success, but empty host name",
				containersResponder: defaultContainerResponder,
				hostsResponder:      httpmock.NewStringResponder(404, ""),
				expectedContainers:  defaultContainersNoHostNames,
				expectedError:       nil,
			},
			{
				description:         "Containers() repository empty",
				containersResponder: httpmock.NewStringResponder(200, `[]`),
				hostsResponder:      defaultHostResponder,
				expectedContainers:  []*Container{},
				expectedError:       ErrContainerRepoEmpty,
			},
			{
				description:         "Containers() upstream unavailable",
				containersResponder: httpmock.NewStringResponder(500, ""),
				hostsResponder:      defaultHostResponder,
				expectedContainers:  []*Container{},
				expectedError:       ErrContainerRepoEmpty,
			},
		})
}

func TestHostsMethod(t *testing.T) {
	HostsTestRunner(t,
		&[]HostsMethodTestAssertion{
			{
				description:         "Hosts() success",
				containersResponder: defaultContainerResponder,
				hostsResponder:      defaultHostResponder,
				expectedHosts:       defaultHosts,
				expectedError:       nil,
			},
			{
				description:         "Hosts() repository empty",
				containersResponder: defaultContainerResponder,
				hostsResponder:      httpmock.NewStringResponder(200, `[]`),
				expectedHosts:       []*Host{},
				expectedError:       ErrHostRepoEmpty,
			},
			{
				description:         "Hosts() upstream unavailable",
				containersResponder: defaultContainerResponder,
				hostsResponder:      httpmock.NewStringResponder(500, ""),
				expectedHosts:       []*Host{},
				expectedError:       ErrHostRepoEmpty,
			},
		})
}

func TestContainerByName(t *testing.T) {
	assert := assert.New(t)
	httpmock.Activate()
	defer httpmock.Deactivate()

	httpmock.RegisterResponder("GET", containersURLStr, defaultContainerResponder)

	repository := NewMetadataCachingRepository(rcs, cacheInterval)
	repository.refreshContainers()
	res, err := repository.ContainerByName("web_gossman_2")
	assert.Equal(defaultContainersNoHostNames[0], res, "ContainerByName() success")
	assert.Equal(nil, err, "ContainerByName() success")

	res, err = repository.ContainerByName("")
	assert.Equal((*Container)(nil), res, "ContainerByName() failure")
	assert.Equal(ErrContainerNotFound, err, "ContainerByName() failure")
}

func TestHostByUUID(t *testing.T) {
	assert := assert.New(t)
	httpmock.Activate()
	defer httpmock.Deactivate()

	httpmock.RegisterResponder("GET", hostsURLStr, defaultHostResponder)

	repository := NewMetadataCachingRepository(rcs, cacheInterval)
	repository.refreshHosts()
	res, err := repository.HostByUUID("bfa1363f-8f2a-44de-afb6-a1bb7db1d614")
	assert.Equal(defaultHosts[1], res, "HostByUUID() success")
	assert.Equal(nil, err, "HostByUUID() success")

	res, err = repository.HostByUUID("")
	assert.Equal((*Host)(nil), res, "HostByUUID() failure")
	assert.Equal(ErrHostNotFound, err, "HostByUUID() failure")
}
