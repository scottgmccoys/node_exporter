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

//go:build !aix

package main

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
)

// newProcessCollector returns client_golang's standard process collector on
// every platform it actually supports. AIX gets its own implementation
// instead -- see process_collector_aix.go -- since client_golang has no
// AIX support at all.
func newProcessCollector(logger *slog.Logger) prometheus.Collector {
	return promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{})
}
