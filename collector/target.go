package collector

import (
	"context"
	"database/sql"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// TargetScraper 采集数据库目标（rac_database / oracle_database）的
// 可用性状态、版本与未关闭 incident 计数。
type TargetScraper struct {
	statusDesc   *prometheus.Desc
	incidentDesc *prometheus.Desc
}

// NewTargetScraper 构造 TargetScraper。
func NewTargetScraper(db *sql.DB) Scraper {
	return &TargetScraper{
		statusDesc: prometheus.NewDesc(
			"oem_target_status",
			"OEM target availability: 1=UP, 0=DOWN, 2=UNKNOWN.",
			[]string{"target_guid", "target_name", "target_type", "host_name", "target_version"}, nil,
		),
		incidentDesc: prometheus.NewDesc(
			"oem_target_incident_count",
			"Count of open OEM incidents by severity.",
			[]string{"target_guid", "target_name", "target_type", "host_name", "severity"}, nil,
		),
	}
}

// targetSQL EM13.5 sysman 视图。上线前请在 OMS 库核对列名与 severity 枚举。
const targetSQL = `
SELECT
    t.target_guid,
    t.target_name,
    t.target_type,
    t.host_name,
    t.target_version,
    a.availability_status,
    COALESCE(SUM(CASE WHEN i.severity = 'FATAL'    THEN 1 ELSE 0 END), 0) AS fatal_cnt,
    COALESCE(SUM(CASE WHEN i.severity = 'CRITICAL' THEN 1 ELSE 0 END), 0) AS critical_cnt,
    COALESCE(SUM(CASE WHEN i.severity = 'WARNING'  THEN 1 ELSE 0 END), 0) AS warning_cnt
FROM sysman.mgmt_targets t
LEFT JOIN sysman.mgmt_target_availability a
       ON t.target_guid = a.target_guid
LEFT JOIN sysman.mgmt_incidents i
       ON t.target_guid = i.target_guid
      AND i.status = 'OPEN'
WHERE t.target_type IN ('rac_database', 'oracle_database')
  AND t.target_status = 1
GROUP BY t.target_guid, t.target_name, t.target_type, t.host_name,
         t.target_version, a.availability_status
`

// Name 实现 Scraper。
func (s *TargetScraper) Name() string { return "target" }

// Help 实现 Scraper。
func (s *TargetScraper) Help() string { return "Database target status, version and open incidents" }

// Scrape 实现 Scraper。
func (s *TargetScraper) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.QueryContext(ctx, targetSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			guid, name, targetType, hostName string
			version, availStatus             sql.NullString
			fatal, critical, warning          sql.NullInt64
		)
		if err := rows.Scan(&guid, &name, &targetType, &hostName, &version,
			&availStatus, &fatal, &critical, &warning); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(
			s.statusDesc, prometheus.GaugeValue,
			mapAvailability(availStatus.String),
			guid, name, targetType, hostName, version.String,
		)

		labels := []string{guid, name, targetType, hostName}
		ch <- prometheus.MustNewConstMetric(s.incidentDesc, prometheus.GaugeValue,
			float64(nullInt(fatal)), append(labels, "fatal")...)
		ch <- prometheus.MustNewConstMetric(s.incidentDesc, prometheus.GaugeValue,
			float64(nullInt(critical)), append(labels, "critical")...)
		ch <- prometheus.MustNewConstMetric(s.incidentDesc, prometheus.GaugeValue,
			float64(nullInt(warning)), append(labels, "warning")...)
	}
	return rows.Err()
}

// mapAvailability 将 OEM 可用性字符串映射为 gauge 取值。
func mapAvailability(v string) float64 {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "UP":
		return 1
	case "DOWN":
		return 0
	default:
		return 2
	}
}

func nullInt(v sql.NullInt64) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}
