package collector

import (
	"context"
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

// DiskScraper 采集主机磁盘使用率（按挂载点）。
// 扩展点：在 mgmt_current_metrics 中定位磁盘 metric（如 HostDisk / DiskUtilizationPct），
// 补充 mountpoint label 后在 Scrape 中输出。
type DiskScraper struct {
	diskDesc *prometheus.Desc
}

// NewDiskScraper 构造 DiskScraper。
func NewDiskScraper(db *sql.DB) Scraper {
	return &DiskScraper{
		diskDesc: prometheus.NewDesc(
			"oem_host_disk_util_pct",
			"OEM host disk utilization percent by mountpoint.",
			[]string{"target_guid", "host_name", "mountpoint"}, nil,
		),
	}
}

// Name 实现 Scraper。
func (s *DiskScraper) Name() string { return "disk" }

// Help 实现 Scraper。
func (s *DiskScraper) Help() string { return "Host disk utilization by mountpoint" }

// Scrape 实现 Scraper。
// TODO: 补充磁盘 SQL（metric_name='HostDisk'），扫描 mountpoint 与使用率后输出 s.diskDesc。
func (s *DiskScraper) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	_ = ctx
	_ = db
	return nil
}
