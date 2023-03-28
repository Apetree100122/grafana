package historian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/ngalert/eval"
	"github.com/grafana/grafana/pkg/services/ngalert/metrics"
	"github.com/grafana/grafana/pkg/services/ngalert/state"
	history_model "github.com/grafana/grafana/pkg/services/ngalert/state/historian/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/http/client"
)

func TestRemoteLokiBackend(t *testing.T) {
	t.Run("statesToStreams", func(t *testing.T) {
		t.Run("skips non-transitory states", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{State: eval.Normal})

			res := statesToStreams(rule, states, nil, l)

			require.Empty(t, res)
		})

		t.Run("maps evaluation errors", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{State: eval.Error, Error: fmt.Errorf("oh no")})

			res := statesToStreams(rule, states, nil, l)

			entry := requireSingleEntry(t, res)
			require.Contains(t, entry.Error, "oh no")
		})

		t.Run("maps NoData results", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{State: eval.NoData})

			res := statesToStreams(rule, states, nil, l)

			_ = requireSingleEntry(t, res)
		})

		t.Run("produces expected stream identifier", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{
				State:  eval.Alerting,
				Labels: data.Labels{"a": "b"},
			})

			res := statesToStreams(rule, states, nil, l)

			require.Len(t, res, 1)
			exp := map[string]string{
				StateHistoryLabelKey: StateHistoryLabelValue,
				"folderUID":          rule.NamespaceUID,
				"group":              rule.Group,
				"orgID":              fmt.Sprint(rule.OrgID),
				"ruleUID":            rule.UID,
				"a":                  "b",
			}
			require.Equal(t, exp, res[0].Stream)
		})

		t.Run("groups streams based on combined labels", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := []state.StateTransition{
				{
					PreviousState: eval.Normal,
					State: &state.State{
						State:  eval.Alerting,
						Labels: data.Labels{"a": "b"},
					},
				},
				{
					PreviousState: eval.Normal,
					State: &state.State{
						State:  eval.Alerting,
						Labels: data.Labels{"a": "b"},
					},
				},
				{
					PreviousState: eval.Normal,
					State: &state.State{
						State:  eval.Alerting,
						Labels: data.Labels{"c": "d"},
					},
				},
			}

			res := statesToStreams(rule, states, nil, l)

			require.Len(t, res, 2)
			sort.Slice(res, func(i, j int) bool { return len(res[i].Values) > len(res[j].Values) })
			require.Contains(t, res[0].Stream, "a")
			require.Len(t, res[0].Values, 2)
			require.Contains(t, res[1].Stream, "c")
			require.Len(t, res[1].Values, 1)
		})

		t.Run("excludes private labels", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{
				State:  eval.Alerting,
				Labels: data.Labels{"__private__": "b"},
			})

			res := statesToStreams(rule, states, nil, l)

			require.Len(t, res, 1)
			require.NotContains(t, res[0].Stream, "__private__")
		})

		t.Run("includes instance labels in log line", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{
				State:  eval.Alerting,
				Labels: data.Labels{"statelabel": "labelvalue"},
			})

			res := statesToStreams(rule, states, nil, l)

			entry := requireSingleEntry(t, res)
			require.Contains(t, entry.InstanceLabels, "statelabel")
		})

		t.Run("does not include labels other than instance labels in log line", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{
				State: eval.Alerting,
				Labels: data.Labels{
					"statelabel": "labelvalue",
					"labeltwo":   "labelvalue",
					"labelthree": "labelvalue",
				},
			})

			res := statesToStreams(rule, states, nil, l)

			entry := requireSingleEntry(t, res)
			require.Len(t, entry.InstanceLabels, 3)
		})

		t.Run("serializes values when regular", func(t *testing.T) {
			rule := createTestRule()
			l := log.NewNopLogger()
			states := singleFromNormal(&state.State{
				State:  eval.Alerting,
				Values: map[string]float64{"A": 2.0, "B": 5.5},
			})

			res := statesToStreams(rule, states, nil, l)

			entry := requireSingleEntry(t, res)
			require.NotNil(t, entry.Values)
			require.NotNil(t, entry.Values.Get("A"))
			require.NotNil(t, entry.Values.Get("B"))
			require.InDelta(t, 2.0, entry.Values.Get("A").MustFloat64(), 1e-4)
			require.InDelta(t, 5.5, entry.Values.Get("B").MustFloat64(), 1e-4)
		})
	})
}

func TestMerge(t *testing.T) {
	testCases := []struct {
		name         string
		res          QueryRes
		ruleID       string
		expectedTime []time.Time
	}{
		{
			name: "Should return values from multiple streams in right order",
			res: QueryRes{
				Data: QueryData{
					Result: []Stream{
						{
							Stream: map[string]string{
								"current": "pending",
							},
							Values: [][2]string{
								{"1", `{"schemaVersion": 1, "previous": "normal", "current": "pending", "values":{"a": "b"}}`},
							},
						},
						{
							Stream: map[string]string{
								"current": "firing",
							},
							Values: [][2]string{
								{"2", `{"schemaVersion": 1, "previous": "pending", "current": "firing", "values":{"a": "b"}}`},
							},
						},
					},
				},
			},
			ruleID: "123456",
			expectedTime: []time.Time{
				time.Unix(0, 1),
				time.Unix(0, 2),
			},
		},
		{
			name: "Should handle empty values",
			res: QueryRes{
				Data: QueryData{
					Result: []Stream{
						{
							Stream: map[string]string{
								"current": "normal",
							},
							Values: [][2]string{},
						},
					},
				},
			},
			ruleID:       "123456",
			expectedTime: []time.Time{},
		},
		{
			name: "Should handle multiple values in one stream",
			res: QueryRes{
				Data: QueryData{
					Result: []Stream{
						{
							Stream: map[string]string{
								"current": "normal",
							},
							Values: [][2]string{
								{"1", `{"schemaVersion": 1, "previous": "firing", "current": "normal", "values":{"a": "b"}}`},
								{"2", `{"schemaVersion": 1, "previous": "firing", "current": "normal", "values":{"a": "b"}}`},
							},
						},
						{
							Stream: map[string]string{
								"current": "firing",
							},
							Values: [][2]string{
								{"3", `{"schemaVersion": 1, "previous": "pending", "current": "firing", "values":{"a": "b"}}`},
							},
						},
					},
				},
			},
			ruleID: "123456",
			expectedTime: []time.Time{
				time.Unix(0, 1),
				time.Unix(0, 2),
				time.Unix(0, 3),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := merge(tc.res, tc.ruleID)
			require.NoError(t, err)

			var dfTimeColumn *data.Field
			for _, f := range m.Fields {
				if f.Name == dfTime {
					dfTimeColumn = f
				}
			}

			require.NotNil(t, dfTimeColumn)

			for i := 0; i < len(tc.expectedTime); i++ {
				require.Equal(t, tc.expectedTime[i], dfTimeColumn.At(i))
			}
		})
	}
}

func TestRecordStates(t *testing.T) {
	t.Run("writes state transitions to loki", func(t *testing.T) {
		req := NewFakeRequester()
		loki := createTestLokiBackend(req, metrics.NewHistorianMetrics(prometheus.NewRegistry()))
		rule := createTestRule()
		states := singleFromNormal(&state.State{
			State:  eval.Alerting,
			Labels: data.Labels{"a": "b"},
		})

		err := <-loki.Record(context.Background(), rule, states)

		require.NoError(t, err)
		require.Contains(t, "/loki/api/v1/push", req.lastRequest.URL.Path)
	})

	t.Run("emits expected write metrics", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		met := metrics.NewHistorianMetrics(reg)
		loki := createTestLokiBackend(NewFakeRequester(), met)
		errLoki := createTestLokiBackend(NewFakeRequester().WithResponse(badResponse()), met) //nolint:bodyclose
		rule := createTestRule()
		states := singleFromNormal(&state.State{
			State:  eval.Alerting,
			Labels: data.Labels{"a": "b"},
		})

		<-loki.Record(context.Background(), rule, states)
		<-errLoki.Record(context.Background(), rule, states)

		exp := bytes.NewBufferString(`
# HELP grafana_alerting_state_history_transitions_failed_total The total number of state transitions that failed to be written - they are not retried.
# TYPE grafana_alerting_state_history_transitions_failed_total counter
grafana_alerting_state_history_transitions_failed_total{org="1"} 1
# HELP grafana_alerting_state_history_transitions_total The total number of state transitions processed.
# TYPE grafana_alerting_state_history_transitions_total counter
grafana_alerting_state_history_transitions_total{org="1"} 2
# HELP grafana_alerting_state_history_writes_failed_total The total number of failed writes of state history batches.
# TYPE grafana_alerting_state_history_writes_failed_total counter
grafana_alerting_state_history_writes_failed_total{backend="loki",org="1"} 1
# HELP grafana_alerting_state_history_writes_total The total number of state history batches that were attempted to be written.
# TYPE grafana_alerting_state_history_writes_total counter
grafana_alerting_state_history_writes_total{backend="loki",org="1"} 2
`)
		err := testutil.GatherAndCompare(reg, exp,
			"grafana_alerting_state_history_transitions_total",
			"grafana_alerting_state_history_transitions_failed_total",
			"grafana_alerting_state_history_writes_total",
			"grafana_alerting_state_history_writes_failed_total",
		)
		require.NoError(t, err)
	})

	t.Run("elides request if nothing to send", func(t *testing.T) {
		req := NewFakeRequester()
		loki := createTestLokiBackend(req, metrics.NewHistorianMetrics(prometheus.NewRegistry()))
		rule := createTestRule()
		states := []state.StateTransition{}

		err := <-loki.Record(context.Background(), rule, states)

		require.NoError(t, err)
		require.Nil(t, req.lastRequest)
	})

	t.Run("succeeds with special chars in labels", func(t *testing.T) {
		req := NewFakeRequester()
		loki := createTestLokiBackend(req, metrics.NewHistorianMetrics(prometheus.NewRegistry()))
		rule := createTestRule()
		states := singleFromNormal(&state.State{
			State: eval.Alerting,
			Labels: data.Labels{
				"dots":   "contains.dot",
				"equals": "contains=equals",
				"emoji":  "contains🤔emoji",
			},
		})

		err := <-loki.Record(context.Background(), rule, states)

		require.NoError(t, err)
		require.Contains(t, "/loki/api/v1/push", req.lastRequest.URL.Path)
		sent := string(readBody(t, req.lastRequest))
		require.Contains(t, sent, "contains.dot")
		require.Contains(t, sent, "contains=equals")
		require.Contains(t, sent, "contains🤔emoji")
	})

	t.Run("adds external labels to log lines", func(t *testing.T) {
		req := NewFakeRequester()
		loki := createTestLokiBackend(req, metrics.NewHistorianMetrics(prometheus.NewRegistry()))
		rule := createTestRule()
		states := singleFromNormal(&state.State{
			State: eval.Alerting,
		})

		err := <-loki.Record(context.Background(), rule, states)

		require.NoError(t, err)
		require.Contains(t, "/loki/api/v1/push", req.lastRequest.URL.Path)
		sent := string(readBody(t, req.lastRequest))
		require.Contains(t, sent, "externalLabelKey")
		require.Contains(t, sent, "externalLabelValue")
	})
}

func createTestLokiBackend(req client.Requester, met *metrics.Historian) *RemoteLokiBackend {
	url, _ := url.Parse("http://some.url")
	cfg := LokiConfig{
		WritePathURL:   url,
		ReadPathURL:    url,
		Encoder:        JsonEncoder{},
		ExternalLabels: map[string]string{"externalLabelKey": "externalLabelValue"},
	}
	return NewRemoteLokiBackend(cfg, req, met)
}

func singleFromNormal(st *state.State) []state.StateTransition {
	return []state.StateTransition{
		{
			PreviousState: eval.Normal,
			State:         st,
		},
	}
}

func createTestRule() history_model.RuleMeta {
	return history_model.RuleMeta{
		OrgID:        1,
		UID:          "rule-uid",
		Group:        "my-group",
		NamespaceUID: "my-folder",
		DashboardUID: "dash-uid",
		PanelID:      123,
	}
}

func requireSingleEntry(t *testing.T, res []stream) lokiEntry {
	require.Len(t, res, 1)
	require.Len(t, res[0].Values, 1)
	return requireEntry(t, res[0].Values[0])
}

func requireEntry(t *testing.T, row row) lokiEntry {
	t.Helper()

	var entry lokiEntry
	err := json.Unmarshal([]byte(row.Val), &entry)
	require.NoError(t, err)
	return entry
}

func badResponse() *http.Response {
	return &http.Response{
		Status:        "400 Bad Request",
		StatusCode:    http.StatusBadRequest,
		Body:          io.NopCloser(bytes.NewBufferString("")),
		ContentLength: int64(0),
		Header:        make(http.Header, 0),
	}
}

func readBody(t *testing.T, req *http.Request) []byte {
	t.Helper()

	val, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	return val
}
