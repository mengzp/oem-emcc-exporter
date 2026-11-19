// oem-emcc-exporter 从 Oracle EMCC 13.5 OMS Repository（sysman）读取
// 数据库目标状态、incident 与主机实时指标，输出标准 Prometheus 指标。
//
// 结构对标 mysqld_exporter：collector 包内每个 Scraper 独立实现采集逻辑，
// 连接与运行参数由 YAML 配置文件承载，采集器可用 --collect.<name> / --no-collect.<name> 开关。
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"oem-emcc-exporter/collector"
	"oem-emcc-exporter/config"

	_ "github.com/sijms/go-ora/v2"
)

// 构建期通过 -ldflags 注入。
var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	configFile := flag.String("config.file", "oem-exporter.yml", "configuration file path")
	listenAddr := flag.String("web.listen-address", "", "override web.listen-address from config")

	// 为每个注册的 Scraper 注册 --collect.<name> 开关，默认跟随配置文件。
	collectFlags := map[string]*bool{}
	for _, f := range collector.Registry {
		name := f.Name
		collectFlags[name] = flag.Bool("collect."+name, true, "enable "+name+" collector")
	}
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	if *listenAddr != "" {
		cfg.Web.ListenAddress = *listenAddr
	}

	timeout, err := cfg.QueryTimeoutDuration()
	if err != nil {
		log.Fatalf("invalid query.timeout: %v", err)
	}

	if cfg.Database.DSN == "" {
		log.Fatalf("database.dsn is empty: set it in config file or OMS_DSN env")
	}

	omsDB, err := sql.Open("oracle", cfg.Database.DSN)
	if err != nil {
		log.Fatalf("open OMS repository failed: %v", err)
	}
	omsDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	omsDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	omsDB.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := omsDB.PingContext(pingCtx); err != nil {
		cancel()
		log.Fatalf("ping OMS repository failed: %v", err)
	}
	cancel()
	defer omsDB.Close()

	// 开关优先级：命令行 --collect.<name> > 配置文件 collectors。
	enabled := map[string]bool{}
	for _, f := range collector.Registry {
		on := cfg.Enabled(f.Name)
		if fl := flag.Lookup("collect." + f.Name); fl != nil && fl.Value.String() != fl.DefValue {
			on = *collectFlags[f.Name]
		}
		enabled[f.Name] = on
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	registry.MustRegister(collector.NewBuilder(omsDB, enabled, timeout))

	mux := http.NewServeMux()
	mux.Handle(cfg.Web.TelemetryPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>OEM EMCC Exporter ` + version + `</title></head>` +
			`<body><h1>OEM EMCC Exporter</h1><p><a href="` + cfg.Web.TelemetryPath + `">` +
			cfg.Web.TelemetryPath + `</a></p></body></html>`))
	})

	srv := &http.Server{Addr: cfg.Web.ListenAddress, Handler: mux}
	go func() {
		log.Printf("oem-emcc-exporter %s (build %s) listening on %s%s",
			version, buildDate, cfg.Web.ListenAddress, cfg.Web.TelemetryPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http serve failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down exporter ...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
}
