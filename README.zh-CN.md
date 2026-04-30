# Nexis 中文说明

Nexis 是一个 TCP 双向网络质量监控系统 MVP，用于观察本地探针到远端节点的去程直连质量，以及在 Probe 没有公网 IPv4 时通过 Console 实现的回程逻辑质量。

## 组件

- Nexis Console：控制中心，提供 Web 控制台、REST API、Agent 控制通道、调度器和反向逻辑端口。
- Nexis Node：被测远端节点，开启 TCP 测试监听端口，并执行回程逻辑 TCP 测试。
- Nexis Probe：本地探针，主动连接 Console，执行 Probe 到 Node 的去程直连测试，并响应逻辑回程流。

## 快速启动

Console 脚本会自动通过 Docker Compose 启动 PostgreSQL，并把 Agent、session、系统快照、TCP 测试结果和 reverse port 分配写入数据库。

脚本会优先使用 Docker 启动 PostgreSQL；如果没有 Docker，会尝试使用 Homebrew 的 PostgreSQL 可执行文件。如果你已有 PostgreSQL，可按“数据库存储”一节设置 `NEXIS_DATABASE_DSN`。

```bash
./start-console.sh
```

再开两个终端：

```bash
./start-node.sh
./start-probe.sh
```

访问：

```text
http://127.0.0.1:47131
```

## 默认端口

| 模块 | 端口 |
|---|---:|
| Nexis Console Web | 47131 |
| Nexis Console API | 47141 |
| Nexis Console Control | 47151 |
| Nexis Console Metrics / Debug | 47191 |
| Nexis Node TCP Test Listener | 47211 |
| Nexis Probe Reverse Tunnel Pool | 47300-47999 |

## 已实现能力

- Agent 注册与 token 鉴权
- 心跳、lease TTL、在线/延迟/离线状态
- 旧 session 替换
- Console 周期调度去程直连和回程逻辑测试
- TCP JSON Lines probe 协议
- 去程指标：connect latency、app RTT、jitter、success rate、timeout rate、probe loss rate
- 回程逻辑指标：connect latency、app RTT、jitter、success rate、timeout rate、probe loss rate
- Probe 反向逻辑端口分配
- Node 列表页、Node 详情页、资源曲线、连接监测曲线
- PostgreSQL 持久化存储
- YAML 配置样例、systemd 样例、PostgreSQL migration、OpenAPI 草案

## 常用命令

```bash
make test
make build
make run-console
make run-node
make run-probe
```

## Docker / systemd

Docker Compose 与 systemd 示例在 `deploy/` 下：

```bash
docker compose -f deploy/docker-compose.yml up --build
```

systemd 服务示例会从 `/etc/nexis/*.yaml` 读取配置。

## 调试接口

```text
http://127.0.0.1:47191/metrics
http://127.0.0.1:47191/debug/state
```

## 数据库存储

默认配置使用：

```text
postgres://nexis:nexis@127.0.0.1:55432/nexis?sslmode=disable
```

`./start-console.sh` 会自动启动 PostgreSQL：优先用 Docker Compose，否则尝试用 Homebrew PostgreSQL 在 `data/postgres` 初始化本地数据库。要使用已有数据库：

```bash
NEXIS_SKIP_DB_START=1 \
NEXIS_DATABASE_DSN='postgres://user:pass@host:5432/nexis?sslmode=disable' \
./start-console.sh
```
