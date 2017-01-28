# rancher-management-service
> A service boilerplate for securely bridging the Rancher private network to perform environment management operations.

[![Golang Programming Language](https://img.shields.io/badge/language-Golang-blue.svg)](https://www.golang.org)
[![Build Status](https://travis-ci.org/martinbaillie/rancher-management-service.svg?branch=master)](https://travis-ci.org/martinbaillie/rancher-management-service)
[![Docker Automated build](https://img.shields.io/docker/automated/jrottenberg/ffmpeg.svg)](https://hub.docker.com/r/martinbaillie/rancher-management-service)

## Purpose

This project can be used as a basis for building out a service that securely bridges Rancher's private network and performs proxied runtime operations against other containers in the environment.

Example use cases:
- Discovering and aggregating container details based on behaviours using Rancher metadata.
- Managing JEE containers discovered in the environment using JMX/Jolokia (e.g. killing their stateful sessions, changing log levels, updating Java properties).
- Changing Rancher LB settings (e.g. HAProxy drain mode or alternate weightings for specific containers).

The project also strives to be a Golang service reference implementation and includes the sort of boilerplate features expected from an enterprise grade Golang service such as (but not exclusive to):
- Swagger spec generated from source code comments.
- Swagger UI bundled into and served from the single binary.
- Registration with:
    - Consul.
    - Eureka (`TODO`).
- Tracing with Zipkin.
- Instrumenting with Prometheus.
- Circuit breaking with Hystrix.
- Structured, leveled logging.
- Testing through:
    - Mocks.
    - Contracts (`TODO`).
- OAuth/JWTs (`TODO`).

And more generally, idiomatic Golang coding through showcasing:
- Best practice project layout.
- Boilerplate Makefiles, Dockerfiles.
- Separation of concerns like middlewares or transport mediums using progressive enhancement/decorator patterns.

## Disclaimer

Since I clone and extend this boilerplate in real-world business contexts it will always be opinionated in its choice of tooling, purely to slot in more easily beside whatever current $COMPANY's established stack is. Just fork and remove or change as required.

## Developing

#### Native Build
```bash
make
```
> Builds locally, referencing makevars like `GOOS` for cross-compilation hints. Linux and Darwin by default.
>
> In addition to compilation, the `all` target encompasses pulling build dependencies (like gb, go-bindata...) as well as go generation, testing, linting, vetting and formatting. Study the Makefile for full detail.

#### Docker
```bash
make docker-build
```
> Creates a golang build environment container (`Dockerfile_build`) and runs a local Linux build inside (`make` as above). The output binary is then put in a `FROM: scratch` container (`Dockerfile`) ready for tagging and pushing to a Docker registry. There are make targets for tagging and pushing if needed.
>
> Up-to-date CA certificates are also packaged into the scratch image for convenience.

## Testing
```bash
make test
```
> For integration testing, deploy an http proxy container (e.g. Squid, Tinyproxy) to bridge the Rancher environment's overlay network. The code obeys proxy vars:
>
> `env http_proxy=username:p%40ssword@rancher-host.corp:3128 ./rancher-management-service`

## Configuration
```bash
Usage of rancher-management-service:
  -debug
    	Turn on debug logging output
  -debug_addr string
    	Debug (pprof) bind address (default "0.0.0.0:8082")
  -http_addr string
    	HTTP transport bind address (default "0.0.0.0:8080")
  -http_basepath string
    	Basepath to serve the HTTP endpoints from (default "/rms/v1")
  -metadata_addr string
    	Rancher metadata service address (default "rancher-metadata.rancher.internal/latest")
  -metadata_interval duration
    	Duration between Rancher metadata cache calls (default 5m0s)
  -metrics_addr string
    	Metrics (Prometheus) transport bind address (default "0.0.0.0:8081")
  -zipkin_addr string
    	Enable Zipkin HTTP tracing to the provided address
```
