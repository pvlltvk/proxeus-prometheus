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

package parser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInspectIsSequential covers the promxy patch: Walk fans the children of a
// node out to goroutines, but Inspect must not, since its visitor is an
// arbitrary closure over unsynchronised shared state. Fails under -race if
// Inspect ever walks in parallel again.
func TestInspectIsSequential(t *testing.T) {
	expr, err := ParseExpr(`rate(http_requests[40s]) - rate(http_requests[1m] offset 10000s) + sum(up) + count(up)`)
	require.NoError(t, err)

	selectors := 0
	_, err = Inspect(context.Background(), &EvalStmt{Expr: expr}, func(node Node, _ []Node) error {
		if _, ok := node.(*VectorSelector); ok {
			selectors++
		}
		return nil
	}, nil)
	require.NoError(t, err)
	require.Equal(t, 4, selectors)
}
