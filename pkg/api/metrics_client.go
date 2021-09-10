package api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	zotErrors "github.com/anuvu/zot/errors"
	"github.com/anuvu/zot/pkg/log"
)

const (
	httpTimeout = 1 * time.Minute
)

// ZotMetricsConfig is used to configure the creation of a Node Exporter http client that will connect to a particular zot instance
type ZotMetricsConfig struct {
	// Address is the address of the zot http server
	Address string

	// Transport is the Transport to use for the http client.
	Transport *http.Transport

	// HttpClient is the client to use. Default will be
	// used if not provided.
	HttpClient *http.Client

	TLSConfig *TLSConfig // currently not used
}

type MetricsClient struct {
	headers http.Header
	config  ZotMetricsConfig
	Log     log.Logger
}

func DefaultConfig() *ZotMetricsConfig {
	config := &ZotMetricsConfig{
		Address:   "http://127.0.0.1:5000",
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	return config
}

// Creates a MetricsClient that can be used to retrive in memory metrics
// The new MetricsClient retrieved must be cached  and reused by the Node Exporter
// in order to prevent concurrent memory leaks
func NewMetricsClient(config *ZotMetricsConfig) (*MetricsClient, error) {
	// bootstrap the config
	defConfig := DefaultConfig()

	// TODO: Customize from config file
	logger := log.NewLogger("debug", "")

	if config.Address == "" {
		config.Address = defConfig.Address
	}

	if config.Transport == nil {
		config.Transport = defConfig.Transport
	}

	if config.HttpClient == nil {
		var verifyTLS bool
		if config.TLSConfig != nil {
			verifyTLS = true
		}
		host := getHostFromAddress(config.Address)
		config.HttpClient = newHTTPMetricsClient(verifyTLS, host)
	}

	return &MetricsClient{config: *config, headers: make(http.Header), Log: logger}, nil
}

func (mc *MetricsClient) GetMetrics() (*MetricsInfo, error) {
	metrics := &MetricsInfo{}
	_, err := mc.makeGETRequest(mc.config.Address+"/v2/metrics", metrics)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func (mc *MetricsClient) makeGETRequest(url string, resultsPtr interface{}) (http.Header, error) {
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, err
	}

	resp, err := mc.config.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, zotErrors.ErrUnauthorizedAccess
		}

		bodyBytes, _ := ioutil.ReadAll(resp.Body)

		return nil, errors.New(string(bodyBytes)) //nolint: goerr113
	}

	if err := json.NewDecoder(resp.Body).Decode(resultsPtr); err != nil {
		return nil, err
	}

	return resp.Header, nil
}

// format of expected address is something similar to http://localhost:5050
func getHostFromAddress(address string) string {
	parts := strings.SplitN(address, "://", 2)
	if len(parts) == 2 {
		parts = strings.SplitN(parts[1], ":", 2)
		if len(parts) == 2 {
			return parts[0]
		}
	}
	// TODO: Fix from configuration file: Spit address into host &port
	return "localhost"
}
func newHTTPMetricsClient(verifyTLS bool, host string) *http.Client {

	defaultTransport := http.DefaultTransport.(*http.Transport).Clone()
	defaultTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint: gosec

	return &http.Client{
		Timeout:   httpTimeout,
		Transport: defaultTransport,
	}

}
