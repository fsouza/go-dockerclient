package docker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// The test for Docker Stat API of the host uses cgroup
func TestStatsV1(t *testing.T) {
	t.Parallel()
	jsonStats1 := `{
       "read" : "2015-01-08T22:57:31.547920715Z",
       "network" : {
          "rx_dropped" : 0,
          "rx_bytes" : 648,
          "rx_errors" : 0,
          "tx_packets" : 8,
          "tx_dropped" : 0,
          "rx_packets" : 8,
          "tx_errors" : 0,
          "tx_bytes" : 648
       },
	   "networks" : {
		   "eth0":{
			   "rx_dropped" : 0,
			   "rx_bytes" : 648,
			   "rx_errors" : 0,
			   "tx_packets" : 8,
			   "tx_dropped" : 0,
			   "rx_packets" : 8,
			   "tx_errors" : 0,
			   "tx_bytes" : 648
		   }
	   },
       "memory_stats" : {
          "stats" : {
             "total_pgmajfault" : 0,
             "cache" : 0,
             "mapped_file" : 0,
             "total_inactive_file" : 0,
             "pgpgout" : 414,
             "rss" : 6537216,
             "total_mapped_file" : 0,
             "writeback" : 0,
             "unevictable" : 0,
             "pgpgin" : 477,
             "total_unevictable" : 0,
             "pgmajfault" : 0,
             "total_rss" : 6537216,
             "total_rss_huge" : 6291456,
             "total_writeback" : 0,
             "total_inactive_anon" : 0,
             "rss_huge" : 6291456,
	     "hierarchical_memory_limit": 189204833,
             "total_pgfault" : 964,
             "total_active_file" : 0,
             "active_anon" : 6537216,
             "total_active_anon" : 6537216,
             "total_pgpgout" : 414,
             "total_cache" : 0,
             "inactive_anon" : 0,
             "active_file" : 0,
             "pgfault" : 964,
             "inactive_file" : 0,
             "total_pgpgin" : 477,
             "swap" : 47312896,
             "hierarchical_memsw_limit" : 1610612736
          },
          "max_usage" : 6651904,
          "usage" : 6537216,
          "failcnt" : 0,
          "limit" : 67108864
       },
       "blkio_stats": {
          "io_service_bytes_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 428795731968
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 388177920
             }
          ],
          "io_serviced_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 25994442
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 1734
             }
          ],
          "io_queue_recursive": [],
          "io_service_time_recursive": [],
          "io_wait_time_recursive": [],
          "io_merged_recursive": [],
          "io_time_recursive": [],
          "sectors_recursive": []
       },
       "cpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       },
       "precpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       }
    }`
	// 1 second later, cache is 100
	jsonStats2 := `{
       "read" : "2015-01-08T22:57:32.547920715Z",
	   "networks" : {
		   "eth0":{
			   "rx_dropped" : 0,
			   "rx_bytes" : 648,
			   "rx_errors" : 0,
			   "tx_packets" : 8,
			   "tx_dropped" : 0,
			   "rx_packets" : 8,
			   "tx_errors" : 0,
			   "tx_bytes" : 648
		   }
	   },
	   "memory_stats" : {
          "stats" : {
             "total_pgmajfault" : 0,
             "cache" : 100,
             "mapped_file" : 0,
             "total_inactive_file" : 0,
             "pgpgout" : 414,
             "rss" : 6537216,
             "total_mapped_file" : 0,
             "writeback" : 0,
             "unevictable" : 0,
             "pgpgin" : 477,
             "total_unevictable" : 0,
             "pgmajfault" : 0,
             "total_rss" : 6537216,
             "total_rss_huge" : 6291456,
             "total_writeback" : 0,
             "total_inactive_anon" : 0,
             "rss_huge" : 6291456,
             "total_pgfault" : 964,
             "total_active_file" : 0,
             "active_anon" : 6537216,
             "total_active_anon" : 6537216,
             "total_pgpgout" : 414,
             "total_cache" : 0,
             "inactive_anon" : 0,
             "active_file" : 0,
             "pgfault" : 964,
             "inactive_file" : 0,
             "total_pgpgin" : 477,
             "swap" : 47312896,
             "hierarchical_memsw_limit" : 1610612736
          },
          "max_usage" : 6651904,
          "usage" : 6537216,
          "failcnt" : 0,
          "limit" : 67108864
       },
       "blkio_stats": {
          "io_service_bytes_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 428795731968
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 388177920
             }
          ],
          "io_serviced_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 25994442
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 1734
             }
          ],
          "io_queue_recursive": [],
          "io_service_time_recursive": [],
          "io_wait_time_recursive": [],
          "io_merged_recursive": [],
          "io_time_recursive": [],
          "sectors_recursive": []
       },
       "cpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       },
       "precpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       }
    }`
	var expected1 Stats
	var expected2 Stats
	err := json.Unmarshal([]byte(jsonStats1), &expected1)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal([]byte(jsonStats2), &expected2)
	if err != nil {
		t.Fatal(err)
	}
	id := "4fa6e0f0"

	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonStats1))
		w.Write([]byte(jsonStats2))
		req = *r
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	errC := make(chan error, 1)
	statsC := make(chan *Stats)
	done := make(chan bool)
	defer close(done)
	go func() {
		errC <- client.Stats(StatsOptions{ID: id, Stats: statsC, Stream: true, Done: done})
		close(errC)
	}()
	var resultStats []*Stats
	for {
		stats, ok := <-statsC
		if !ok {
			break
		}
		resultStats = append(resultStats, stats)
	}
	err = <-errC
	if err != nil {
		t.Fatal(err)
	}
	if len(resultStats) != 2 {
		t.Fatalf("Stats: Expected 2 results. Got %d.", len(resultStats))
	}
	if !reflect.DeepEqual(resultStats[0], &expected1) {
		t.Errorf("Stats: Expected:\n%+v\nGot:\n%+v", expected1, resultStats[0])
	}
	if !reflect.DeepEqual(resultStats[1], &expected2) {
		t.Errorf("Stats: Expected:\n%+v\nGot:\n%+v", expected2, resultStats[1])
	}
	if req.Method != http.MethodGet {
		t.Errorf("Stats: wrong HTTP method. Want GET. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/containers/" + id + "/stats"))
	if req.URL.Path != u.Path {
		t.Errorf("Stats: wrong HTTP path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

// The test for Docker Stat API of the host uses cgroup2
func TestStatsV2(t *testing.T) {
	t.Parallel()
	jsonStats1 := `{
      "read": "2022-05-18T14:37:18.175989615Z",
      "preread": "2022-05-18T14:37:17.171501052Z",
      "pids_stats": {
        "current": 29,
        "limit": 76948
      },
      "blkio_stats": {
        "io_service_bytes_recursive": [
          {
            "major": 259,
            "minor": 0,
            "op": "read",
            "value": 11390976
          },
          {
            "major": 259,
            "minor": 0,
            "op": "write",
            "value": 0
          }
        ],
        "io_serviced_recursive": null,
        "io_queue_recursive": null,
        "io_service_time_recursive": null,
        "io_wait_time_recursive": null,
        "io_merged_recursive": null,
        "io_time_recursive": null,
        "sectors_recursive": null
      },
      "num_procs": 0,
      "storage_stats": {},
      "cpu_stats": {
        "cpu_usage": {
          "total_usage": 185266562000,
          "usage_in_kernelmode": 37912635000,
          "usage_in_usermode": 147353926000
        },
        "system_cpu_usage": 26707255190000000,
        "online_cpus": 24,
        "throttling_data": {
          "periods": 0,
          "throttled_periods": 0,
          "throttled_time": 0
        }
      },
      "precpu_stats": {
        "cpu_usage": {
          "total_usage": 185266562000,
          "usage_in_kernelmode": 37912635000,
          "usage_in_usermode": 147353926000
        },
        "system_cpu_usage": 26707231080000000,
        "online_cpus": 24,
        "throttling_data": {
          "periods": 0,
          "throttled_periods": 0,
          "throttled_time": 0
        }
      },
      "memory_stats": {
        "usage": 28557312,
        "stats": {
          "active_anon": 4096,
          "active_file": 7446528,
          "anon": 16572416,
          "anon_thp": 0,
          "file": 10829824,
          "file_dirty": 0,
          "file_mapped": 9740288,
          "file_writeback": 0,
          "inactive_anon": 8069120,
          "inactive_file": 11882496,
          "kernel_stack": 475136,
          "pgactivate": 241,
          "pgdeactivate": 253,
          "pgfault": 7714,
          "pglazyfree": 3042,
          "pglazyfreed": 967,
          "pgmajfault": 155,
          "pgrefill": 301,
          "pgscan": 1802,
          "pgsteal": 1100,
          "shmem": 0,
          "slab": 488920,
          "slab_reclaimable": 159664,
          "slab_unreclaimable": 329256,
          "sock": 0,
          "thp_collapse_alloc": 0,
          "thp_fault_alloc": 0,
          "unevictable": 0,
          "workingset_activate": 0,
          "workingset_nodereclaim": 0,
          "workingset_refault": 0
        },
        "limit": 67353382912
      },
      "networks": {
        "eth0": {
          "rx_bytes": 96802652,
          "rx_packets": 623704,
          "rx_errors": 0,
          "rx_dropped": 0,
          "tx_bytes": 16597749,
          "tx_packets": 91982,
          "tx_errors": 0,
          "tx_dropped": 0
        }
      }
    }`
	// 1 second later, shmem is 100
	jsonStats2 := `{
      "read": "2022-05-18T14:37:18.175989615Z",
      "preread": "2022-05-18T14:37:17.171501052Z",
      "pids_stats": {
        "current": 29,
        "limit": 76948
      },
      "blkio_stats": {
        "io_service_bytes_recursive": [
          {
            "major": 259,
            "minor": 0,
            "op": "read",
            "value": 11390976
          },
          {
            "major": 259,
            "minor": 0,
            "op": "write",
            "value": 0
          }
        ],
        "io_serviced_recursive": null,
        "io_queue_recursive": null,
        "io_service_time_recursive": null,
        "io_wait_time_recursive": null,
        "io_merged_recursive": null,
        "io_time_recursive": null,
        "sectors_recursive": null
      },
      "num_procs": 0,
      "storage_stats": {},
      "cpu_stats": {
        "cpu_usage": {
          "total_usage": 185266562000,
          "usage_in_kernelmode": 37912635000,
          "usage_in_usermode": 147353926000
        },
        "system_cpu_usage": 26707255190000000,
        "online_cpus": 24,
        "throttling_data": {
          "periods": 0,
          "throttled_periods": 0,
          "throttled_time": 0
        }
      },
      "precpu_stats": {
        "cpu_usage": {
          "total_usage": 185266562000,
          "usage_in_kernelmode": 37912635000,
          "usage_in_usermode": 147353926000
        },
        "system_cpu_usage": 26707231080000000,
        "online_cpus": 24,
        "throttling_data": {
          "periods": 0,
          "throttled_periods": 0,
          "throttled_time": 0
        }
      },
      "memory_stats": {
        "usage": 28557312,
        "stats": {
          "active_anon": 4096,
          "active_file": 7446528,
          "anon": 16572416,
          "anon_thp": 0,
          "file": 10829824,
          "file_dirty": 0,
          "file_mapped": 9740288,
          "file_writeback": 0,
          "inactive_anon": 8069120,
          "inactive_file": 11882496,
          "kernel_stack": 475136,
          "pgactivate": 241,
          "pgdeactivate": 253,
          "pgfault": 7714,
          "pglazyfree": 3042,
          "pglazyfreed": 967,
          "pgmajfault": 155,
          "pgrefill": 301,
          "pgscan": 1802,
          "pgsteal": 1100,
          "shmem": 100,
          "slab": 488920,
          "slab_reclaimable": 159664,
          "slab_unreclaimable": 329256,
          "sock": 0,
          "thp_collapse_alloc": 0,
          "thp_fault_alloc": 0,
          "unevictable": 0,
          "workingset_activate": 0,
          "workingset_nodereclaim": 0,
          "workingset_refault": 0
        },
        "limit": 67353382912
      },
      "networks": {
        "eth0": {
          "rx_bytes": 96802652,
          "rx_packets": 623704,
          "rx_errors": 0,
          "rx_dropped": 0,
          "tx_bytes": 16597749,
          "tx_packets": 91982,
          "tx_errors": 0,
          "tx_dropped": 0
        }
      }
    }`
	var expected1 Stats
	var expected2 Stats
	err := json.Unmarshal([]byte(jsonStats1), &expected1)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal([]byte(jsonStats2), &expected2)
	if err != nil {
		t.Fatal(err)
	}
	id := "4fa6e0f0"

	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonStats1))
		w.Write([]byte(jsonStats2))
		req = *r
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	errC := make(chan error, 1)
	statsC := make(chan *Stats)
	done := make(chan bool)
	defer close(done)
	go func() {
		errC <- client.Stats(StatsOptions{ID: id, Stats: statsC, Stream: true, Done: done})
		close(errC)
	}()
	var resultStats []*Stats
	for {
		stats, ok := <-statsC
		if !ok {
			break
		}
		resultStats = append(resultStats, stats)
	}
	err = <-errC
	if err != nil {
		t.Fatal(err)
	}
	if len(resultStats) != 2 {
		t.Fatalf("Stats: Expected 2 results. Got %d.", len(resultStats))
	}
	if !reflect.DeepEqual(resultStats[0], &expected1) {
		t.Errorf("Stats: Expected:\n%+v\nGot:\n%+v", expected1, resultStats[0])
	}
	if !reflect.DeepEqual(resultStats[1], &expected2) {
		t.Errorf("Stats: Expected:\n%+v\nGot:\n%+v", expected2, resultStats[1])
	}
	if req.Method != http.MethodGet {
		t.Errorf("Stats: wrong HTTP method. Want GET. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/containers/" + id + "/stats"))
	if req.URL.Path != u.Path {
		t.Errorf("Stats: wrong HTTP path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestStatsContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	statsC := make(chan *Stats)
	done := make(chan bool)
	defer close(done)
	err := client.Stats(StatsOptions{ID: "abef348", Stats: statsC, Stream: true, Done: done})
	expectNoSuchContainer(t, "abef348", err)
}
