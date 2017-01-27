// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

package rancher

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"
)

// Business errors
var (
	// The container was not found
	//
	// swagger:response ErrContainerNotFound
	ErrContainerNotFound  = errors.New("container not found")
	ErrContainerRepoEmpty = errors.New("container repository is empty")

	ErrHostNotFound  = errors.New("host not found")
	ErrHostRepoEmpty = errors.New("host repository is empty")

	ErrNotImplemented = errors.New("not implemented")
)

// Repository acts as storage for Rancher's metadata.
// See the Rancher package's metadataCachingRepository implementation.
type Repository interface {
	ContainerByName(name string) (*Container, error)
	Containers() ([]*Container, error)
	refreshContainers()

	HostByUUID(uuid string) (*Host, error)
	Hosts() ([]*Host, error)
	refreshHosts()

	cachePopulateEvery(time.Duration)
}

// Container is a Rancher container representation.
//
// swagger:model rancherContainer
type Container struct {
	// the name of the container in Rancher
	// required: true
	// min: 1
	Name string `json:"Name"`
	// the current Rancher state for this container
	// required: true
	// min: 1
	State string `json:"State"`
	// the container's IP address on the Rancher internal overlay network
	// required: true
	// min: 1
	PrivateIP string `json:"PrivateIP"`
	// the Rancher service index for this container
	// NOTE: 0 means container is orphaned on the Rancher host
	// required: true
	// min: 1
	ServiceIndex int64 `json:"ServiceIndex"`
	// the Rancher host this container is running on
	// required: true
	// min: 1
	Host
}

// UnmarshalJSON unmarshals the Rancher container struct
//
// The method is implemented in order to fudge the Rancher JSON to match
// the LetterCase used by Verint/KANA in their JMX beans, for consistency
// in this API. The JSON tags on the structs are what the end user sees.
//
// NOTE: the pointer receiver is important here!
func (c *Container) UnmarshalJSON(b []byte) (err error) {
	var data map[string]interface{}
	if err = json.Unmarshal(b, &data); err != nil {
		return err
	}

	c.Name = data["name"].(string)
	c.State = data["state"].(string)
	c.PrivateIP = data["primary_ip"].(string)

	if serviceIndex := data["service_index"]; serviceIndex != nil {
		c.ServiceIndex, _ = strconv.ParseInt(serviceIndex.(string), 10, 64)
	}

	if hostUUID := data["host_uuid"]; hostUUID != nil {
		c.Host.UUID = hostUUID.(string)
	}

	return
}

// Host is a Rancher host representation.
//
// swagger:model rancherHost
type Host struct {
	// the internal rancher uuid for this host
	// required: true
	// min: 1
	UUID string `json:"-"`
	// the hostname
	// required: true
	// min: 1
	Name string `json:"HostName"`
}

// UnmarshalJSON unmarshals the Rancher host struct
//
// The method is implemented in order to fudge the Rancher JSON to match
// the LetterCase used by Verint/KANA in their JMX beans, for consistency
// in this API. The JSON tags on the structs are what the end user sees.
func (h *Host) UnmarshalJSON(b []byte) (err error) {
	var data map[string]interface{}
	if err = json.Unmarshal(b, &data); err != nil {
		return err
	}

	h.UUID = data["uuid"].(string)
	h.Name = data["name"].(string)

	return
}

// NewMetadataCachingRepository creates a new metadataCachingRepository, an
// in-memory implementation of the Rancher package's Repository interface.
//
// Its purpose is to periodically call into Rancher's metadata service and
// populate data structures needed by this project such as the current list of
// Docker containers or Rancher hosts in the environment.
func NewMetadataCachingRepository(sc ClientService, cacheInterval time.Duration) (mcr Repository) {
	mcr = &metadataCachingRepository{
		containers:   []*Container{},
		containerMap: make(map[string]*Container),

		hosts:   []*Host{},
		hostMap: make(map[string]*Host),

		client: sc,
	}
	mcr.cachePopulateEvery(cacheInterval)
	return
}

type metadataCachingRepository struct {
	containers   []*Container
	containerMap map[string]*Container

	hosts   []*Host
	hostMap map[string]*Host

	// For making external calls to Rancher's metadata service
	client ClientService
}

// ContainerByName returns the Container in the repository identified by the given name.
func (mcr *metadataCachingRepository) ContainerByName(name string) (*Container, error) {
	if len(mcr.containerMap) == 0 {
		return nil, ErrContainerRepoEmpty
	} else if c, ok := mcr.containerMap[name]; !ok {
		return nil, ErrContainerNotFound
	} else {
		return c, nil
	}
}

// Containers returns all Containers found in the repository.
func (mcr *metadataCachingRepository) Containers() (cs []*Container, err error) {
	if cs = mcr.containers; len(cs) == 0 {
		err = ErrContainerRepoEmpty
	}
	return
}

// refreshContainers atomically replenishes the repository Containers cache.
func (mcr *metadataCachingRepository) refreshContainers() {
	cs, err := mcr.client.MetadataContainers()
	if err != nil {
		return
	}
	cm := make(map[string]*Container)
	for _, c := range cs {
		cm[c.Name] = c
	}

	// Synchronize this pair of updates
	ch := make(chan int)
	go func() {
		mcr.containerMap = cm
		mcr.containers = cs
		ch <- 0
	}()
	<-ch
}

// HostByUUID returns the Host in the repository identified by the given UUID
func (mcr *metadataCachingRepository) HostByUUID(uuid string) (*Host, error) {
	if len(mcr.hostMap) == 0 {
		return nil, ErrHostRepoEmpty
	} else if h, ok := mcr.hostMap[uuid]; !ok {
		return nil, ErrHostNotFound
	} else {
		return h, nil
	}
}

// Hosts returns all Hosts found in the repository
func (mcr *metadataCachingRepository) Hosts() (hs []*Host, err error) {
	if hs = mcr.hosts; len(hs) == 0 {
		err = ErrHostRepoEmpty
	}
	return
}

// refreshHosts atomically replenishes the repository Hosts cache
func (mcr *metadataCachingRepository) refreshHosts() {
	hs, err := mcr.client.MetadataHosts()
	if err != nil {
		return
	}

	hm := make(map[string]*Host)
	for _, h := range hs {
		hm[h.UUID] = h
	}

	// Synchronize this pair of updates
	ch := make(chan int)
	go func() {
		mcr.hostMap = hm
		mcr.hosts = hs
		ch <- 0
	}()
	<-ch
}

// cachePopulateEvery concurrently refreshes the caches every d Duration
func (mcr *metadataCachingRepository) cachePopulateEvery(d time.Duration) {
	// Refresh caches concurrently
	var wg sync.WaitGroup
	wg.Add(2)
	run := func(f func()) { defer wg.Done(); f() }
	go run(mcr.refreshHosts)
	go run(mcr.refreshContainers)
	wg.Wait()

	// Set the full host name on each container after refreshing
	cs, _ := mcr.Containers()
	for _, c := range cs {
		h, err := mcr.HostByUUID(c.Host.UUID)
		if err == nil {
			c.Host.Name = h.Name
		}
	}

	// Begin cache loop
	time.AfterFunc(d, func() { mcr.cachePopulateEvery(d) })
}
