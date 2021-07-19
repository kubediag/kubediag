package log

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	elasticsearch "github.com/elastic/go-elasticsearch/v7"
	"github.com/go-logr/logr"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

const (
	ElasticsearchAddress  = "param.collector.log.elasticsearch.address"
	ElasticsearchUsername = "param.collector.log.elasticsearch.username"
	ElasticsearchPassword = "param.collector.log.elasticsearch.password"
	ElasticsearchIndex    = "param.collector.log.elasticsearch.index"
	ElasticsearchMatch    = "param.collector.log.elasticsearch.match"
	ElasticsearchTimeFrom = "param.collector.log.elasticsearch.timeFrom"
	ElasticsearchTimeTo   = "param.collector.log.elasticsearch.timeTo"

	ContextKeyElasticsearchResultHits = "collector.log.elasticsearch.result.hits"
)

// elasticsearchCollector query log information from elasticsearch.
type elasticsearchCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// elasticsearchCollectorEnabled indicates whether elasticsearchCollector is enabled.
	elasticsearchCollectorEnabled bool
}

// NewElasticsearchCollector creates a new elasticsearchCollector.
func NewElasticsearchCollector(ctx context.Context,
	logger logr.Logger,
	elasticsearchCollectorEnabled bool,
) processors.Processor {
	return &elasticsearchCollector{
		Context:                       ctx,
		Logger:                        logger,
		elasticsearchCollectorEnabled: elasticsearchCollectorEnabled,
	}
}

// Handler handles http requests for collecting elasticsearch log info.
func (ec *elasticsearchCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !ec.elasticsearchCollectorEnabled {
		http.Error(w, fmt.Sprintf("elasticsearch collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		ec.Info("handle POST request")
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			ec.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var address []string
		if value, ok := contexts[ElasticsearchAddress]; !ok {
			http.Error(w, fmt.Sprintf("must specify elasticsearch address."), http.StatusBadRequest)
			return
		} else {
			address = strings.Split(value, ",")
		}
		var match string
		if value, ok := contexts[ElasticsearchMatch]; !ok {
			http.Error(w, fmt.Sprintf("must specify elasticsearch query words."), http.StatusBadRequest)
			return
		} else {
			match = value
		}
		username := contexts[ElasticsearchUsername]
		password := contexts[ElasticsearchPassword]
		if strings.HasPrefix(address[0], "https") && (username == "" || password == "") {
			http.Error(w, fmt.Sprintf("must specify username and password when elasticsearch address is https."), http.StatusBadRequest)
			return
		}
		index := contexts[ElasticsearchIndex]
		timeFrom := contexts[ElasticsearchTimeFrom]
		timeTo := contexts[ElasticsearchTimeTo]

		hits, err := ec.runElasticsearchQuery(address, username, password, index, match, timeFrom, timeTo)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to run elasticsearch collector: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyElasticsearchResultHits] = hits
		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal elasticsearch collector results: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// runElasticsearchQuery do query in elasticsearch.
func (ec *elasticsearchCollector) runElasticsearchQuery(address []string, username, password, index, match, timeFrom, timeTo string) (string, error) {
	esClient, err := ec.initElasticsearchClient(address, username, password)
	if err != nil {
		return "", err
	}

	// Build the request body.
	query, err := buildQueryBody(match, timeFrom, timeTo)
	if err != nil {
		return "", err
	}

	// Perform the search request for the indexed documents.
	res, err := esClient.Search(
		esClient.Search.WithContext(context.Background()),
		esClient.Search.WithIndex(index),
		esClient.Search.WithBody(query),
		esClient.Search.WithTrackTotalHits(true),
		esClient.Search.WithPretty(),
	)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			ec.Error(err, "error parsing the response body")
			return "", err
		} else {
			// Return the response status and error information.
			return "", fmt.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"])
		}
	}

	var data map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}
	// Log the response status, number of results, and request duration.
	ec.Info(
		res.Status(),
		"hits", int(data["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)),
		"took", int(data["took"].(float64)),
	)
	hits, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(hits), nil
}

// initElasticsearchClient init elasticsearch client from params.
func (ec *elasticsearchCollector) initElasticsearchClient(address []string, username, password string) (*elasticsearch.Client, error) {
	// set the InsecureSkipVerify to true in case of self-signed certificates.
	transport := http.DefaultTransport
	transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	cfg := elasticsearch.Config{
		Username:  username,
		Password:  password,
		Addresses: address,
		Transport: transport,
	}
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init elasticsearch client: %v", err)
	}
	res, err := esClient.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get elasticsearch client info: %v", err)
	}

	// Check response status
	if res.IsError() {
		return nil, fmt.Errorf("failed to check elasticsearch client response status: %s", res.String())
	}

	var elasticInfo map[string]interface{}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&elasticInfo); err != nil {
		return nil, err
	}
	// Log client and server version numbers.
	ec.Info("client info", "version", elasticsearch.Version)
	ec.Info("server info", "version", elasticInfo["version"].(map[string]interface{})["number"])

	return esClient, nil
}

// buildQueryBody combines keywords into the language elasticsearch can understand.
func buildQueryBody(match, timeFrom, timeTo string) (*bytes.Buffer, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"match": map[string]interface{}{
							"message": match,
						},
					},
				},
			},
		},
	}
	if timeFrom != "" && timeTo != "" {
		queryRange := map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": timeFrom,
					"lte": timeTo,
				},
			},
		}
		queryMatch := query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]map[string]interface{})
		queryMatch = append(queryMatch, queryRange)
		query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = queryMatch
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("failed to encode query body: %v", err)
	}

	return &buf, nil
}
