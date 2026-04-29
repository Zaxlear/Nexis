# Nexis TCP 双向监控系统 Vibe Coding 开发文档

> 项目名：**Nexis**  
> 核心组件：**Nexis Console / Nexis Node / Nexis Probe**  
> 版本：MVP v1.1  
> 面向对象：AI 编码助手（Codex / Claude Code / Cursor / Gemini CLI 等）+ 人类开发者  
> 文档目标：作为 AI 进行 vibe coding 的完整开发说明书，帮助快速生成可运行的前端、后端与 Agent 原型。  
> UI 风格参考：用户附件中的浅色卡片式控制台界面。

---

## 1. 项目定位

**Nexis** 是一个面向服务器、家宽、本地网络、跨运营商链路的 **TCP 双向网络质量监控系统**。

它的核心目标是：

- 通过多个本地探针，持续监控它们到远端节点的 TCP 质量；
- 区分不同地域、运营商、协议栈下的链路表现；
- 展示 TCP 去程、回程的延迟、波动、失败率、超时率等指标；
- 在本地探针没有公网 IPv4 的情况下，仍然通过主动连接与反向逻辑通道完成可用的回程检测；
- 提供类似附件截图的轻量、现代、卡片式监控控制台。

---

## 2. 命名体系

Nexis 的三个核心角色如下：

| 名称 | 定位 | 原始理解 |
|---|---|---|
| **Nexis Console** | 控制中心 | 部署在 VPS 上，负责前端、后端、数据库、调度、控制通道 |
| **Nexis Node** | 被测远端节点 | 具备公网 IP 的远端服务器 / VPS / 云主机 |
| **Nexis Probe** | 本地探针 | 本地网络中的检测端，可能没有公网 IPv4 |

以后 UI、代码注释、API 字段、README、部署文档中统一使用这三个名字。

不要再使用旧概念名作为产品名；如果需要解释历史概念，只能在注释中说明。

---

## 3. 核心角色定义

## 3.1 Nexis Console

**Nexis Console** 是 Nexis 的中心控制台，通常部署在有公网 IP 的 VPS 上。

它负责：

- 托管 Web 前端；
- 提供后端 REST API；
- 提供 Agent 控制通道；
- 接收 Nexis Node / Nexis Probe 的注册、心跳、状态上报；
- 下发 TCP 测试任务；
- 存储 TCP 时序指标；
- 存储节点系统指标；
- 管理反向逻辑通道；
- 为前端提供图表数据；
- 展示所有 Node 与 Probe 的连接质量。

在 UI 上，Nexis Console 可以显示为：

```text
Nexis Console
```

---

## 3.2 Nexis Node

**Nexis Node** 是被检测的远端节点。

它通常具备公网 IP，部署在需要被观测的 VPS、云服务器、海外节点或机房机器上。

它负责：

- 主动连接 Nexis Console；
- 保持控制长连接；
- 上报基础系统资源；
- 开启 TCP 测试监听端口；
- 接收 Nexis Probe 发起的 TCP 去程探测；
- 执行对 Nexis Probe 的逻辑回程检测；
- 向 Nexis Console 上报测试结果。

在 UI 上，一个 Nexis Node 可以叫：

```text
Oracle
Tokyo-N1
LA-CN2GIA
HongKong-9929
```

---

## 3.3 Nexis Probe

**Nexis Probe** 是本地探针。

它通常部署在本地网络、家宽、公司网络、NAS、小主机、软路由、边缘设备上。

它可能没有公网 IPv4，因此不能指望外部 Node 直接主动拨入。

它负责：

- 主动连接 Nexis Console；
- 保持控制长连接常驻；
- 断线后自动重连；
- 接收 Nexis Console 下发的 TCP 测试任务；
- 主动检测自己到各个 Nexis Node 的 TCP 去程质量；
- 通过已建立的长连接配合实现逻辑回程检测；
- 上报测试结果。

在 UI 上，Nexis Probe 可以叫：

```text
广东电信IPv4
广东电信IPv6
广东移动IPv4
广东移动IPv6
广东联通IPv4
广东联通IPv6
```

---

## 4. 现实约束与产品定义

## 4.1 本地没有公网 IPv4 时的问题

如果 Nexis Probe 没有公网 IPv4，Nexis Node 无法直接通过公网 IPv4 主动新建 TCP 连接到这个 Probe。

也就是说，这种路径通常不可行：

```text
Nexis Node -> Nexis Probe:某个公网IPv4端口
```

原因是 Nexis Probe 可能在：

- 家用路由器 NAT 后面；
- 运营商 CGNAT 后面；
- 公司内网；
- 无入站端口映射的局域网中。

所以 MVP 不能假设 Probe 一定可以被公网 TCP 直接打进来。

---

## 4.2 Nexis 的回程定义

Nexis 当前只做 TCP，因此先定义两类方向：

### A. 去程（直连）

```text
Nexis Probe -> Nexis Node
```

这是 Probe 主动连接 Node 的测试端口，是 **真实直连路径**。

它用于观察：

- 本地网络到远端节点的 TCP connect 延迟；
- 应用层 RTT；
- 抖动；
- 连接失败率；
- 超时率；
- TCP 探测失败率。

### B. 回程（逻辑）

```text
Nexis Node -> Nexis Console -> Nexis Probe
```

这是 Node 通过 Console 提供的反向逻辑通道访问 Probe。

它不是严格意义上的纯公网直连回程，但它能解决 Probe 没有公网 IPv4 时的反向测试问题。

在 UI 中建议显示为：

```text
回程（逻辑）
Reverse via Console
```

未来如果 Probe 具备公网 IPv4 / 公网 IPv6 / 手动端口映射，可以扩展为：

```text
回程（直连）
Reverse Direct
```

---

## 5. MVP 范围

## 5.1 必须实现

### Nexis Console

- Agent 注册；
- Agent 鉴权；
- 心跳管理；
- 在线 / 离线状态；
- 测试任务调度；
- 测试结果接收；
- 指标存储；
- 图表查询 API；
- Probe 反向逻辑端口管理；
- Web 控制台。

### Nexis Node

- 主动连接 Nexis Console；
- 断线自动重连；
- 上报系统信息；
- 开启 TCP 测试监听器；
- 响应 Probe 的 TCP probe；
- 对 Console 分配的 Probe 逻辑端口发起反向测试；
- 上报回程逻辑测试结果。

### Nexis Probe

- 主动连接 Nexis Console；
- 保持长连接常驻；
- 断线自动重连；
- 执行 Probe -> Node 的 TCP 直连测试；
- 通过与 Console 的长连接响应 Node 的逻辑回程 probe；
- 上报去程测试结果。

### Web UI

- Node 列表页；
- Node 详情页；
- Node 基础信息卡片；
- CPU / 内存 / 磁盘 / 网络流量折线图；
- Node 到各个 Probe 的连接质量面板；
- 时间范围切换：`1小时 / 6小时 / 24小时 / 7天`。

---

## 5.2 暂不做

MVP 阶段暂不实现：

- UDP 检测；
- ICMP ping；
- TCP hole punching；
- 复杂 NAT 穿透；
- 多租户 RBAC；
- 告警推送；
- 地图视图；
- eBPF 抓包；
- App 客户端。

---

## 6. 推荐技术栈

## 6.1 Monorepo 目录结构

```text
nexis/
  apps/
    web/                    # Nexis Console 前端
    console/                # Nexis Console 后端 API + 控制面
  agents/
    nexis-node/             # Nexis Node Agent
    nexis-probe/            # Nexis Probe Agent
  packages/
    proto/                  # JSON 协议、共享类型、OpenAPI 文档
  deploy/
    docker-compose.yml
    migrations/
    systemd/
  docs/
    architecture.md
    api.md
    agent-protocol.md
```

---

## 6.2 前端

推荐：

- Next.js 15
- React
- TypeScript
- Tailwind CSS
- shadcn/ui
- ECharts
- TanStack Query

---

## 6.3 Nexis Console 后端

推荐：

- Go 1.24+
- Fiber 或 Gin，优先 Fiber
- WebSocket 控制通道
- PostgreSQL 16+
- TimescaleDB 可选
- sqlc 或 gorm，优先 sqlc

---

## 6.4 Agent

推荐：

- Go 1.24+
- 单二进制发布
- YAML 配置
- systemd 托管
- 支持 Linux amd64 / arm64
- 后续可支持 macOS / Windows

---

## 7. 端口规划

要求：所有端口必须使用 **高位、不常用端口号**，并且全部可配置。

| 模块 | 默认端口 | 说明 |
|---|---:|---|
| Nexis Console Web | 47131 | Web 前端 |
| Nexis Console API | 47141 | REST API |
| Nexis Console Control | 47151 | Node / Probe 控制通道，建议 WSS |
| Nexis Node TCP Test Listener | 47211 | Probe 发起 TCP 去程测试的目标端口 |
| Nexis Probe Reverse Tunnel Pool | 47300-47999 | Console 为 Probe 分配的逻辑回程端口池 |
| Nexis Console Metrics / Debug | 47191 | 内部调试，可选 |

要求：

- 不使用 80 / 443 / 22 / 3306 / 5432 / 8080 等常见业务端口作为默认 Agent 端口；
- 生产部署如需走 443，可由 Nginx / Caddy 反代到高位端口；
- Node 测试端口必须可自定义；
- Probe 逻辑回程端口必须从端口池动态分配。

---

## 8. 总体架构

```mermaid
flowchart LR
    U[User Browser]

    subgraph NC["Nexis Console on VPS"]
        WEB[Web Frontend]
        API[REST API]
        CTRL[Control Hub]
        TUN[Reverse Tunnel Manager]
        DB[(PostgreSQL / TimescaleDB)]
    end

    subgraph NP["Nexis Probe"]
        P1[广东电信IPv4]
        P2[广东移动IPv6]
        P3[广东联通IPv4]
    end

    subgraph NN["Nexis Node"]
        N1[Oracle]
        N2[Tokyo-N1]
        N3[LA-CN2GIA]
    end

    U --> WEB
    WEB --> API
    API --> DB
    API --> CTRL
    CTRL --> TUN

    P1 -. WSS Control .-> CTRL
    P2 -. WSS Control .-> CTRL
    P3 -. WSS Control .-> CTRL

    N1 -. WSS Control .-> CTRL
    N2 -. WSS Control .-> CTRL
    N3 -. WSS Control .-> CTRL

    P1 -->|Forward TCP Direct| N1
    P1 -->|Forward TCP Direct| N2
    P2 -->|Forward TCP Direct| N1

    N1 -->|Reverse TCP Logical| TUN
    TUN -->|Existing Probe Connection| P1
```

---

## 9. 控制通道设计

## 9.1 基本原则

Nexis Node 和 Nexis Probe 都必须主动连接 Nexis Console。

推荐使用：

```text
WSS over TCP
```

原因：

- 易于调试；
- 便于穿过防火墙；
- 适合 AI 快速生成代码；
- 后续可以平滑升级为自定义 TCP + TLS 协议。

---

## 9.2 长连接要求

所有 Agent 控制连接必须实现：

- 注册；
- 鉴权；
- 心跳；
- lease TTL；
- 自动重连；
- 旧 session 替换；
- 任务幂等；
- 写队列超时检测；
- 应用层 heartbeat；
- 系统层 TCP keepalive。

---

## 9.3 心跳与重连策略

默认参数：

```yaml
heartbeat_interval_sec: 15
heartbeat_timeout_sec: 45
lease_ttl_sec: 90
reconnect_min_backoff_sec: 1
reconnect_max_backoff_sec: 30
reconnect_jitter_ratio: 0.2
```

策略：

1. Agent 每 15 秒发送一次 heartbeat；
2. Console 超过 45 秒没有收到 heartbeat，将连接标记为 `stale`；
3. Console 超过 90 秒没有收到 heartbeat，将 Agent 标记为 `offline`；
4. Agent 发现连接断开后立即重连；
5. 重连使用指数退避：`1s -> 2s -> 4s -> 8s -> 15s -> 30s`；
6. 每次退避增加 `0~20%` 随机抖动；
7. 成功重连后退避状态清零；
8. 同一个 Agent 重新上线后，新 session 替换旧 session；
9. Console 只认最后一个活跃 session；
10. 被替换的旧 session 标记为 `superseded`。

---

## 10. TCP 测试类型

MVP 只做 TCP。

## 10.1 去程直连测试

方向：

```text
Nexis Probe -> Nexis Node
```

流程：

1. Console 下发 `forward_direct_tcp` 任务给 Probe；
2. Probe 连接 Node 的 TCP 测试端口；
3. Probe 记录 `connect()` 耗时；
4. Probe 发送多次小型 probe payload；
5. Node 立即返回 echo ack；
6. Probe 计算 RTT、抖动、失败率；
7. Probe 将结果上报给 Console；
8. Console 入库并给前端图表查询。

---

## 10.2 回程逻辑测试

方向：

```text
Nexis Node -> Nexis Console -> Nexis Probe
```

流程：

1. Probe 连接 Console 并注册；
2. Console 为 Probe 分配一个 reverse port，例如 `47321`；
3. Console 维护 `reverse_port -> probe_session` 映射；
4. Console 下发 `reverse_logical_tcp` 任务给 Node；
5. Node 连接 `Console_IP:47321`；
6. Console 将这个 TCP stream 桥接到 Probe 的长连接；
7. Probe 收到 probe payload 后返回 ack；
8. Node 计算回程逻辑 RTT、抖动、失败率；
9. Node 将结果上报给 Console。

UI 上必须明确标注：

```text
回程（逻辑）
```

避免误导用户以为这是纯公网直连回程。

---

## 11. TCP 指标定义

由于 TCP 不像 ICMP 那样直接表达“丢包”，MVP 中“丢包”建议用应用层探测失败率和超时率近似表达。

## 11.1 基础指标

| 指标 | 字段 | 说明 |
|---|---|---|
| TCP 连接延迟 | `connect_latency_ms` | `connect()` 成功耗时 |
| 应用层 RTT | `app_rtt_ms` | probe 到 ack 的往返耗时 |
| 抖动 | `jitter_ms` | 连续 RTT 样本波动 |
| 连接成功率 | `connect_success_rate` | 多次连接成功占比 |
| 超时率 | `timeout_rate` | 连接或读写超时占比 |
| 探测失败率 | `probe_loss_rate` | probe 未收到 ack 的占比 |
| TCP 重传 | `tcp_retransmissions` | Linux TCP_INFO 可选采集 |

---

## 11.2 去程指标

字段建议：

```text
forward_connect_latency_ms
forward_app_rtt_ms
forward_jitter_ms
forward_success_rate
forward_timeout_rate
forward_probe_loss_rate
forward_tcp_retransmissions
```

---

## 11.3 回程逻辑指标

字段建议：

```text
reverse_connect_latency_ms
reverse_app_rtt_ms
reverse_jitter_ms
reverse_success_rate
reverse_timeout_rate
reverse_probe_loss_rate
reverse_tcp_retransmissions
```

---

## 12. TCP Probe 协议

MVP 推荐使用 JSON Lines，易调试、易生成。

## 12.1 请求

```json
{"type":"probe","task_id":"task_123","seq":1,"ts":1711111111111,"payload":"ping"}
```

## 12.2 响应

```json
{"type":"probe_ack","task_id":"task_123","seq":1,"ts":1711111111120,"payload":"pong"}
```

## 12.3 单次测试建议

默认参数：

```yaml
probe_count: 5
connect_timeout_ms: 3000
read_timeout_ms: 3000
write_timeout_ms: 3000
interval_ms: 200
```

测试步骤：

1. 建立 TCP 连接；
2. 记录 connect 延迟；
3. 发送 5 个 probe；
4. 计算每个 probe 的应用层 RTT；
5. 统计超时、失败、抖动；
6. 关闭连接；
7. 上报聚合结果与原始样本。

---

## 13. 数据库模型

## 13.1 agents

```sql
CREATE TABLE agents (
  id UUID PRIMARY KEY,
  component_type TEXT NOT NULL CHECK (component_type IN ('node', 'probe')),
  name TEXT NOT NULL,
  region TEXT,
  isp TEXT,
  network_stack TEXT,
  public_ipv4 TEXT,
  public_ipv6 TEXT,
  status TEXT NOT NULL DEFAULT 'offline',
  auth_token_hash TEXT NOT NULL,
  last_seen_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 13.2 agent_sessions

```sql
CREATE TABLE agent_sessions (
  id UUID PRIMARY KEY,
  agent_id UUID NOT NULL REFERENCES agents(id),
  component_type TEXT NOT NULL CHECK (component_type IN ('node', 'probe')),
  connected_at TIMESTAMPTZ NOT NULL,
  last_heartbeat_at TIMESTAMPTZ,
  lease_expires_at TIMESTAMPTZ,
  disconnected_at TIMESTAMPTZ,
  is_active BOOLEAN NOT NULL DEFAULT true,
  close_reason TEXT,
  remote_addr TEXT,
  metadata JSONB DEFAULT '{}'::jsonb
);
```

---

## 13.3 node_system_snapshots

```sql
CREATE TABLE node_system_snapshots (
  id BIGSERIAL PRIMARY KEY,
  node_id UUID NOT NULL REFERENCES agents(id),
  ts TIMESTAMPTZ NOT NULL,
  cpu_usage_percent DOUBLE PRECISION,
  cpu_cores INTEGER,
  memory_total_mb DOUBLE PRECISION,
  memory_used_mb DOUBLE PRECISION,
  disk_total_gb DOUBLE PRECISION,
  disk_used_gb DOUBLE PRECISION,
  network_rx_bytes BIGINT,
  network_tx_bytes BIGINT,
  uptime_seconds BIGINT,
  os TEXT,
  arch TEXT,
  virtualization TEXT
);
```

---

## 13.4 tcp_test_results

```sql
CREATE TABLE tcp_test_results (
  id BIGSERIAL PRIMARY KEY,
  ts TIMESTAMPTZ NOT NULL,
  node_id UUID NOT NULL REFERENCES agents(id),
  probe_id UUID NOT NULL REFERENCES agents(id),
  test_type TEXT NOT NULL CHECK (test_type IN ('forward_direct_tcp', 'reverse_logical_tcp')),
  direction TEXT NOT NULL CHECK (direction IN ('forward', 'reverse')),
  path_mode TEXT NOT NULL CHECK (path_mode IN ('direct', 'tunnel')),
  connect_latency_ms DOUBLE PRECISION,
  app_rtt_ms DOUBLE PRECISION,
  jitter_ms DOUBLE PRECISION,
  success_rate DOUBLE PRECISION,
  timeout_rate DOUBLE PRECISION,
  probe_loss_rate DOUBLE PRECISION,
  tcp_retransmissions INTEGER,
  sample_count INTEGER,
  raw JSONB DEFAULT '{}'::jsonb
);
```

---

## 13.5 reverse_tunnel_ports

```sql
CREATE TABLE reverse_tunnel_ports (
  id BIGSERIAL PRIMARY KEY,
  probe_id UUID NOT NULL REFERENCES agents(id),
  port INTEGER NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'assigned',
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  released_at TIMESTAMPTZ
);
```

---

## 14. 控制通道消息协议

所有控制消息使用 JSON。

## 14.1 注册

```json
{
  "type": "register",
  "component": "nexis-probe",
  "agent_id": "8b0d9dd2-1111-2222-3333-444444444444",
  "name": "广东电信IPv4",
  "token": "replace-with-token",
  "version": "0.1.0",
  "capabilities": [
    "tcp-forward-probe",
    "reverse-logical-responder"
  ],
  "meta": {
    "os": "linux",
    "arch": "amd64"
  }
}
```

---

## 14.2 注册响应

```json
{
  "type": "register_ack",
  "session_id": "session_uuid",
  "heartbeat_interval_sec": 15,
  "lease_ttl_sec": 90,
  "server_time": 1711111111111,
  "reverse_tunnel": {
    "enabled": true,
    "listen_port": 47321
  }
}
```

---

## 14.3 心跳

```json
{
  "type": "heartbeat",
  "session_id": "session_uuid",
  "ts": 1711111111111,
  "stats": {
    "cpu_usage_percent": 12.3,
    "memory_used_mb": 1024
  }
}
```

---

## 14.4 下发去程测试任务

```json
{
  "type": "run_test",
  "job_id": "job_uuid",
  "test_type": "forward_direct_tcp",
  "node_id": "node_uuid",
  "probe_id": "probe_uuid",
  "target": {
    "host": "140.245.44.107",
    "port": 47211
  },
  "params": {
    "probe_count": 5,
    "connect_timeout_ms": 3000,
    "read_timeout_ms": 3000
  }
}
```

---

## 14.5 下发回程逻辑测试任务

```json
{
  "type": "run_test",
  "job_id": "job_uuid",
  "test_type": "reverse_logical_tcp",
  "node_id": "node_uuid",
  "probe_id": "probe_uuid",
  "target": {
    "host": "console_public_ip_or_domain",
    "port": 47321
  },
  "params": {
    "probe_count": 5,
    "connect_timeout_ms": 3000,
    "read_timeout_ms": 3000
  }
}
```

---

## 14.6 上报测试结果

```json
{
  "type": "test_result",
  "job_id": "job_uuid",
  "test_type": "forward_direct_tcp",
  "direction": "forward",
  "path_mode": "direct",
  "node_id": "node_uuid",
  "probe_id": "probe_uuid",
  "result": {
    "connect_latency_ms": 82,
    "app_rtt_ms": 96,
    "jitter_ms": 4.3,
    "success_rate": 1,
    "timeout_rate": 0,
    "probe_loss_rate": 0,
    "tcp_retransmissions": 0,
    "sample_count": 5
  },
  "raw": {
    "samples": [95, 96, 98, 92, 99]
  }
}
```

---

## 15. Nexis Console API 设计

## 15.1 查询 Node 列表

```http
GET /api/v1/nodes
```

返回：

- Node 名称；
- 在线状态；
- 公网 IPv4 / IPv6；
- OS / Arch；
- 最近心跳；
- 简要健康状态。

---

## 15.2 查询 Node 详情

```http
GET /api/v1/nodes/:nodeId
```

返回：

- Node 基础信息；
- 在线时长；
- CPU / 内存 / 磁盘 / 流量；
- 虚拟化信息；
- 平台信息；
- 最近系统快照。

---

## 15.3 查询 Node 系统指标曲线

```http
GET /api/v1/nodes/:nodeId/resource-series?range=24h&metric=cpu
```

支持 metric：

```text
cpu
memory
network
disk
```

---

## 15.4 查询 Node 与所有 Probe 的连接质量

```http
GET /api/v1/nodes/:nodeId/connectivity?range=24h
```

返回：

- Probe 名称；
- 去程 RTT；
- 去程抖动；
- 去程成功率；
- 回程逻辑 RTT；
- 回程逻辑抖动；
- 回程逻辑成功率；
- 最近测试时间；
- 健康状态。

---

## 15.5 查询某个 Probe 到某个 Node 的历史曲线

```http
GET /api/v1/nodes/:nodeId/probes/:probeId/series?range=24h&metric=app_rtt&direction=forward
```

参数：

- `range`: `1h / 6h / 24h / 7d`
- `direction`: `forward / reverse`
- `metric`: `connect_latency / app_rtt / jitter / success_rate / timeout_rate / probe_loss_rate`

---

## 16. Nexis Console UI 设计

## 16.1 总体风格

参考附件中的控制台界面：

- 浅色背景；
- 居中内容区；
- 大圆角卡片；
- 轻微阴影；
- 细边框；
- 低饱和配色；
- 高级灰背景；
- 蓝绿色作为状态点缀；
- 折线图简洁；
- 不要过度赛博朋克；
- 不要暗黑风作为默认。

建议主色：

```text
Primary: #2563EB
Success: #10B981
Warning: #F59E0B
Danger:  #EF4444
Background: #F8FAFC
Card: #FFFFFF
Border: #E5E7EB
Text: #0F172A
Muted Text: #64748B
```

---

## 16.2 页面一：Node 列表页

功能：

- 展示所有 Nexis Node；
- 支持搜索；
- 支持在线状态筛选；
- 点击 Node 进入详情页。

Node 卡片字段：

- 名称；
- 公网 IP；
- 平台；
- 在线状态；
- 最近心跳；
- 简要 RTT 状态。

---

## 16.3 页面二：Node 详情页

这是核心页面，参考附件布局。

### 顶部区域

- 返回按钮；
- Nexis Console 标识；
- 语言切换按钮，可选；
- 刷新按钮；
- 主题按钮，可选。

### 节点基础卡片

显示：

```text
节点
Oracle
这个 Nexis Node 的基础身份和连接信息。
```

标签：

- 公网 IP；
- linux/arm64；
- 在线时长；
- 在线状态 badge。

### 系统概览卡片

五个小卡片：

- 核心 / 线程；
- 内存；
- 磁盘；
- 虚拟化；
- 平台。

### 时间范围切换

```text
1小时 / 6小时 / 24小时 / 7天
```

### 资源图表区

Tabs：

```text
CPU 使用率
内存使用率
网络流量
磁盘使用率
```

### 连接监测区

标题：

```text
连接监测
当前节点与所有 Nexis Probe 的 TCP 连接质量。
```

Probe chip 示例：

```text
广东电信IPv4  去程 0% 回程 0% 延迟 40.3ms
广东移动IPv6  去程 0% 回程 10% 延迟 55.1ms
广东联通IPv4  去程 20% 回程 0% 延迟 97.7ms
```

下方折线图支持切换：

- 去程 RTT；
- 去程抖动；
- 去程失败率；
- 回程逻辑 RTT；
- 回程逻辑抖动；
- 回程逻辑失败率。

---

## 17. 前端组件拆分

建议组件：

```text
AgentStatusBadge
NodeHeaderCard
NodeSystemOverview
TimeRangeSwitch
ResourceMetricTabs
MetricLineChart
ProbeChipGroup
ConnectivitySummaryCard
ConnectivitySeriesChart
EmptyState
LoadingSkeleton
```

其中 `AgentStatusBadge` 可以同时用于 Node 和 Probe。

---

## 18. Nexis Node 设计

## 18.1 模块划分

```text
config
logger
control_client
heartbeat_manager
resource_collector
tcp_test_listener
reverse_probe_runner
result_reporter
reconnect_manager
```

---

## 18.2 启动流程

1. 读取配置；
2. 初始化日志；
3. 启动 TCP Test Listener；
4. 主动连接 Nexis Console；
5. 注册为 `nexis-node`；
6. 开始心跳；
7. 周期上报系统资源；
8. 等待 Console 下发任务；
9. 执行 reverse logical TCP 测试；
10. 上报结果。

---

## 18.3 系统信息采集

需要采集：

- CPU 核心数；
- CPU 使用率；
- 内存总量；
- 内存已用；
- 磁盘总量；
- 磁盘已用；
- 网络 RX / TX；
- uptime；
- OS；
- arch；
- virtualization。

---

## 19. Nexis Probe 设计

## 19.1 模块划分

```text
config
logger
control_client
heartbeat_manager
tcp_forward_probe
reverse_logical_responder
result_reporter
reconnect_manager
```

---

## 19.2 启动流程

1. 读取配置；
2. 初始化日志；
3. 主动连接 Nexis Console；
4. 注册为 `nexis-probe`；
5. 接收 reverse tunnel 端口分配；
6. 开始心跳；
7. 等待 Console 下发去程测试任务；
8. 执行 Probe -> Node TCP 直连测试；
9. 响应 Node -> Console -> Probe 的逻辑回程测试；
10. 断线后自动重连。

---

## 19.3 探测并发控制

Nexis Probe 可能同时检测多个 Node，所以需要：

- 全局并发上限；
- 单 Node 并发上限；
- 每个任务都有 timeout；
- 任务结果必须幂等上报；
- 连接失败要上报明确错误类型。

建议默认：

```yaml
probe_concurrency: 4
per_node_concurrency: 1
```

---

## 20. 反向逻辑通道设计

## 20.1 端口分配

Nexis Console 为每个在线 Probe 分配一个高位端口，例如：

```text
广东电信IPv4 -> 47321
广东移动IPv6 -> 47322
广东联通IPv4 -> 47323
```

Node 做回程逻辑测试时连接：

```text
Nexis Console IP:47321
```

Console 根据端口找到对应 Probe 的活跃 session，并桥接数据。

---

## 20.2 连接过期处理

如果 Probe 断开：

- reverse port 暂时标记为 unavailable；
- 新的外部连接直接拒绝或返回错误；
- Probe 重连成功后重新绑定该端口；
- 如端口已被占用，重新分配新端口并通知所有 Node。

---

## 20.3 Stream 映射

MVP 可以先做简单模式：

```text
外部 TCP 连接 -> Console -> Probe 控制连接中的逻辑 stream
```

每个 stream 需要：

- stream_id；
- probe_id；
- node_id；
- created_at；
- timeout；
- close_reason。

---

## 21. 调度设计

默认调度周期：

```yaml
node_resource_report_interval_sec: 15
forward_probe_interval_sec: 30
reverse_probe_interval_sec: 30
aggregation_interval_sec: 60
```

对于每一组 `(Node, Probe)`，Console 应周期创建两类任务：

```text
forward_direct_tcp
reverse_logical_tcp
```

示例：

```text
广东电信IPv4 -> Oracle：去程直连
Oracle -> 广东电信IPv4：回程逻辑
广东移动IPv6 -> Oracle：去程直连
Oracle -> 广东移动IPv6：回程逻辑
```

---

## 22. 状态与健康度判定

## 22.1 Agent 状态

```text
online  = 45秒内有心跳
stale   = 45秒到90秒没有心跳
offline = 超过90秒没有心跳
```

---

## 22.2 链路状态

### healthy

```text
success_rate >= 0.95
app_rtt_ms < 150
timeout_rate < 0.05
```

### degraded

```text
success_rate >= 0.8
或 app_rtt_ms 在 150~400ms
或 jitter 明显升高
```

### critical

```text
success_rate < 0.8
或 timeout_rate >= 0.2
或 connect 基本失败
```

---

## 23. 安全设计

## 23.1 Agent Token

- 每个 Node / Probe 都有独立 token；
- token 只在 Console 中保存 hash；
- Agent 注册时提交 token；
- 支持 token 轮换；
- token 不应出现在日志中。

---

## 23.2 传输安全

- 控制通道必须使用 TLS；
- 推荐 `wss://console.example.com:47151/ws`；
- 生产环境建议使用反向代理终止 TLS；
- 内部端口仍然保持高位端口。

---

## 23.3 测试端口安全

- Node 的 TCP 测试端口只响应 Nexis probe 协议；
- 可以添加简单 token / challenge；
- 避免被当作开放 echo 服务滥用；
- 建议限制 probe payload 大小。

---

## 24. 配置文件示例

## 24.1 Nexis Console

```yaml
console:
  web_port: 47131
  api_port: 47141
  control_port: 47151
  reverse_port_start: 47300
  reverse_port_end: 47999
  public_host: your-vps.example.com
  tls_cert: ./certs/server.crt
  tls_key: ./certs/server.key

database:
  dsn: postgres://nexis:nexis@127.0.0.1:5432/nexis?sslmode=disable

scheduler:
  node_resource_report_interval_sec: 15
  forward_probe_interval_sec: 30
  reverse_probe_interval_sec: 30

agent:
  heartbeat_interval_sec: 15
  heartbeat_timeout_sec: 45
  lease_ttl_sec: 90
```

---

## 24.2 Nexis Node

```yaml
agent:
  id: f54f17be-1111-2222-3333-555555555555
  component: nexis-node
  name: Oracle
  token: replace-with-token

console:
  control_url: wss://your-vps.example.com:47151/ws

listener:
  host: 0.0.0.0
  port: 47211

resource:
  report_interval_sec: 15

probe:
  connect_timeout_ms: 3000
  read_timeout_ms: 3000
  write_timeout_ms: 3000
  probe_count: 5
```

---

## 24.3 Nexis Probe

```yaml
agent:
  id: 8b0d9dd2-1111-2222-3333-444444444444
  component: nexis-probe
  name: 广东电信IPv4
  token: replace-with-token

console:
  control_url: wss://your-vps.example.com:47151/ws

probe:
  connect_timeout_ms: 3000
  read_timeout_ms: 3000
  write_timeout_ms: 3000
  probe_count: 5
  concurrency: 4

reconnect:
  min_backoff_sec: 1
  max_backoff_sec: 30
  jitter_ratio: 0.2
```

---

## 25. AI 编码任务拆分

## Milestone 1：初始化 Nexis 项目骨架

目标：

- 创建 monorepo；
- 建立 Web、Console 后端、Node、Probe 四个核心模块；
- Docker Compose 启动 PostgreSQL；
- 所有模块都有 README 和配置示例。

验收：

- `docker compose up` 可启动数据库；
- Console API 可启动；
- Web 可启动；
- Node / Probe 可编译。

---

## Milestone 2：实现 Nexis Console 控制面

目标：

- WebSocket 控制通道；
- Agent 注册；
- Agent 心跳；
- session 管理；
- 断线状态更新；
- PostgreSQL 入库。

验收：

- Node / Probe 能上线；
- Console 能显示在线状态；
- 心跳过期后自动离线。

---

## Milestone 3：实现 Nexis Node

目标：

- 连接 Console；
- 上报系统资源；
- 开启 TCP Test Listener；
- 响应 Probe 的 probe；
- 断线重连。

验收：

- Node 详情页能显示 CPU / 内存 / 磁盘 / 平台；
- Probe 能成功连接 Node 测试端口。

---

## Milestone 4：实现 Nexis Probe

目标：

- 连接 Console；
- 执行 forward direct TCP 测试；
- 统计 connect latency / app RTT / jitter / timeout / probe loss；
- 上报结果；
- 断线重连。

验收：

- Node 详情页能显示某个 Probe 到 Node 的去程曲线。

---

## Milestone 5：实现回程逻辑通道

目标：

- Console 为 Probe 分配 reverse port；
- Node 可连接 Console 的 reverse port；
- Console 将 stream 桥接到 Probe；
- Probe 返回 ack；
- Node 上报 reverse logical TCP 结果。

验收：

- Node 详情页能显示 Probe 的回程逻辑曲线；
- Probe 重连后 reverse port 能重新绑定。

---

## Milestone 6：实现前端详情页

目标：

- Node 列表页；
- Node 详情页；
- 系统概览卡片；
- 时间范围切换；
- 资源折线图；
- 连接监测折线图；
- Probe chip 切换。

验收：

- UI 风格接近附件；
- 图表可切换时间范围；
- 点击不同 Probe 能切换连接质量曲线。

---

## 26. 可直接复制给 AI 编码助手的提示词

## Prompt A：初始化项目

```text
请创建一个名为 Nexis 的 monorepo 项目。

组件命名必须使用：
- Nexis Console：控制中心，部署在 VPS 上，负责前端、API、控制通道、数据库和调度。
- Nexis Node：被检测的远端节点，具备公网 IP，开启 TCP 测试监听端口。
- Nexis Probe：本地探针，可能没有公网 IPv4，主动连接 Nexis Console，检测到各个 Nexis Node 的 TCP 质量。

目录结构：
- apps/web：Next.js + TypeScript + Tailwind + shadcn/ui + ECharts
- apps/console：Go + Fiber + WebSocket + PostgreSQL
- agents/nexis-node：Go
- agents/nexis-probe：Go
- packages/proto：共享协议定义
- deploy：Docker Compose、SQL migration、systemd 示例

所有默认端口使用高位不常用端口：47131、47141、47151、47211、47300-47999。
```

---

## Prompt B：实现 Nexis Console

```text
请实现 Nexis Console 的 Go 后端：
- 提供 REST API
- 提供 WebSocket 控制通道
- 支持 Nexis Node / Nexis Probe 注册
- 支持 token 鉴权
- 支持 heartbeat
- 支持 session replace
- 心跳间隔 15 秒，lease TTL 90 秒
- 支持 PostgreSQL 存储 agents、sessions、node_system_snapshots、tcp_test_results
- 支持为 Nexis Probe 分配 reverse tunnel 高位端口
- 支持下发 forward_direct_tcp 和 reverse_logical_tcp 测试任务
```

---

## Prompt C：实现 Nexis Node

```text
请实现 Nexis Node：
- 使用 Go 编写
- 启动后主动连接 Nexis Console 控制通道
- 注册 component=nexis-node
- 定期上报 CPU、内存、磁盘、网络流量、uptime、os、arch、virtualization
- 在高位端口 47211 开启 TCP probe listener
- 收到 JSON Lines probe 后立即返回 probe_ack
- 支持执行 reverse_logical_tcp：连接 Nexis Console 为某个 Nexis Probe 分配的 reverse port，测量 connect latency、app RTT、jitter、timeout rate、probe loss rate
- 所有连接必须有 timeout
- 断线后自动重连
```

---

## Prompt D：实现 Nexis Probe

```text
请实现 Nexis Probe：
- 使用 Go 编写
- 启动后主动连接 Nexis Console 控制通道
- 注册 component=nexis-probe
- 保持长连接常驻
- 支持 heartbeat、lease、自动重连、指数退避、session 替换
- 支持 forward_direct_tcp：主动连接 Nexis Node 的 TCP 测试端口，测量 connect latency、app RTT、jitter、timeout rate、probe loss rate
- 支持 reverse_logical_responder：通过与 Nexis Console 的长连接响应 Nexis Node 的逻辑回程 probe
- 测试结果通过控制通道上报 Nexis Console
```

---

## Prompt E：实现 Nexis Console 前端

```text
请实现 Nexis Console 的前端页面，使用 Next.js + TypeScript + Tailwind + shadcn/ui + ECharts。

UI 风格参考浅色卡片式监控控制台：
- 大圆角卡片
- 浅灰背景
- 轻阴影
- 低饱和配色
- 简洁折线图

页面要求：
1. Nexis Node 列表页
2. Nexis Node 详情页

Node 详情页包含：
- 返回按钮
- Node 名称、IP、平台、在线状态、在线时长
- 系统概览卡片：核心/线程、内存、磁盘、虚拟化、平台
- 时间范围切换：1小时、6小时、24小时、7天
- 资源图表 Tab：CPU 使用率、内存使用率、网络流量、磁盘使用率
- 连接监测区域：显示多个 Nexis Probe chip，例如广东电信IPv4、广东移动IPv6
- 选中某个 Probe 后显示它与当前 Node 的去程/回程逻辑曲线
- 支持指标：去程 RTT、去程抖动、去程失败率、回程逻辑 RTT、回程逻辑抖动、回程逻辑失败率
```

---

## 27. 验收清单

## 27.1 基础启动

- [ ] Nexis Console 可启动；
- [ ] Nexis Console Web 可访问；
- [ ] PostgreSQL 可连接；
- [ ] Nexis Node 可启动并上线；
- [ ] Nexis Probe 可启动并上线。

## 27.2 去程直连 TCP

- [ ] Probe 能连接 Node 的 47211 端口；
- [ ] 能测量 connect latency；
- [ ] 能测量 app RTT；
- [ ] 能计算 jitter；
- [ ] 能统计 timeout rate；
- [ ] 能统计 probe loss rate；
- [ ] 结果能入库；
- [ ] 前端能显示去程曲线。

## 27.3 回程逻辑 TCP

- [ ] Console 能为 Probe 分配 reverse port；
- [ ] Node 能连接 Console 的 reverse port；
- [ ] Console 能桥接到 Probe；
- [ ] Probe 能返回 probe_ack；
- [ ] Node 能上报回程逻辑指标；
- [ ] 前端能显示回程逻辑曲线。

## 27.4 长连接可靠性

- [ ] Agent 断网后自动重连；
- [ ] Console 重启后 Agent 自动重连；
- [ ] 心跳过期后状态变成 offline；
- [ ] 新 session 替换旧 session；
- [ ] Probe 重连后 reverse port 重新绑定；
- [ ] 正在执行的任务失败时能明确上报错误。

## 27.5 UI

- [ ] Node 列表页可用；
- [ ] Node 详情页可用；
- [ ] 系统概览卡片正常显示；
- [ ] 资源图表可切换；
- [ ] Probe chip 可切换；
- [ ] 去程/回程逻辑图表可切换；
- [ ] 风格接近附件截图。

---

## 28. 后续扩展方向

后续可以扩展：

1. UDP 检测；
2. ICMP ping；
3. IPv4 / IPv6 独立策略；
4. Probe 公网可达时的真实回程直连检测；
5. 告警通知；
6. 链路健康评分；
7. 多用户与权限；
8. 运营商与地域分组；
9. 节点地图；
10. 更高级的多路复用隧道协议；
11. TCP_INFO 深度指标；
12. BBR / 拥塞控制状态展示。

---

## 29. 最终开发原则

AI 编码时必须遵守：

1. 项目名统一为 **Nexis**；
2. 控制中心统一叫 **Nexis Console**；
3. 被测远端节点统一叫 **Nexis Node**；
4. 本地探针统一叫 **Nexis Probe**；
5. 当前只实现 TCP；
6. Probe 必须主动连接 Console；
7. Probe 连接断开或过期后必须主动重建连接；
8. 去程为 Probe -> Node 的直连 TCP；
9. 回程为 Node -> Console -> Probe 的逻辑 TCP；
10. 所有端口使用高位不常用端口，并且可配置；
11. UI 参考附件的浅色卡片式控制台；
12. 先实现可运行 MVP，再做复杂优化。

