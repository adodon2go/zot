package api

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/anuvu/zot/pkg/log"
	"github.com/gorilla/mux"
)

var MetricsEnabled bool
var LastMetricsCheck time.Time

const (
	HttpConnRequests = "zot.http.requests"
)

// GaugeValue stores one value that is updated as time goes on, such as
// the amount of memory allocated.
type GaugeValue struct {
	Name   string
	Value  float32
	Labels map[string]string
}

// SampledValue stores info about a metric that is incremented over time,
// such as the number of requests to an HTTP endpoint.
type SampledValue struct {
	Name        string
	Count       int
	Sum         float64
	LabelNames  []string
	LabelValues []string
}

type MetricsInfo struct {
	mutex    *sync.RWMutex
	Gauges   []GaugeValue
	Counters []SampledValue
	Samples  []SampledValue
}

var InMemoryMetrics *MetricsInfo
var zotCounterList map[string][]string

func init() {
	// contains a map with key=CounterName and value=CounterLabels
	zotCounterList = map[string][]string{
		HttpConnRequests: []string{"method", "code"},
	}

	InMemoryMetrics = &MetricsInfo{
		mutex:    &sync.RWMutex{},
		Gauges:   make([]GaugeValue, 0),
		Counters: make([]SampledValue, 0),
		Samples:  make([]SampledValue, 0),
	}
}

// SessionMetrics updates in memory counters if MetricsEnabled (triggered by a node exporter that scrapes periodically metrics).
func SessionMetrics(log log.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var start time.Time
			// disable metrics if node exporting is not running (scraping didn't happen for more than 5 minutes)
			if MetricsEnabled { //
				if time.Now().Sub(LastMetricsCheck) > (300 * time.Second) {
					MetricsEnabled = false
					fmt.Println("!!! Metrics is disabled !!!")
				}
			}
			if MetricsEnabled {
				// Start timer
				start = time.Now()
				sw := statusWriter{ResponseWriter: w}

				// Process request
				next.ServeHTTP(&sw, r)

				// Stop timer
				end := time.Now()
				latency := end.Sub(start)
				if latency > time.Minute {
					// Truncate in a golang < 1.8 safe way
					latency -= latency % time.Second
				}
				method := r.Method
				statusCode := sw.status
				Counter(HttpConnRequests,
					[]string{"method", "code"},
					[]string{method, strconv.Itoa(statusCode)}, log).Inc()
			} else {
				next.ServeHTTP(w, r)
			}

			//metrics.HttpConnRequests.WithLabelValues(method, strconv.Itoa(statusCode)).Inc()
			/*re := regexp.MustCompile("\\/v2\\/(.*?)\\/(blobs|tags|manifests)\\/(.*)$")
			match := re.FindStringSubmatch(path)
			if len(match) > 1 {
				metrics.HttpServeLatency.WithLabelValues(match[1]).Observe(latency.Seconds())
			} else {
				metrics.HttpServeLatency.WithLabelValues("N/A").Observe(latency.Seconds())
			} */
		})
	}
}

// For Counters with no value we can send nil as LabelNames & LabelValues (equivalent of )
func Counter(name string, labelNames []string, labelValues []string, log log.Logger) *SampledValue {
	var sv SampledValue
	// Sanity Checks
	kLabels, ok := zotCounterList[name] // known label names for the 'name' counter
	if !ok {
		goto error
	}
	if len(labelNames) != len(labelValues) ||
		len(labelNames) != len(zotCounterList[name]) {
		goto error
	}
	// The list of label names defined in init() for the counter must match what was provided in labelNames
	for i, label := range labelNames {
		if label != kLabels[i] {
			goto error
		}
	}
	for i, sv := range InMemoryMetrics.Counters {
		if sv.Name == name {
			if labelNames == nil && labelValues == nil {
				//found the sampled values
				return &InMemoryMetrics.Counters[i]
			}
			if len(labelValues) == len(sv.LabelValues) {
				found := true
				for i, v := range sv.LabelValues {
					if v != labelValues[i] {
						found = false
						break
					}
				}
				if found {
					return &InMemoryMetrics.Counters[i]
				}
			}
		}
	}
	sv = SampledValue{
		Name:        name,
		LabelNames:  labelNames,
		LabelValues: labelValues,
	}
	// The Counter/SampledValue still not found: create one and return
	InMemoryMetrics.Counters = append(InMemoryMetrics.Counters, sv)
	return &InMemoryMetrics.Counters[len(InMemoryMetrics.Counters)-1]
error:
	log.Fatal().Msg("Counter sanity check failed")
	// The last thing we want is to panic/stop the server due to instrumentation:
	// thus return a dummy metric address that can be incremented and let Go's gc do its job
	return &sv
}

func (sv *SampledValue) Inc() {
	go func() {
		InMemoryMetrics.mutex.Lock()
		sv.Count++
		InMemoryMetrics.mutex.Unlock()
	}()
}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}

	n, err := w.ResponseWriter.Write(b)
	w.length += n

	return n, err
}
