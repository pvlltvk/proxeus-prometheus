// Copyright 2026 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promql_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/promqltest"
)

// TestVectorSelectorLookbackDelta covers the promxy patch: a VectorSelector
// carrying its own LookbackDelta must not have its samples repeated past that
// window. promxy sets it on the selectors it substitutes for a pushed-down
// aggregation, so that each step only sees the sample computed for it.
func TestVectorSelectorLookbackDelta(t *testing.T) {
	storage := promqltest.LoadedStorage(t, `
load 1m
	metric 1
`)
	t.Cleanup(func() { storage.Close() })

	start := time.Unix(0, 0)
	end := start.Add(5 * time.Minute)

	for _, c := range []struct {
		name          string
		query         string
		lookbackDelta time.Duration
		points        int
	}{
		{"selector, engine default", "metric", 0, 5},
		{"selector, per-selector delta", "metric", 59 * time.Second, 1},
		{"timestamp(), engine default", "timestamp(metric)", 0, 5},
		{"timestamp(), per-selector delta", "timestamp(metric)", 59 * time.Second, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			engine := promqltest.NewTestEngine(t, false, 0, promqltest.DefaultMaxSamplesPerQuery)
			engine.NodeReplacer = func(_ context.Context, _ *parser.EvalStmt, node parser.Node, _ []parser.Node) (parser.Node, error) {
				if vs, ok := node.(*parser.VectorSelector); ok {
					vs.LookbackDelta = c.lookbackDelta
				}
				return nil, nil
			}

			qry, err := engine.NewRangeQuery(context.Background(), storage, nil, c.query, start, end, time.Minute)
			require.NoError(t, err)
			defer qry.Close()

			res := qry.Exec(context.Background())
			require.NoError(t, res.Err)

			m, ok := res.Value.(promql.Matrix)
			require.True(t, ok)
			require.Len(t, m, 1)
			require.Len(t, m[0].Floats, c.points)
		})
	}
}
