// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Stats represents container statistics, returned by /containers/<id>/stats.
//
// See http://goo.gl/DFMiYD for more details.
type Stats struct {
	Read        time.Time    `json:"read,omitempty" yaml:"read,omitempty"`
	Network     NetworkStats `json:"network,omitempty" yaml:"network,omitempty"`
	MemoryStats MemoryStats  `json:"memory_stats,omitempty" yaml:"memory_stats,omitempty"`
	BlkioStats  BlkioStats   `json:"blkio_stats,omitempty" yaml:"blkio_stats,omitempty"`
	CPUStats    CPUStats     `json:"cpu_stats,omitempty" yaml:"cpu_stats,omitempty"`
}

// NetworkStats represents container network statistics.
type NetworkStats struct {
	RxDropped uint64 `json:"rx_dropped,omitempty" yaml:"rx_dropped,omitempty"`
	RxBytes   uint64 `json:"rx_bytes,omitempty" yaml:"rx_bytes,omitempty"`
	RxErrors  uint64 `json:"rx_errors,omitempty" yaml:"rx_errors,omitempty"`
	TxPackets uint64 `json:"tx_packets,omitempty" yaml:"tx_packets,omitempty"`
	TxDropped uint64 `json:"tx_dropped,omitempty" yaml:"tx_dropped,omitempty"`
	RxPackets uint64 `json:"rx_packets,omitempty" yaml:"rx_packets,omitempty"`
	TxErrors  uint64 `json:"tx_errors,omitempty" yaml:"tx_errors,omitempty"`
	TxBytes   uint64 `json:"tx_bytes,omitempty" yaml:"tx_bytes,omitempty"`
}

// MemoryStats represents container memory statistics.
type MemoryStats struct {
	Stats    SysMemoryStats `json:"stats,omitempty" yaml:"stats,omitempty"`
	MaxUsage uint64         `json:"max_usage,omitempty" yaml:"max_usage,omitempty"`
	Usage    uint64         `json:"usage,omitempty" yaml:"usage,omitempty"`
	Failcnt  uint64         `json:"failcnt,omitempty" yaml:"failcnt,omitempty"`
	Limit    uint64         `json:"limit,omitempty" yaml:"limit,omitempty"`
}

// SysMemoryStats represents the container memory statistics exported via
// proc's memory.stat.
type SysMemoryStats struct {
	TotalPgmafault          uint64 `json"total_pgmafault,omitempty" yaml:"total_pgmafault,omitempty"`
	Cache                   uint64 `json:"cache,omitempty" yaml:"cache,omitempty"`
	MappedFile              uint64 `json:"mapped_file,omitempty" yaml:"mapped_file,omitempty"`
	TotalInactiveFile       uint64 `json:"total_inactive_file,omitempty" yaml:"total_inactive_file,omitempty"`
	Pgpgout                 uint64 `json:"pgpgout,omitempty" yaml:"pgpgout,omitempty"`
	Rss                     uint64 `json:"rss,omitempty" yaml:"rss,omitempty"`
	TotalMappedFile         uint64 `json:"total_mapped_file,omitempty" yaml:"total_mapped_file,omitempty"`
	Writeback               uint64 `json:"writeback,omitempty" yaml:"writeback,omitempty"`
	Unevictable             uint64 `json:"unevictable,omitempty" yaml:"unevictable,omitempty"`
	Pgpgin                  uint64 `json:"pgpgin,omitempty" yaml:"pgpgin,omitempty"`
	TotalUnevictable        uint64 `json:"total_unevictable,omitempty" yaml:"total_unevictable,omitempty"`
	Pgmajfault              uint64 `json:"pgmajfault,omitempty" yaml:"pgmajfault,omitempty"`
	TotalRss                uint64 `json:"total_rss,omitempty" yaml:"total_rss,omitempty"`
	TotalRssHuge            uint64 `json:"total_rss_huge,omitempty" yaml:"total_rss_huge,omitempty"`
	TotalWriteback          uint64 `json:"total_writeback,omitempty" yaml:"total_writeback,omitempty"`
	TotalInactiveAnon       uint64 `json:"total_inactive_anon,omitempty" yaml:"total_inactive_anon,omitempty"`
	RssHuge                 uint64 `json:"rss_huge,omitempty" yaml:"rss_huge,omitempty"`
	HierarchicalMemoryLimit uint64 `json:"hierarchical_memory_limit,omitempty" yaml:"hierarchical_memory_limit,omitempty"`
	TotalPgfault            uint64 `json:"total_pgfault,omitempty" yaml:"total_pgfault,omitempty"`
	TotalActiveFile         uint64 `json:"total_active_file,omitempty" yaml:"total_active_file,omitempty"`
	ActiveAnon              uint64 `json:"active_anon,omitempty" yaml:"active_anon,omitempty"`
	TotalActiveAnon         uint64 `json:"total_active_anon,omitempty" yaml:"total_active_anon,omitempty"`
	TotalPgpgout            uint64 `json:"total_pgpgout,omitempty" yaml:"total_pgpgout,omitempty"`
	TotalCache              uint64 `json:"total_cache,omitempty" yaml:"total_cache,omitempty"`
	InactiveAnon            uint64 `json:"inactive_anon,omitempty" yaml:"inactive_anon,omitempty"`
	ActiveFile              uint64 `json:"active_file,omitempty" yaml:"active_file,omitempty"`
	Pgfault                 uint64 `json:"pgfault,omitempty" yaml:"pgfault,omitempty"`
	InactiveFile            uint64 `json:"inactive_file,omitempty" yaml:"inactive_file,omitempty"`
	TotalPgpgin             uint64 `json:"total_pgpgin,omitempty" yaml:"total_pgpgin,omitempty"`
}

// BlkioStats represents container block device statistics.
type BlkioStats struct {
	IOServiceBytesRecursive []BlkioStatsEntry `json:"io_service_bytes_recursive,omitempty" yaml:"io_service_bytes_recursive,omitempty"`
	IOServicedRecursive     []BlkioStatsEntry `json:"io_serviced_recursive,omitempty" yaml:"io_serviced_recursive,omitempty"`
	IOQueueRecursive        []BlkioStatsEntry `json:"io_queue_recursive,omitempty" yaml:"io_queue_recursive,omitempty"`
	IOServiceTimeRecursive  []BlkioStatsEntry `json:"io_service_time_recursive,omitempty" yaml:"io_service_time_recursive,omitempty"`
	IOWaitTimeRecursive     []BlkioStatsEntry `json:"io_wait_time_recursive,omitempty" yaml:"io_wait_time_recursive,omitempty"`
	IOMergedRecursive       []BlkioStatsEntry `json:"io_merged_recursive,omitempty" yaml:"io_merged_recursive,omitempty"`
	IOTimeRecursive         []BlkioStatsEntry `json:"io_time_recursive,omitempty" yaml:"io_time_recursive,omitempty"`
	SectorsRecursive        []BlkioStatsEntry `json:"sectors_recursive,omitempty" yaml:"sectors_recursive,omitempty"`
}

// BlkioStatsEntry is a stats entry for blkio_stats.
type BlkioStatsEntry struct {
	Major uint64 `json:"major,omitempty" yaml:"major,omitempty"`
	Minor uint64 `json:"major,omitempty" yaml:"major,omitempty"`
	Op    string `json:"op,omitempty" yaml:"op,omitempty"`
	Value uint64 `json:"value,omitempty" yaml:"value,omitempty"`
}

// CPUStats represents container CPU statistics.
type CPUStats struct {
	CPUUsage       CPUUsage       `json:"cpu_usage,omitempty" yaml:"cpu_usage,omitempty"`
	SystemCPUUsage uint64         `json:"system_cpu_usage,omitempty" yaml:"system_cpu_usage,omitempty"`
	ThrottlingData ThrottlingData `json:"throttling_data,omitempty" yaml:"throttling_data,omitempty"`
}

// CPUUsage represents container CPU usage, as reported by the system.
type CPUUsage struct {
	PercpuUsage       []uint64 `json:"percpu_usage,omitempty" yaml:"percpu_usage,omitempty"`
	UsageInUsermode   uint64   `json:"usage_in_usermode,omitempty" yaml:"usage_in_usermode,omitempty"`
	TotalUsage        uint64   `json:"total_usage,omitempty" yaml:"total_usage,omitempty"`
	UsageInKernelmode uint64   `json:"usage_in_kernelmode,omitempty" yaml:"usage_in_kernelmode,omitempty"`
}

// ThrottlingData represents container CPU throttling information.
type ThrottlingData struct {
	Periods          uint64 `json:"periods,omitempty"`
	ThrottledPeriods uint64 `json:"throttled_periods,omitempty"`
	ThrottledTime    uint64 `json:"throttled_time,omitempty"`
}

// StatsOptions specify parameters to the Stats function.
//
// See http://goo.gl/DFMiYD for more details.
type StatsOptions struct {
	ID    string
	Stats chan<- *Stats
}

// Stats sends container statistics for the given container to the given channel.
//
// This function is blocking, similar to a streaming call for logs, and should be run
// on a separate goroutine from the caller. Note that this function will block until
// the given container is removed, not just exited. When finished, this function
// will close the given channel.
//
// See http://goo.gl/DFMiYD for more details.
func (c *Client) Stats(opts StatsOptions) (retErr error) {
	errC := make(chan error, 1)
	readCloser, writeCloser := io.Pipe()

	defer func() {
		close(opts.Stats)
		if err := <-errC; err != nil && retErr == nil {
			retErr = err
		}
		if err := readCloser.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	go func() {
		err := c.stream("GET", fmt.Sprintf("/containers/%s/stats", opts.ID), streamOptions{
			rawJSONStream: true,
			stdout:        writeCloser,
		})
		if err != nil {
			dockerError, ok := err.(*Error)
			if ok {
				if dockerError.Status == http.StatusNotFound {
					err = &NoSuchContainer{ID: opts.ID}
				}
			}
		}
		if closeErr := writeCloser.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		errC <- err
		close(errC)
	}()

	decoder := json.NewDecoder(readCloser)
	stats := new(Stats)
	for err := decoder.Decode(&stats); err != io.EOF; err = decoder.Decode(stats) {
		if err != nil {
			return err
		}
		opts.Stats <- stats
		stats = new(Stats)
	}
	return nil
}
