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

//go:build aix

package main

import (
	"log/slog"
	"os"

	"github.com/power-devops/perfstat"
	"github.com/prometheus/client_golang/prometheus"
)

// aixProcessCollector exposes this process's own resource usage on AIX,
// filling the gap client_golang's standard process collector leaves on
// this platform -- it only supports Linux/Darwin/Windows (see
// errorCollectFn in client_golang's process_collector.go), so AIX gets
// nothing at all from promcollectors.NewProcessCollector. Sourced via
// perfstat.ProcessStat(), the same library every other AIX collector in
// this package already depends on (cpu_aix.go, meminfo_aix.go, etc.),
// filtered down to this process's own PID -- nothing here calls out to
// libperfstat that isn't already a dependency.
//
// Metric names/help text for the three that overlap with the standard
// collector (process_cpu_seconds_total, process_virtual_memory_bytes,
// process_resident_memory_bytes) deliberately match client_golang's own
// wording exactly, so this is a real drop-in on AIX, not a parallel
// naming scheme -- anything built against the standard names elsewhere
// just starts working. process_num_threads and process_pinned_memory_bytes
// have no standard client_golang equivalent; pinned (never-pageable)
// memory specifically matters for AIX paging-space exhaustion in a way
// that doesn't have a cross-platform analog.
//
// Deliberately NOT implemented: process_open_fds/max_fds and
// process_virtual_memory_max_bytes (no direct equivalent in
// perfstat_process_t without additional work), and
// process_start_time_seconds (perfstat_process_t has no start-time field;
// getting one would mean shelling out to `ps` on every scrape, which isn't
// acceptable inside node_exporter's own request path).
type aixProcessCollector struct {
	logger *slog.Logger

	cpuTotal *prometheus.Desc
	vsize    *prometheus.Desc
	rss      *prometheus.Desc
	threads  *prometheus.Desc
	pinned   *prometheus.Desc
}

// newProcessCollector returns a Collector exposing this process's own
// resource usage. See process_collector_default.go for the non-AIX
// implementation this replaces.
func newProcessCollector(logger *slog.Logger) prometheus.Collector {
	return &aixProcessCollector{
		logger: logger,
		cpuTotal: prometheus.NewDesc(
			"process_cpu_seconds_total",
			"Total user and system CPU time spent in seconds.",
			nil, nil,
		),
		vsize: prometheus.NewDesc(
			"process_virtual_memory_bytes",
			"Virtual memory size in bytes.",
			nil, nil,
		),
		rss: prometheus.NewDesc(
			"process_resident_memory_bytes",
			"Resident memory size in bytes.",
			nil, nil,
		),
		threads: prometheus.NewDesc(
			"process_num_threads",
			"Number of OS threads in the process.",
			nil, nil,
		),
		pinned: prometheus.NewDesc(
			"process_pinned_memory_bytes",
			"Pinned (never-pageable) memory of the process in bytes -- doesn't relieve paging-space pressure no matter how low free paging space gets, unlike the rest of resident memory.",
			nil, nil,
		),
	}
}

func (c *aixProcessCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.cpuTotal
	ch <- c.vsize
	ch <- c.rss
	ch <- c.threads
	ch <- c.pinned
}

func (c *aixProcessCollector) Collect(ch chan<- prometheus.Metric) {
	procs, err := perfstat.ProcessStat()
	if err != nil {
		c.logger.Error("couldn't get process stats for self-monitoring", "err", err)
		return
	}

	pid := int64(os.Getpid())
	for _, p := range procs {
		if p.PID != pid {
			continue
		}

		// RealMemData/RealMemText/VirtMemData/VirtMemText/Pinned are in
		// KB; UCpuTime/SCpuTime are in milliseconds when filled by
		// perfstat_process() (as opposed to percentage when filled by
		// perfstat_process_util() instead) -- same units already
		// confirmed against the real libperfstat header on this
		// platform (see aix_grafana/reswatch/reswatch.c, built against
		// the same struct).
		ch <- prometheus.MustNewConstMetric(c.cpuTotal, prometheus.CounterValue, (p.UCpuTime+p.SCpuTime)/1000.0)
		ch <- prometheus.MustNewConstMetric(c.vsize, prometheus.GaugeValue, float64(p.VirtMemData+p.VirtMemText)*1024)
		ch <- prometheus.MustNewConstMetric(c.rss, prometheus.GaugeValue, float64(p.RealMemData+p.RealMemText)*1024)
		ch <- prometheus.MustNewConstMetric(c.threads, prometheus.GaugeValue, float64(p.NumThreads))
		ch <- prometheus.MustNewConstMetric(c.pinned, prometheus.GaugeValue, float64(p.Pinned)*1024)
		return
	}

	c.logger.Error("couldn't find self in perfstat.ProcessStat() output", "pid", pid)
}
