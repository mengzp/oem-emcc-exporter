package collector

import (
	"context"
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

// HostScraper 采集主机（host target）的 CPU、内存利用率等实时指标。
type HostScraper struct {
	metricDesc *prometheus.Desc
}

// NewHostScraper 构造 HostScraper。
func NewHostScraper(db *sql.DB) Scraper {
	return &HostScraper{
		metricDesc: prometheus.NewDesc(
			"oem_host_metric_value",
			"OEM host real-time metric value from mgmt_current_metrics.",
			[]string{"target_guid", "host_name", "metric", "metric_column"}, nil,
		),
	}
}

// hostSQL EM13.5 主机 CPU / 内存。metric_name、metric_column 以实际 EM 定义为准。
const hostSQL = `
SELECT
    t.target_guid,
    t.target_name,
    m.metric_name,
    m.metric_column,
    TO_NUMBER(m.metric_value)
FROM sysman.mgmt_targets t
JOIN sysman.mgmt_current_metrics m
  ON t.target_guid = m.target_guid
WHERE t.target_type = 'host'
  AND m.metric_name IN ('HostCPU', 'HostMemory')
  AND m.metric_column IN ('CPUUtilization', 'MemoryUtilization')
`

// Name 实现 Scraper。
func (s *HostScraper) Name() string { return "host" }

// Help 实现 Scraper。
func (s *HostScraper) Help() string { return "Host CPU and memory utilization" }

// Scrape 实现 Scraper。
func (s *HostScraper) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.QueryContext(ctx, hostSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			guid, hostName, metricName, metricColumn string
			value                                    sql.NullFloat64
		)
		if err := rows.Scan(&guid, &hostName, &metricName, &metricColumn, &value); err != nil {
			return err
		}
		if !value.Valid {
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			s.metricDesc, prometheus.GaugeValue, value.Float64,
			guid, hostName, metricName, metricColumn,
		)
	}
	return rows.Err()
}
