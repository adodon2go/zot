//go:build metrics
// +build metrics

package extensions

import (
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"zotregistry.io/zot/pkg/api/config"
	"zotregistry.io/zot/pkg/extensions/monitoring"
	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/scheduler"
)

func EnableMetricsExtension(config *config.Config, metrics monitoring.MetricServer, sch *scheduler.Scheduler, log log.Logger) {
	if config.IsMetricsEnabled() {
		if config.Extensions.Metrics.Prometheus != nil &&
			config.Extensions.Metrics.Prometheus.Path == "" {
			config.Extensions.Metrics.Prometheus.Path = "/metrics"

			log.Warn().Msg("Prometheus instrumentation Path not set, changing to '/metrics'.")
		}
		go func() {
			// periodically save metrics cache
			time.Sleep(15 * time.Second)
			gen := monitoring.NewSaveCacheTaskGenerator(metrics, log)
			sch.SubmitGenerator(gen, time.Duration(1*time.Minute), scheduler.LowPriority)
		}()
	} else {
		log.Info().Msg("Metrics config not provided, skipping Metrics config update")
	}
}

func SetupMetricsRoutes(config *config.Config, router *mux.Router,
	authnFunc, authzFunc mux.MiddlewareFunc, log log.Logger, metrics monitoring.MetricServer,
) {
	log.Info().Msg("setting up metrics routes")

	if config.IsMetricsEnabled() {
		extRouter := router.PathPrefix(config.Extensions.Metrics.Prometheus.Path).Subrouter()
		extRouter.Use(authnFunc)
		extRouter.Use(authzFunc)
		extRouter.Methods("GET").Handler(promhttp.Handler())
	}
}
