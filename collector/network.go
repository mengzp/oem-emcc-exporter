package collector

import (
	"context"
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

// NetworkScraper 采集主机网卡入/出流量。
// 扩展点：在 mgmt_current_metrics 中定位网卡 metric（如 HostNetwork / NetworkInBytes、NetworkOutBytes），
// 补充 interface、direction label 后在 Scrape 中输出。
type NetworkScraper struct {
	netDesc *prometheus.Desc
}

// NewNetworkScraper 构造 NetworkScraper。
func NewNetworkScraper(db *sql.DB) Scraper {
	return &NetworkScraper{
		netDesc: prometheus.NewDesc(
			"oem_host_network_bytes",
			"OEM host network bytes by interface and direction.",
			[]string{"target_guid", "host_name", "interface", "direction"}, nil,
		),
	}
}

// Name 实现 Scraper。
func (s *NetworkScraper) Name() string { return "network" }

// Help 实现 Scraper。
func (s *NetworkScraper) Help() string { return "Host network bytes by interface" }

// Scrape 实现 Scraper。
// TODO: 补充网卡 SQL，扫描 interface、direction、字节数后输出 s.netDesc。
func (s *NetworkScraper) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	_ = ctx
	_ = db
	return nil
}
