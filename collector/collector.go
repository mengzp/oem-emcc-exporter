// Package collector 对标 mysqld_exporter 的 Scraper 模型：
// 每个采集域实现 Scraper 接口，由 Builder 统一包装为 prometheus.Collector。
// 指标名、label、SQL 固化在各 Scraper 代码中，运行时仅通过开关启用/停用。
package collector

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Scraper 单个采集器，对标 mysqld_exporter 的 scraper。
type Scraper interface {
	// Name 采集器唯一标识，用于 --collect.<name> 开关与自监控 label。
	Name() string
	// Help 采集器说明。
	Help() string
	// Scrape 执行一次采集，把指标推入 ch。
	Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error
}

// Factory 绑定采集器名称与构造函数。
type Factory struct {
	Name string
	New  func(db *sql.DB) Scraper
}

// Registry 全部已注册的 Scraper 工厂，新增采集域在此追加。
var Registry = []Factory{
	{"target", NewTargetScraper},
	{"host", NewHostScraper},
	{"disk", NewDiskScraper},
	{"network", NewNetworkScraper},
	{"process", NewProcessScraper},
}

// Builder 把一组启用的 Scraper 包装成标准 prometheus.Collector，
// 并按采集器维度上报耗时与错误自监控指标。
type Builder struct {
	db           *sql.DB
	scrapers     []Scraper
	queryTimeout time.Duration

	durationDesc *prometheus.Desc
	errorDesc    *prometheus.Desc
}

// NewBuilder 构造 Collector，只包含 enabled 名单内的 Scraper。
func NewBuilder(db *sql.DB, enabled map[string]bool, queryTimeout time.Duration) *Builder {
	b := &Builder{
		db:           db,
		queryTimeout: queryTimeout,
		durationDesc: prometheus.NewDesc(
			"oem_collector_scrape_duration_seconds",
			"Duration of one scraper scrape.",
			[]string{"collector"}, nil,
		),
		errorDesc: prometheus.NewDesc(
			"oem_collector_scrape_error",
			"1 if the scraper scrape failed, 0 otherwise.",
			[]string{"collector"}, nil,
		),
	}
	for _, f := range Registry {
		if !enabled[f.Name] {
			continue
		}
		b.scrapers = append(b.scrapers, f.New(db))
	}
	return b
}

// Describe 实现 prometheus.Collector。Scraper 指标在 Collect 时动态生成，
// 这里只注册自监控描述。
func (b *Builder) Describe(ch chan<- *prometheus.Desc) {
	ch <- b.durationDesc
	ch <- b.errorDesc
}

// Collect 实现 prometheus.Collector：逐个调用 Scraper，错误不中断其他采集器。
func (b *Builder) Collect(ch chan<- prometheus.Metric) {
	for _, s := range b.scrapers {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), b.queryTimeout)
		err := s.Scrape(ctx, b.db, ch)
		cancel()

		errVal := 0.0
		if err != nil {
			errVal = 1.0
		}
		ch <- prometheus.MustNewConstMetric(
			b.durationDesc, prometheus.GaugeValue, time.Since(start).Seconds(), s.Name(),
		)
		ch <- prometheus.MustNewConstMetric(
			b.errorDesc, prometheus.GaugeValue, errVal, s.Name(),
		)
	}
}
