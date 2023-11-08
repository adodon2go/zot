package monitoring

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/scheduler"
	"zotregistry.io/zot/pkg/storage/cache"
)

var re = regexp.MustCompile(`\/v2\/(.*?)\/(blobs|tags|manifests)\/(.*)$`)

type MetricServer interface {
	SendMetric(interface{})
	// works like SendMetric, but adds the metric regardless of the value of 'enabled' field for MetricServer
	ForceSendMetric(interface{})
	ReceiveMetrics() interface{}
	IsEnabled() bool
	SetCacheDriver(cache.Cache) // for persistent storage
	PersistCache() error
	RestoreFromCache() ([]byte, error)
	SetURL(url string)
}

type SaveCacheGenerator struct {
	metrics  MetricServer
	done     bool
	log      log.Logger
	firstRun bool
}

func NewSaveCacheTaskGenerator(ms MetricServer, log log.Logger) *SaveCacheGenerator {
	return &SaveCacheGenerator{
		metrics:  ms,
		done:     false,
		log:      log,
		firstRun: true,
	}
}

func (gen *SaveCacheGenerator) Next() (scheduler.Task, error) {
	// first time we don't save cache for metrics because we need to restore it first (if available)
	if gen.firstRun {
		gen.log.Info().Msg("monitoring: restoring metrics cache")
	} else {
		gen.log.Info().Msg("monitoring: saving metrics cache")
	}
	return newSaveCacheTask(gen), nil
}

func (gen *SaveCacheGenerator) IsDone() bool {
	return gen.done
}

func (gen *SaveCacheGenerator) IsReady() bool {
	return true
}

func (gen *SaveCacheGenerator) Reset() {
	gen.done = false
}

type saveCacheTask struct {
	gen *SaveCacheGenerator
}

func newSaveCacheTask(gen *SaveCacheGenerator) *saveCacheTask {
	gen.log.Debug().Bool("firstRun", gen.firstRun).Msg("newSaveCacheTask()")
	return &saveCacheTask{gen}
}

func (sct *saveCacheTask) DoWork(ctx context.Context) error {
	var err error
	sct.gen.log.Debug().Bool("firstRun", sct.gen.firstRun).Msg("DoWork()")
	if sct.gen.firstRun {
		var data []byte
		sct.gen.firstRun = false
		data, err = sct.gen.metrics.RestoreFromCache()
		sct.gen.log.Debug().Str("data", string(data)).Msg("RestoreFromCache() output")
	} else {
		err = sct.gen.metrics.PersistCache()
		sct.gen.log.Debug().Err(err).Msg("PersistCache() returned")
	}
	sct.gen.done = true
	return err
}

func GetDirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}

		return err
	})

	return size, err
}
