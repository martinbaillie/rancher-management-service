// Copyright 2017 Martin Baillie <martin.t.baillie@gmail.com>.
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file or at:
// https://opensource.org/licenses/BSD-3-Clause

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	stdlog "log"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/namsral/flag"

	stdopentracing "github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/go-kit/kit/log"
	level "github.com/go-kit/kit/log/experimental_level"
	"github.com/go-kit/kit/metrics/prometheus"

	"github.com/martinbaillie/rancher-management-service/rancher"
	"github.com/martinbaillie/rancher-management-service/swagger"
)

// Linker-provided project/build information
var (
	projectName,
	projectVersion,
	buildTime,
	buildHash,
	buildUser string
)

func main() {
	// Behaviour
	const (
		defHTTPBasePath     = "/rms/v1"
		defHTTPAddr         = "0.0.0.0:8080"
		defMetricsAddr      = "0.0.0.0:8081"
		defDebugAddr        = "0.0.0.0:8082"
		defMetadataInterval = time.Duration(300) * time.Second
		defMetadataAddr     = "rancher-metadata.rancher.internal/latest"
	)
	var (
		// In keeping with 12 factor, all flags can also be set in the environment.
		// NOTE: do this by uppercasing the entire CLI flag e.g. HTTP_ADDR.
		debug            = flag.Bool("debug", false, "Turn on debug logging output")
		httpBasepath     = flag.String("http_basepath", defHTTPBasePath, "Basepath to serve the HTTP endpoints from")
		httpAddr         = flag.String("http_addr", defHTTPAddr, "HTTP transport bind address")
		metricsAddr      = flag.String("metrics_addr", defMetricsAddr, "Metrics (Prometheus) transport bind address")
		debugAddr        = flag.String("debug_addr", defDebugAddr, "Debug (pprof) bind address")
		zipkinAddr       = flag.String("zipkin_addr", "", "Enable Zipkin HTTP tracing to the provided address")
		metadataAddr     = flag.String("metadata_addr", defMetadataAddr, "Rancher metadata service address")
		metadataInterval = flag.Duration("metadata_interval", defMetadataInterval, "Duration between Rancher metadata cache calls")
	)
	flag.Parse()

	// Logging
	//
	// Our approach here is structured logging, and just
	// enough of it. Keep in mind that a lot of the time log statements
	// would be better served as Prometheus metrics or Zipkin traces.
	//
	// NOTE: Experimenting with levels, not sure if keeping.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stdout)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestamp)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)

		if *debug {
			// Show debug level log statements
			logger = level.New(logger, level.Allowed(level.AllowDebugAndAbove()))
		} else {
			logger = level.New(logger, level.Allowed(level.AllowInfoAndAbove()))
		}

		// Redirect stdlib logger to Go kit logger.
		stdlog.SetOutput(log.NewStdlibAdapter(logger))
	}
	level.Info(logger).Log("msg", "starting", "service", projectName, "version", projectVersion, "debug", *debug)
	level.Info(logger).Log("build_time", buildTime, "build_commit", buildHash, "build_user", buildUser)
	defer level.Info(logger).Log("msg", "stopping", "service", projectName)

	// Context plumbing and interrupt/error channels
	ctx := context.Background()
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// Tracing (Zipkin)
	var tracer stdopentracing.Tracer
	{
		if *zipkinAddr != "" {
			logger := log.NewContext(logger).With("tracer", "Zipkin")
			level.Info(logger).Log("addr", *zipkinAddr)
			collector, err := zipkin.NewHTTPCollector(
				*zipkinAddr,
				zipkin.HTTPLogger(level.Error(logger)),
			)
			if err != nil {
				level.Error(logger).Log("err", err)
				os.Exit(1)
			}
			tracer, err = zipkin.NewTracer(
				zipkin.NewRecorder(collector, true, *httpAddr, projectName),
			)
			if err != nil {
				level.Error(logger).Log("err", err)
				os.Exit(1)
			}
		} else {
			logger := log.NewContext(logger).With("tracer", "none")
			// No-ops
			level.Info(logger).Log()
			tracer = stdopentracing.GlobalTracer()
		}
	}

	// Instrumentation (Prometheus)
	var (
		prometheusNamespace = strings.Replace(projectName, "-", "_", -1)
		prometheusFieldKeys = []string{"method"}
	)

	// Client Endpoints
	//
	// Client Services use these Client Endpoints for 3rd party integrations
	// e.g. Rancher metadata service, Jolokia JMX-over-HTTP (JVM) etc.
	//
	// NOTE: These endpoints are decorated with tracing and circuit breaking
	var rcses rancher.ClientEndpoints
	rcses = rancher.NewClientEndpoints(ctx, metadataServiceURLFromStr(*metadataAddr), tracer)

	// Client Services
	//
	// Wraps Client Endpoints to provide service layers to external
	// integrations. Client Services are used by internal package business
	// logic and functionality when they need to talk to 3rd parties.
	var rcs rancher.ClientService
	{
		// Create the service and provide the endpoints to use
		rcs = rancher.NewClientService(ctx, rcses)

		// Decorate the service with logging and instrumentation
		rcs = rancher.NewClientServiceLogger(
			log.NewContext(logger).With("component", "rancher"),
			rcs,
		)
		rcs = rancher.NewClientServiceInstrumenter(
			// Transport related metrics
			prometheus.NewCounterFrom(stdprometheus.CounterOpts{
				Namespace: prometheusNamespace,
				Subsystem: "rancher_client_service",
				Name:      "request_count",
				Help:      "Number of requests received.",
			}, prometheusFieldKeys),
			prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
				Namespace: prometheusNamespace,
				Subsystem: "rancher_client_service",
				Name:      "request_latency_microseconds",
				Help:      "Total duration of requests in microseconds.",
			}, prometheusFieldKeys),
			// Business related metrics
			prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: "rancher_client_service",
				Name:      "containers",
				Help:      "Number of containers in the Rancher environment.",
			}, prometheusFieldKeys),
			prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: "rancher_client_service",
				Name:      "hosts",
				Help:      "Number of hosts in the Rancher environment.",
			}, prometheusFieldKeys),
			rcs,
		)
	}

	// Server Services
	//
	// Wrap internal package business logic and functionality into service
	// layers which are in turn used by Server Endpoints.
	//
	// NOTE: Opposite of Client Services. These are used _by_ Server Endpoints
	// to serve functionality to consumers.
	var rss rancher.ServerService
	{
		// Create the service
		rss = rancher.NewServerService(
			rancher.NewMetadataCachingRepository(rcs, *metadataInterval),
		)

		// Decorate the service with logging and instrumentation
		rss = rancher.NewServerServiceLogger(
			log.NewContext(logger).With("component", "rancher"),
			rss,
		)
		rss = rancher.NewServerServiceInstrumenter(
			// Transport related metrics
			prometheus.NewCounterFrom(stdprometheus.CounterOpts{
				Namespace: prometheusNamespace,
				Subsystem: "rancher_server_service",
				Name:      "request_count",
				Help:      "Number of requests received.",
			}, prometheusFieldKeys),
			prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
				Namespace: prometheusNamespace,
				Subsystem: "rancher_server_service",
				Name:      "request_latency_microseconds",
				Help:      "Total duration of requests in microseconds.",
			}, prometheusFieldKeys),
			rss,
		)
	}

	// Server Endpoints
	//
	// These endpoints make use of Server Services to present internal package
	// business logic and functionality to consumers. They can be used by
	// various transports to offer up APIs.
	//
	// NOTE: Endpoints split from transport allows for transport mediums other
	// than JSON-over-HTTP e.g. gRPC/Thrift.
	//
	// NOTE: These endpoints are decorated with tracing
	var rses rancher.ServerEndpoints
	rses = rancher.NewServerEndpoints(rss, tracer)

	// HTTP transport
	go func() {
		logger := log.NewContext(logger).With("transport", "HTTP")

		// Create the router
		r := mux.NewRouter().StrictSlash(true)

		// Add Rancher handlers to router
		var rhs rancher.HTTPHandlers
		rhs = rancher.MakeHTTPHandlers(ctx, rses, tracer, logger)
		r.Methods("GET").Path(*httpBasepath + "/containers").Handler(rhs.Containers)
		r.Methods("GET").Path(*httpBasepath + "/containers/{name}").Handler(rhs.Container)

		// TODO: Jolokia handlers
		// TODO: JBoss handlers
		// TODO: HAProxy handlers

		// Add Swagger handlers to router
		swaggerPath := *httpBasepath + "/swagger-ui"
		swagger := swagger.NewSwaggerUI(swaggerPath)
		r.PathPrefix(swaggerPath).Handler(swagger)

		// Log every 404, even those not covered by our pre-defined routes
		r.NotFoundHandler = notFoundLogger(logger)

		// Further decorate the router with useful HTTP middlewares
		var rmws http.Handler = r
		rmws = handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(rmws)
		rmws = handlers.CompressHandler(rmws)
		rmws = handlers.ProxyHeaders(rmws)
		rmws = handlers.RecoveryHandler(handlers.RecoveryLogger(wrapLogger{level.Error(logger)}))(rmws)

		level.Info(logger).Log("msg", "started", "addr", *httpAddr, "base_path", *httpBasepath)
		errc <- http.ListenAndServe(*httpAddr, rmws)
	}()

	// TODO: gRPC transport
	// TODO: Thrift transport

	// Metrics transport
	go func() {
		logger := log.NewContext(logger).With("transport", "Metrics")

		r := mux.NewRouter()
		r.Handle("/metrics", stdprometheus.Handler())

		level.Info(logger).Log("msg", "started", "addr", *metricsAddr, "base_path", "/metrics")
		errc <- http.ListenAndServe(*metricsAddr, r)
	}()

	// Debug transport
	go func() {
		logger := log.NewContext(logger).With("transport", "Debug")

		r := mux.NewRouter()
		r.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		r.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		r.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		r.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		r.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

		level.Info(logger).Log("msg", "started", "addr", *debugAddr, "base_path", "/debug")
		errc <- http.ListenAndServe(*debugAddr, r)
	}()

	// Run!
	level.Info(logger).Log("msg", <-errc)
}

func metadataServiceURLFromStr(metadataServiceStr string) (metadataServiceURL *url.URL) {
	metadataServiceURL, err := url.Parse(metadataServiceStr)
	if err != nil {
		panic(err)
	}

	if metadataServiceURL.Scheme == "" {
		// Rancher.metadata is usually http
		metadataServiceURL.Scheme = "http"
	}
	return
}

// Useful error logging helpers
func notFoundLogger(logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		level.Error(logger).Log("err", http.StatusText(http.StatusNotFound), "url", r.URL)
		w.WriteHeader(http.StatusNotFound)
	})
}

// wrapLogger wraps a Go kit logger so we can use it as the logging service for
// Gorilla middlewares like the recovery handler.
type wrapLogger struct {
	log.Logger
}

func (logger wrapLogger) Println(args ...interface{}) {
	logger.Log("msg", fmt.Sprint(args...))
}
