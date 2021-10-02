// +build minimal

package api_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	zotapi "github.com/anuvu/zot/pkg/api"
	"github.com/anuvu/zot/pkg/exporter/api"
	"github.com/anuvu/zot/pkg/extensions/monitoring"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/resty.v1"
)

const (
	BaseURL   = "http://127.0.0.1:%s"
	SleepTime = 50 * time.Millisecond
)

func getFreePort() string {
	port, err := freeport.GetFreePort()
	if err != nil {
		panic(err)
	}
	return fmt.Sprint(port)
}

func TestNew(t *testing.T) {
	Convey("Make a new controller", t, func() {
		config := api.DefaultConfig()
		So(config, ShouldNotBeNil)
		So(api.NewController(config), ShouldNotBeNil)
	})
}

func isChannelDrained(ch chan prometheus.Metric) bool {
	time.Sleep(SleepTime)
	select {
	case _ = <-ch:
		return false
	default:
		return true
	}
}

func readDefaultMetrics(zc *api.ZotCollector, ch chan prometheus.Metric) {
	var metric dto.Metric

	pm := <-ch
	So(pm.Desc().String(), ShouldEqual, zc.MetricsDesc["zot_up"].String())

	pm.Write(&metric)
	fmt.Printf("Prometheus Metric: %f \n", *metric.Gauge.Value)
	So(*metric.Gauge.Value, ShouldEqual, 1)

	pm = <-ch
	So(pm.Desc().String(), ShouldEqual, zc.MetricsDesc["zot_info"].String())

	pm.Write(&metric)
	So(*metric.Gauge.Value, ShouldEqual, 0)
}

func TestNewExporter(t *testing.T) {
	Convey("Make an exporter controller", t, func() {
		exporterConfig := api.DefaultConfig()
		So(exporterConfig, ShouldNotBeNil)
		exporterPort := getFreePort()
		serverPort := getFreePort()
		exporterConfig.ZotExporter.Port = exporterPort
		dir, _ := ioutil.TempDir("", "metrics")
		exporterConfig.ZotExporter.Metrics.Path = strings.TrimPrefix(dir, "/tmp/")
		exporterConfig.ZotServer.Port = serverPort
		exporterController := api.NewController(exporterConfig)

		Convey("Start the zot exporter", func() {
			go func() {
				// this blocks
				exporterController.Run()
				So(nil, ShouldNotBeNil) // Fail the test in case zot exporter unexpectedly exits
			}()
			zc := api.GetZotCollector(exporterController)
			ch := make(chan prometheus.Metric)

			Convey("When zot server not running", func() {
				go func() {
					// this blocks
					zc.Collect(ch)
				}()
				// Read from the channel expected values
				pm := <-ch
				So(pm.Desc().String(), ShouldEqual, zc.MetricsDesc["zot_up"].String())

				var metric dto.Metric
				pm.Write(&metric)
				So(*metric.Gauge.Value, ShouldEqual, 0) // "zot_up=0" means zot server is not running

				// Check that no more data was written to the channel
				So(isChannelDrained(ch), ShouldEqual, true)
			})
			Convey("When zot server is running", func() {
				servercConfig := zotapi.NewConfig()
				So(servercConfig, ShouldNotBeNil)
				baseURL := fmt.Sprintf(BaseURL, serverPort)
				servercConfig.HTTP.Port = serverPort
				serverController := zotapi.NewController(servercConfig)
				So(serverController, ShouldNotBeNil)

				dir, err := ioutil.TempDir("", "exporter-test")
				So(err, ShouldBeNil)
				defer os.RemoveAll(dir)
				serverController.Config.Storage.RootDirectory = dir
				go func(c *zotapi.Controller) {
					// this blocks
					if err := c.Run(); err != http.ErrServerClosed {
						panic(err)
					}
				}(serverController)
				defer func(c *zotapi.Controller) {
					_ = c.Server.Shutdown(context.TODO())
				}(serverController)
				// wait till ready
				for {
					_, err := resty.R().Get(baseURL)
					if err == nil {
						break
					}
					time.Sleep(SleepTime)
				}

				// Side effect of calling this endpoint is that it will enable metrics
				resp, err := resty.R().Get(baseURL + "/v2/metrics")
				So(resp, ShouldNotBeNil)
				So(err, ShouldBeNil)
				So(resp.StatusCode(), ShouldEqual, 200)

				Convey("Collecting data: default metrics", func() {
					go func() {
						// this blocks
						zc.Collect(ch)
					}()
					readDefaultMetrics(zc, ch)
					So(isChannelDrained(ch), ShouldEqual, true)
				})
				Convey("Collecting data: Test init value & that increment works on Counters", func() {
					//Testing initial value of the counter to be 1 after first incrementation call
					monitoring.IncUploadCounter("testrepo")
					time.Sleep(SleepTime)

					go func() {
						// this blocks
						zc.Collect(ch)
					}()
					readDefaultMetrics(zc, ch)

					pm := <-ch
					So(pm.Desc().String(), ShouldEqual, zc.MetricsDesc["zot_repo_uploads_total"].String())

					var metric dto.Metric
					pm.Write(&metric)
					So(*metric.Counter.Value, ShouldEqual, 1)

					So(isChannelDrained(ch), ShouldEqual, true)

					//Testing that counter is incremented by 1
					monitoring.IncUploadCounter("testrepo")
					time.Sleep(50 * time.Millisecond)

					go func() {
						// this blocks
						zc.Collect(ch)
					}()
					readDefaultMetrics(zc, ch)

					pm = <-ch
					So(pm.Desc().String(), ShouldEqual, zc.MetricsDesc["zot_repo_uploads_total"].String())

					pm.Write(&metric)
					So(*metric.Counter.Value, ShouldEqual, 2)

					So(isChannelDrained(ch), ShouldEqual, true)
				})
				Convey("Collecting data: Test that concurent Counter increment requests works properly", func() {
					reqsSize := 100
					for i := 0; i < reqsSize; i++ {
						monitoring.IncDownloadCounter("dummyrepo")
					}
					time.Sleep(SleepTime)

					go func() {
						// this blocks
						zc.Collect(ch)
					}()
					readDefaultMetrics(zc, ch)

					pm := <-ch
					So(pm.Desc().String(), ShouldEqual, zc.MetricsDesc["zot_repo_uploads_total"].String())

					var metric dto.Metric
					pm.Write(&metric)
					So(*metric.Counter.Value, ShouldEqual, 2)

					pm = <-ch
					So(pm.Desc().String(), ShouldEqual, zc.MetricsDesc["zot_repo_downloads_total"].String())

					pm.Write(&metric)
					So(*metric.Counter.Value, ShouldEqual, reqsSize)

					So(isChannelDrained(ch), ShouldEqual, true)
				})
			})
		})
	})
}
