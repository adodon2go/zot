//go:build metrics
// +build metrics

package monitoring

import (
	"encoding/json"
	"net/http"
	"path"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gopkg.in/resty.v1"

	"zotregistry.io/zot/errors"
	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/storage/cache"
)

const metricsNamespace = "zot"

var (
	httpConnRequests = promauto.NewCounterVec( //nolint: gochecknoglobals
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "http_requests_total",
			Help:      "Total number of http request in zot",
		},
		[]string{"method", "code"},
	)
	httpRepoLatency = promauto.NewSummaryVec( //nolint: gochecknoglobals
		prometheus.SummaryOpts{
			Namespace: metricsNamespace,
			Name:      "http_repo_latency_seconds",
			Help:      "Latency of serving HTTP requests",
		},
		[]string{"repo"},
	)
	httpMethodLatency = promauto.NewHistogramVec( //nolint: gochecknoglobals
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "http_method_latency_seconds",
			Help:      "Latency of serving HTTP requests",
			Buckets:   GetDefaultBuckets(),
		},
		[]string{"method"},
	)
	repoStorageBytes = promauto.NewGaugeVec( //nolint: gochecknoglobals
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "repo_storage_bytes",
			Help:      "Storage used per zot repo",
		},
		[]string{"repo"},
	)
	uploadCounter = promauto.NewCounterVec( //nolint: gochecknoglobals
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "repo_uploads_total",
			Help:      "Total number times an image was uploaded",
		},
		[]string{"repo"},
	)
	downloadCounter = promauto.NewCounterVec( //nolint: gochecknoglobals
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "repo_downloads_total",
			Help:      "Total number times an image was downloaded",
		},
		[]string{"repo"},
	)
	serverInfo = promauto.NewGaugeVec( //nolint: gochecknoglobals
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "info",
			Help:      "Server general information",
		},
		[]string{"commit", "binaryType", "goVersion", "version"},
	)
	storageLockLatency = promauto.NewHistogramVec( //nolint: gochecknoglobals
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "storage_lock_latency_seconds",
			Help:      "Latency of serving HTTP requests",
			Buckets:   GetStorageLatencyBuckets(),
		},
		[]string{"storageName", "lockType"},
	)
)

type metricServer struct {
	enabled     bool
	log         log.Logger
	cacheDriver cache.Cache
	url         string
}

func GetDefaultBuckets() []float64 {
	return []float64{.05, .5, 1, 5, 30, 60, 600}
}

func GetStorageLatencyBuckets() []float64 {
	return []float64{.001, .01, 0.1, 1, 5, 10, 15, 30, 60}
}

func NewMetricsServer(enabled bool, log log.Logger) MetricServer {
	return &metricServer{
		enabled: enabled,
		log:     log,
	}
}

func (ms *metricServer) SetCacheDriver(cd cache.Cache) {
	ms.cacheDriver = cd
}

// implementing the MetricServer interface.
func (ms *metricServer) SendMetric(mfunc interface{}) {
	if ms.enabled {
		mfn, ok := mfunc.(func())
		if !ok {
			ms.log.Error().Err(errors.ErrInvalidMetric).Msg("type conversion")

			return
		}

		mfn()
	}
}

func (ms *metricServer) ForceSendMetric(mfunc interface{}) {
	mfn, ok := mfunc.(func())
	if !ok {
		ms.log.Error().Err(errors.ErrInvalidMetric).Msg("type conversion")

		return
	}

	mfn()
}

func (ms *metricServer) ReceiveMetrics() interface{} {
	resp, err := resty.R().Get(ms.url)
	if err != nil && resp != nil && resp.StatusCode() == http.StatusOK {
		return resp.Body()
	}
	return nil
}

func (ms *metricServer) IsEnabled() bool {
	return ms.enabled
}

func (ms *metricServer) PersistCache() error {
	if ms.IsEnabled() {
		data := ms.ReceiveMetrics()
		m, err := json.Marshal(data)
		if err != nil {
			panic(err)
		}
		return ms.cacheDriver.PutMetrics(m)
	}

	return nil
}

func (ms *metricServer) SetURL(url string) {
	ms.url = url
}

func (ms *metricServer) RestoreFromCache() ([]byte, error) {
	if ms.IsEnabled() {
		return ms.cacheDriver.GetMetrics()
	}
	return []byte{}, nil
}

func IncHTTPConnRequests(ms MetricServer, lvalues ...string) {
	ms.SendMetric(func() {
		httpConnRequests.WithLabelValues(lvalues...).Inc()
	})
}

func ObserveHTTPRepoLatency(ms MetricServer, path string, latency time.Duration) {
	ms.SendMetric(func() {
		match := re.FindStringSubmatch(path)

		if len(match) > 1 {
			httpRepoLatency.WithLabelValues(match[1]).Observe(latency.Seconds())
		} else {
			httpRepoLatency.WithLabelValues("N/A").Observe(latency.Seconds())
		}
	})
}

func ObserveHTTPMethodLatency(ms MetricServer, method string, latency time.Duration) {
	ms.SendMetric(func() {
		httpMethodLatency.WithLabelValues(method).Observe(latency.Seconds())
	})
}

func IncDownloadCounter(ms MetricServer, repo string) {
	ms.SendMetric(func() {
		downloadCounter.WithLabelValues(repo).Inc()
	})
}

func SetStorageUsage(ms MetricServer, rootDir, repo string) {
	ms.SendMetric(func() {
		dir := path.Join(rootDir, repo)
		repoSize, err := GetDirSize(dir)

		if err == nil {
			repoStorageBytes.WithLabelValues(repo).Set(float64(repoSize))
		}
	})
}

func IncUploadCounter(ms MetricServer, repo string) {
	ms.SendMetric(func() {
		uploadCounter.WithLabelValues(repo).Inc()
	})
}

func SetServerInfo(ms MetricServer, lvalues ...string) {
	ms.ForceSendMetric(func() {
		serverInfo.WithLabelValues(lvalues...).Set(0)
	})
}

func ObserveStorageLockLatency(ms MetricServer, latency time.Duration, storageName, lockType string) {
	ms.SendMetric(func() {
		storageLockLatency.WithLabelValues(storageName, lockType).Observe(latency.Seconds())
	})
}
