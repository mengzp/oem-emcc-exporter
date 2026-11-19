# oem-emcc-exporter

从 Oracle EMCC 13.5 OMS Repository（`sysman` schema）读取数据库目标状态、incident 与主机实时指标，
输出标准 Prometheus 指标。代码结构对标 [`mysqld_exporter`](https://github.com/prometheus/mysqld_exporter)：
每个采集域是一个独立 `Scraper`，连接与运行参数由 YAML 配置文件承载。

## 目录结构

```
oem-emcc-exporter/
├── collector/
│   ├── collector.go      # Scraper 接口、Registry、Builder（包装为 prometheus.Collector + 自监控）
│   ├── target.go         # DB target 状态、版本、OPEN incident 计数
│   ├── host.go           # 主机 CPU / 内存利用率
│   ├── disk.go           # 磁盘使用率（占位，待补 SQL）
│   ├── network.go        # 网卡流量（占位，待补 SQL）
│   └── process.go        # 进程 CPU / 内存（占位，待补 SQL）
├── config/
│   └── config.go         # YAML 配置加载、默认值、采集器开关
├── deploy/
│   ├── oem-exporter.service  # systemd 单元
│   └── prometheus.yml         # Prometheus scrape 配置示例
├── oem-exporter.yml      # 运行配置示例（监听、DSN、超时、采集器开关）
├── main.go                # 入口：加载配置、建连接池、注册 Scraper、启动 /metrics
├── Makefile               # Linux 静态构建与打包
└── go.mod
```

## 与 mysqld_exporter 的对应关系

| mysqld_exporter | 本项目 |
|---|---|
| `Scraper` 接口（Name/Help/Scrape） | `collector.Scraper` |
| 内置 scraper 列表 | `collector.Registry` |
| `--collect.<name>` / `--no-collect.<name>` | 同名 flag，默认跟随配置文件 |
| 自定义 query 配置 | 本项目不做配置化指标；指标/SQL 固化在 Scraper 代码中 |
| 单 DB 连接采集 | 单连接 OMS Repository，不直连 500+ 业务库 |

## 配置文件 `oem-exporter.yml`

```yaml
web:
  listen_address: ":9161"
  telemetry_path: /metrics

database:
  # go-ora DSN；生产建议留空，用环境变量 OMS_DSN 注入，避免密码明文落盘
  dsn: "oracle://mon_exporter:CHANGE_ME@oms-scan:1521/EMREP"
  max_open_conns: 5
  max_idle_conns: 2
  query_timeout: 30s

collectors:
  target: true
  host: true
  disk: true
  network: true
  process: true
```

- `database.dsn` 可被环境变量 `OMS_DSN` 覆盖。
- `collectors` 留空表示全部启用；命令行 `--collect.<name>` / `--no-collect.<name>` 优先级高于配置文件。

## 指标清单

| 指标 | 类型 | 说明 | label |
|---|---|---|---|
| `oem_target_status` | gauge | 目标可用性：1=UP, 0=DOWN, 2=UNKNOWN | target_guid, target_name, target_type, host_name, target_version |
| `oem_target_incident_count` | gauge | 未关闭 incident 数，按严重级别 | target_guid, target_name, target_type, host_name, severity(fatal/critical/warning) |
| `oem_host_metric_value` | gauge | 主机实时指标值 | target_guid, host_name, metric, metric_column |
| `oem_host_disk_util_pct` | gauge | 磁盘使用率（待补） | target_guid, host_name, mountpoint |
| `oem_host_network_bytes` | gauge | 网卡流量（待补） | target_guid, host_name, interface, direction |
| `oem_host_process_cpu_pct` / `oem_host_process_mem_pct` | gauge | 进程占用（待补） | target_guid, host_name, process_name |
| `oem_collector_scrape_duration_seconds` | gauge | 单个 Scraper 采集耗时 | collector |
| `oem_collector_scrape_error` | gauge | 采集是否失败：1=失败, 0=正常 | collector |

## 前置授权（OMS 库）

```sql
GRANT SELECT ON sysman.mgmt_targets            TO mon_exporter;
GRANT SELECT ON sysman.mgmt_target_availability TO mon_exporter;
GRANT SELECT ON sysman.mgmt_incidents          TO mon_exporter;
GRANT SELECT ON sysman.mgmt_current_metrics    TO mon_exporter;
```

> ⚠️ 上线前在 OMS 库手工核对：可用性取值枚举、`severity` 枚举、主机 `metric_name/metric_column`；
> `sysman` 为内部视图，EMCC 升级或打 PSU 后需回归 SQL。

## 构建（RHEL/CentOS 8, x86_64）

`CGO_ENABLED=0` 产出静态链接二进制，不依赖目标机 glibc 版本：

```bash
# 本机直接构建
make build

# 构建并打 tar.gz（二进制 + 配置示例 + README）
make tarball VERSION=1.0.0

# 跨架构示例（arm64）
make build GOARCH=arm64
```

产物：`oem-emcc-exporter`（静态 ELF），打包文件 `oem-emcc-exporter-<VERSION>.linux-amd64.tar.gz`。

### 在 CentOS/RHEL 8 部署

```bash
tar -xzf oem-emcc-exporter-1.0.0.linux-amd64.tar.gz
cp oem-emcc-exporter-1.0.0.linux-amd64/oem-emcc-exporter /usr/local/bin/
mkdir -p /etc/oem-exporter
cp oem-exporter.yml /etc/oem-exporter/
# 编辑 /etc/oem-exporter/oem-exporter.yml 填入 DSN，或写入 /etc/oem-exporter/oem-exporter.env:
#   OMS_DSN=oracle://mon_exporter:******@oms-scan:1521/EMREP
cp deploy/oem-exporter.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now oem-exporter
```

校验：

```bash
curl -s localhost:9161/metrics | grep oem_
```

## Prometheus 采集

见 `deploy/prometheus.yml`，建议 `scrape_interval: 5m`。

## 新增一个 Scraper

1. 在 `collector/` 新建 `xxx.go`，实现 `Scraper` 接口（`Name/Help/Scrape`）；
2. 在 `collector.Registry` 追加 `{"xxx", NewXxxScraper}`；
3. 编译后自动出现 `--collect.xxx` 开关，配置文件 `collectors.xxx` 同步生效。

## 常用 PromQL

```promql
# DOWN 的数据库目标
oem_target_status == 0

# 存在 Critical 未关闭事件的目标
oem_target_incident_count{severity="critical"} > 0

# 主机 CPU 利用率
oem_host_metric_value{metric="HostCPU", metric_column="CPUUtilization"}

# 主机内存利用率
oem_host_metric_value{metric="HostMemory", metric_column="MemoryUtilization"}
```
