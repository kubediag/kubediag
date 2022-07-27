package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"

	"github.com/go-logr/logr"
	api "github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	PrometheusQuerierAddress    = "prometheus_querier.address"
	PrometheusQuerierExpression = "prometheus_querier.expression"
	PrometheusQuerierTimeFrom   = "prometheus_querier.timeFrom"
	PrometheusQuerierTimeTo     = "prometheus_querier.timeTo"
	PrometheusQuerierStep       = "prometheus_querier.step"

	PrometheusQuerierResult = "prometheus_querier.result"
)

var (
	varRegex  *regexp.Regexp
	SEPERATOR string
)

func init() {
	varRegex = regexp.MustCompile(`(\$\{.+?\})`)
	SEPERATOR = ";"
}

// PrometheusQuerier queries time series data from prometheus.
type PrometheusQuerier struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// prometheusQuerierEnabled indicates whether prometheusQuerier is enabled.
	prometheusQuerierEnabled bool
}

// PrometheusQuery contains parameters required by PrometheusQuerier.
type PrometheusQuery struct {
	Address   string
	Expr      string
	TimeFrom  string
	TimeTo    string
	Step      string
	Variables map[string]string
}

type QueryResult struct {
	Query  string       `json:"query"`
	Result model.Matrix `json:"result"`
}

// NewPrometheusQuerier creates a new PrometheusQuerier.
func NewPrometheusQuerier(
	ctx context.Context,
	logger logr.Logger,
	prometheusQuerierEnabled bool,
) processors.Processor {
	return &PrometheusQuerier{
		Context:                  ctx,
		Logger:                   logger,
		prometheusQuerierEnabled: prometheusQuerierEnabled,
	}
}

// Handler handles http requests for prometheus querier.
func (q *PrometheusQuerier) Handler(w http.ResponseWriter, r *http.Request) {
	if !q.prometheusQuerierEnabled {
		http.Error(w, "prometheus querier is not enabled", http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		q.Info("handle POST request")
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			q.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Init PrometheusQuery
		query := PrometheusQuery{
			Address:   contexts[PrometheusQuerierAddress],
			Expr:      contexts[PrometheusQuerierExpression],
			TimeFrom:  contexts[PrometheusQuerierTimeFrom],
			TimeTo:    contexts[PrometheusQuerierTimeTo],
			Step:      contexts[PrometheusQuerierStep],
			Variables: contexts,
		}

		queryResults, err := query.query(q.Context)
		if err != nil {
			q.Error(err, "failed to query prometheus")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(queryResults)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal prometheus query results: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[PrometheusQuerierResult] = string(raw)
		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal prometheus querier result: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

func (query *PrometheusQuery) query(ctx context.Context) (results []QueryResult, err error) {
	// Split expression with seperator
	exprs := strings.Split(query.Expr, SEPERATOR)
	for i, expr := range exprs {
		// Replace all variables
		exprs[i] = query.interpolateExpression(expr)
	}

	v1api, err := getClient(query.Address)
	if err != nil {
		return results, err
	}

	for _, expr := range exprs {
		// If have a TimeFrom for the query, do range query
		if query.TimeFrom != "" {
			result, _, err := query.rangeQuery(ctx, v1api, expr)
			if err != nil {
				return results, err
			}

			results = append(results, QueryResult{
				Query:  expr,
				Result: result,
			})
		} else {
			result, _, err := query.instantQuery(ctx, v1api, expr)
			if err != nil {
				return results, err
			}

			results = append(results, QueryResult{
				Query:  expr,
				Result: result,
			})
		}

	}

	return results, err
}

// rangeQuery performs a range query
func (query *PrometheusQuery) rangeQuery(ctx context.Context, api apiv1.API, expr string) (model.Matrix, apiv1.Warnings, error) {
	r, err := query.getRange()
	if err != nil {
		return nil, nil, err
	}

	// Execute query
	result, warnings, err := api.QueryRange(ctx, expr, r)
	if err != nil {
		return nil, warnings, err
	}

	if result, ok := result.(model.Matrix); ok {
		return result, warnings, err
	} else {
		return nil, warnings, fmt.Errorf("didn't receive result from a range query")
	}
}

// instantQuery performs a instant query
func (query *PrometheusQuery) instantQuery(ctx context.Context, api apiv1.API, expr string) (model.Matrix, apiv1.Warnings, error) {
	// Parse end time
	time, err := parseTimeTo(query.TimeTo)
	if err != nil {
		return nil, nil, err
	}

	// Execute query
	result, warnings, err := api.Query(ctx, expr, time)
	if err != nil {
		return nil, warnings, err
	}

	if result, ok := result.(model.Matrix); ok {
		return result, warnings, err
	} else {
		return nil, warnings, fmt.Errorf("didn't receive result from a instant query")
	}
}

// interpolateExpression replaces all variables in expression.
func (query *PrometheusQuery) interpolateExpression(expression string) string {
	interpolated := varRegex.ReplaceAllStringFunc(expression, func(match string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		replacement, exists := query.Variables[key]
		if exists {
			return replacement
		}
		return match
	})

	return interpolated
}

// getClient returns a prometheus api client.
func getClient(address string) (apiv1.API, error) {
	cfg := api.Config{
		Address: address,
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return apiv1.NewAPI(client), nil
}

// parseTimeFrom parses timeFrom parameter.
func parseTimeFrom(timeFrom string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, timeFrom); err == nil {
		return t, nil
	} else if d, err := time.ParseDuration(timeFrom); err == nil {
		return time.Now().Add(-d), nil
	} else {
		err = fmt.Errorf("unable to parse range start time, %v", err)
		return time.Time{}, err
	}
}

// parseTimeFrom parses timeTo parameter.
func parseTimeTo(timeTo string) (time.Time, error) {
	if timeTo == "now" || timeTo == "" {
		return time.Now(), nil
	}
	t, err := time.Parse(time.RFC3339, timeTo)
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing range end time, %v", err)
	}
	return t, nil
}

// getRange creates a prometheus range from the provided PrometheusQuery options
func (query *PrometheusQuery) getRange() (r apiv1.Range, err error) {
	// Parse start time
	r.Start, err = parseTimeFrom(query.TimeFrom)
	if err != nil {
		return r, err
	}
	// Set up default step
	r.Step = time.Minute
	// Parse time.Duration if user provided step value
	if query.Step != "" {
		r.Step, err = time.ParseDuration(query.Step)
		if err != nil {
			return r, err
		}
	}
	// Parse end time
	r.End, err = parseTimeTo(query.TimeTo)
	if err != nil {
		return r, err
	}

	return r, err
}
