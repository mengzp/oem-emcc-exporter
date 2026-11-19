package collector

import (
	"context"
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

// ProcessScraper 采集主机关键进程的 CPU / 内存占用。
// 扩展点：定位进程类 metric（如 HostProcess），补充 process_name label 后输出。
type ProcessScraper struct {
	procCPUMDesc *prometheus.Desc
	procMemDesc  *prometheus.Desc
}

// NewProcessScraper 构造 ProcessScraper。
func NewProcessScraper(db *sql.DB) Scraper {
	return &ProcessScraper{
		procCPUMDesc: prometheus.NewDesc(
			"oem_host_process_cpu_pct",
			"OEM host process CPU percent.",
			[]string{"target_guid", "host_name", "process_name"}, nil,
		),
		procMemDesc: prometheus.NewDesc(
			"oem_host_process_mem_pct",
			"OEM host process memory percent.",
			[]string{"target_guid", "host_name", "process_name"}, nil,
		),
	}
}

// Name 实现 Scraper。
func (s *ProcessScraper) Name() string { return "process" }

// Help 实现 Scraper。
func (s *ProcessScraper) Help() string { return "Host process CPU and memory utilization" }

// Scrape 实现 Scraper。
// TODO: 补充进程 SQL，扫描 process_name、CPU、内存后输出上述两个 Desc。
func (s *ProcessScraper) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	_ = ctx
	_ = db
	return nil
}
