# TCP 双向监控系统（VPS Server / Remote Agent / Local Agent）Vibe Coding 开发文档

> 版本：MVP v1.0  
> 面向对象：AI 编码助手（Codex / Claude Code / Cursor / Gemini CLI 等）+ 人类开发者  
> 文档目标：直接作为 AI 开发任务说明书使用，帮助快速生成可运行的前后端与 Agent 原型。  
> UI 风格参考：用户附件中的控制台截图（浅色、卡片化、圆角、低饱和、信息密度适中、折线图为主）。

---

## 1. 项目背景与目标

我要做一个 **基于 TCP 的双向网络质量监控系统**，核心场景如下：

- **VPS Server**：部署前端、后端 API、控制通道、数据库，是整个系统的控制中心。
- **Remote Agent**：被检测设备，通常具备公网 IP。
- **Local Agent**：本地探针设备，可能 **没有公网 IPv4**，负责检测自己到各个 Remote Agent 的 TCP 去程 / 回程质量。

系统需要实现：

1. **Local Agent 主动连接 VPS Server，并保持长连接常驻**。
2. 即便 Local Agent 没有公网 IPv4，也要能在系统内体现“从 Remote Agent 到 Local Agent 的逻辑回程检测”。
3. 当前 **只做 TCP**，不做 UDP / ICMP。
4. UI 风格参考附件：
   - 点击某个 **Remote Agent** 进入详情页；
   - 顶部展示该 Remote Agent 的基础信息：CPU、内存、磁盘、架构、在线状态、流量等；
   - 下方展示这个 Remote Agent 与多个 Local Agent 的连接质量；
   - Local Agent 在界面中会以“广东电信IPv4 / 广东移动IPv6 / 广东联通IPv4”等标签卡形式出现。
5. 所有监听端口与服务端口尽量使用 **高位且不常用端口**。
6. 如果常驻长连接失效、过期、半开、异常断开，Agent 必须能 **自动重建新连接**。

---

## 2. 概念定义（必须严格使用）

### 2.1 VPS Server

VPS Server 是中心节点，负责：

- 托管 Web 前端；
- 暴露后端 API；
- 接收 Local Agent / Remote Agent 的控制连接；
- 下发测试任务；
- 接收测试结果；
- 存储时序指标与节点状态；
- 提供图表查询与告警扩展能力；
- 为无公网 IPv4 的 Local Agent 提供“反向逻辑可达能力”（基于已建立的长连接 / 反向隧道）。

### 2.2 Remote Agent

Remote Agent 是 **被检测设备**，特点：

- 通常具备公网 IP；
- 可被 Local Agent 主动发起 TCP 探测；
- 也要主动连接 VPS Server，作为受管节点上线；
- 对外开启一个或多个 **高位测试端口**，用于被动接受 TCP 测试；
- 向 VPS Server 定期上报系统资源信息（CPU、内存、磁盘、网络流量等）。

### 2.3 Local Agent

Local Agent 是 **本地探针设备**，特点：

- 可能没有公网 IPv4；
- 主动连接 VPS Server，保持长连接常驻；
- 执行“到各个 Remote Agent 的 TCP 去程检测”；
- 配合 VPS Server/Remote Agent 完成“回程逻辑检测”；
- 在 UI 中会以运营商/地域/协议族维度命名，例如：
  - 广东电信IPv4
  - 广东移动IPv6
  - 广东联通IPv4

---

## 3. 现实约束（很重要）

### 3.1 关于“真实回程检测”的边界

当 **Local Agent 没有公网 IPv4** 时，Remote Agent **无法直接主动新建 TCP 连接到 Local Agent**。这是 NAT / CGNAT 的天然限制。

因此，本项目的 **MVP 定义**如下：

#### A. 真实直连去程检测（必须实现）

- `Local Agent -> Remote Agent` 直接发起 TCP 测试。
- 这是 **真实路径**。

#### B. 逻辑回程检测（MVP 必须实现）

- `Remote Agent -> VPS Server -> Local Agent`，通过 Local Agent 已建立的长连接 / 反向隧道实现。
- 这是 **逻辑上的回程测试**，不是严格意义上的纯直连 Remote->Local。
- 它的价值是：
  - 在 Local Agent 无公网 IPv4 的前提下，依然能建立“反向主动探测”的产品能力；
  - 能体现从 Remote 视角访问 Local 的成功率、延迟、抖动、连接稳定性。

#### C. 未来增强模式（文档中预留）

如果满足下列任一条件，可升级为 **真实直连回程检测**：

- Local Agent 有公网 IPv4；
- Local Agent 有可直达公网 IPv6；
- 用户手工做了端口映射；
- 后续引入更复杂的 TCP hole punching / 中继优化机制。

MVP 先不做复杂 NAT 穿透；先把控制面、指标面、UI 和长连接可靠性做好。

---

## 4. MVP 功能范围

### 4.1 必做功能

1. **VPS Server**
   - Agent 注册 / 鉴权 / 心跳
   - 节点管理 API
   - 测试任务调度
   - 指标接收与存储
   - 图表查询 API
   - 反向逻辑通道管理

2. **Remote Agent**
   - 主动连接 VPS Server
   - 上报系统信息
   - 开启 TCP 测试监听端口
   - 响应 Local Agent 的直连探测
   - 响应 Server 下发的逻辑回程任务

3. **Local Agent**
   - 主动连接 VPS Server
   - 常驻控制连接
   - 断线重连、会话续租
   - 对 Remote Agent 发起直连 TCP 检测
   - 配合完成反向逻辑回程检测

4. **前端控制台**
   - Remote Agent 列表页
   - Remote Agent 详情页
   - 基础硬件卡片
   - CPU / 内存 / 网络流量 / 磁盘图表
   - 各 Local Agent 到该 Remote Agent 的连接质量面板
   - 时间范围选择：`1小时 / 6小时 / 24小时 / 7天`

### 4.2 暂不做（MVP 不做）

- UDP 监控
- ICMP / ping
- 告警通知（可预留）
- 多租户复杂 RBAC
- 地图可视化
- 复杂 NAT 穿透
- 自动发现节点
- eBPF 深度抓包分析

---

## 5. 推荐技术栈

为了适合 AI 快速编码、部署和维护，建议如下：

### 5.1 Monorepo 结构

- `apps/web`：前端
- `apps/server`：后端 API + 控制面
- `agents/local-agent`：Local Agent
- `agents/remote-agent`：Remote Agent
- `packages/proto`：共享协议定义
- `deploy/`：Docker Compose / systemd / SQL

### 5.2 前端

- **Next.js 15**
- **React + TypeScript**
- **Tailwind CSS**
- **shadcn/ui**
- **ECharts**（更适合仪表盘）
- `react-query` / `tanstack query`

### 5.3 后端

- **Go 1.24+**
- Web 框架：**Fiber** 或 **Gin**（任选其一，推荐 Fiber）
- WebSocket：原生或 `gorilla/websocket`
- 数据访问：`sqlc` 或 `gorm`（推荐 `sqlc` 更清晰）
- 定时任务：内建 scheduler

### 5.4 数据库

- **PostgreSQL 16+**
- 推荐扩展：**TimescaleDB**（存储时序数据更舒服）

### 5.5 部署

- Docker Compose（MVP）
- 未来可扩展到 k8s
- Agent 使用 systemd 托管

---

## 6. 端口规划（全部使用高位、不常用端口）

下面给出默认端口，要求全部可配置。

| 模块 | 默认端口 | 说明 |
|---|---:|---|
| Web 前端 | 47131 | 前端页面访问 |
| Server API | 47141 | REST API |
| Server Agent Control（WSS/TCP） | 47151 | Local/Remote Agent 的控制通道 |
| Remote Agent TCP Test Listener | 47211 | 供 Local Agent 发起 TCP 探测 |
| Local Reverse Tunnel Base | 47300-47999 | VPS 为 Local Agent 动态分配的回程逻辑测试端口区间 |
| Metrics/Debug（可选） | 47191 | 内部调试 |

### 6.1 端口要求

- 所有端口必须可在配置文件中修改；
- 默认不要使用 80 / 443 / 8080 / 22 / 3306 等常见端口；
- Local Reverse Tunnel 使用动态高位端口池；
- 每个 Local Agent 可被分配一个或多个隧道端口。

---

## 7. 系统总体架构

```mermaid
flowchart LR
    subgraph User
        U[Web Console]
    end

    subgraph VPS["VPS Server"]
        WEB[Frontend]
        API[API / Scheduler]
        CTRL[Control Hub]
        DB[(PostgreSQL / TimescaleDB)]
        TUN[Reverse Tunnel Manager]
    end

    subgraph LA["Local Agent(s)"]
        L1[广东电信IPv4]
        L2[广东移动IPv6]
        L3[广东联通IPv4]
    end

    subgraph RA["Remote Agent(s)"]
        R1[Oracle]
        R2[Tokyo]
        R3[LA Node]
    end

    U --> WEB
    WEB --> API
    API --> DB
    API --> CTRL
    CTRL --> TUN

    L1 -. control ws/tcp .-> CTRL
    L2 -. control ws/tcp .-> CTRL
    L3 -. control ws/tcp .-> CTRL

    R1 -. control ws/tcp .-> CTRL
    R2 -. control ws/tcp .-> CTRL
    R3 -. control ws/tcp .-> CTRL

    L1 -->|TCP direct test| R1
    L1 -->|TCP direct test| R2
    L1 -->|TCP direct test| R3

    R1 -->|logical reverse test via VPS| TUN
    TUN -->|existing long connection| L1
```

---

## 8. 核心设计思路

## 8.1 双通道模型

系统分为：

### A. 控制通道（必须常驻）

- Agent 与 VPS Server 之间的长连接；
- 用于：
  - 注册
  - 鉴权
  - 心跳
  - 任务下发
  - 结果回传
  - 隧道复用
- 推荐使用：
  - `WSS`（WebSocket over TLS）或
  - 自定义 `TLS over TCP`

MVP 推荐：**WSS**，因为调试与 AI 编码速度更快。

### B. 测试通道（按任务触发）

分两种：

1. **直连去程测试通道**：Local Agent 主动连 Remote Agent 的测试端口。  
2. **逻辑回程测试通道**：Remote Agent 主动连 VPS 暴露给该 Local Agent 的反向隧道端口，再经 VPS 转发给 Local Agent。

---

## 8.2 长连接可靠性策略（必须实现）

Local Agent / Remote Agent 与 VPS Server 的控制连接必须具备以下能力：

### 心跳策略

- 心跳间隔：`15s`
- 心跳超时：`45s`
- 会话租期（lease TTL）：`90s`

### 重连策略

- 连接断开立即进入重连；
- 指数退避：`1s -> 2s -> 4s -> 8s -> 15s -> 30s`；
- 增加随机抖动：`0~20%`；
- 成功重连后清零退避状态；
- 新连接建立成功后，旧 session 必须被标记为 `superseded`；
- 服务端只保留最后一个活跃 session 作为当前有效连接。

### 半开检测

- 如果底层 socket 未显式报错，但连续多个心跳收不到 ack，则主动断开并重连；
- 本地写队列持续阻塞超过阈值时，也要触发连接重建；
- 所有连接需启用 TCP keepalive（系统层）+ 应用层 heartbeat（双保险）。

### 热替换要求

- 新连接建立后，旧连接的任务必须能平滑迁移；
- 对于正在执行的长任务，允许服务端将任务重新派发；
- Agent 端需支持幂等任务处理。

---

## 9. 监控指标定义

> 因为当前只做 TCP，所以“丢包”不能像 ICMP 那样直接定义为 IP 层分组丢包率。MVP 中建议从 **连接失败率 / 超时率 / 应用层探测失败率 / TCP 重传统计** 四个角度表达 TCP 质量。

### 9.1 去程检测指标（Local -> Remote，真实直连）

由 Local Agent 主动对 Remote Agent 执行：

1. `tcp_connect_latency_ms`
   - 从 `connect()` 开始到三次握手成功耗时。

2. `tcp_tls_handshake_ms`（如果测试端口上启用了 TLS，可选）
   - TLS 握手耗时。

3. `app_rtt_ms`
   - 建立连接后发送一段很小的 probe payload，Remote 立即回 echo，测量应用层 RTT。

4. `jitter_ms`
   - 对连续 N 次 `app_rtt_ms` 计算抖动，可使用：
     - 简单标准差；或
     - 相邻样本差值的平均绝对值。

5. `connect_success_rate`
   - N 次连接尝试中成功占比。

6. `timeout_rate`
   - N 次连接或读写中超时占比。

7. `app_probe_loss_rate`
   - 应用层探测请求成功发送但未在超时窗口收到回包的占比。

8. `tcp_retransmissions`
   - Linux 下可从 `TCP_INFO` 读取重传统计（若可用）。

### 9.2 逻辑回程检测指标（Remote -> Local，经 VPS 隧道）

由 Remote Agent 主动对 VPS 暴露的 Local 反向隧道端口执行：

1. `reverse_tcp_connect_latency_ms`
2. `reverse_app_rtt_ms`
3. `reverse_jitter_ms`
4. `reverse_connect_success_rate`
5. `reverse_timeout_rate`
6. `reverse_app_probe_loss_rate`
7. `reverse_tcp_retransmissions`（若能采到）

### 9.3 节点资源指标（用于详情页顶部）

Remote Agent 需要周期上报：

- `cpu_usage_percent`
- `memory_total_mb`
- `memory_used_mb`
- `disk_total_gb`
- `disk_used_gb`
- `network_rx_bytes`
- `network_tx_bytes`
- `uptime_seconds`
- `os`
- `arch`
- `virtualization`
- `public_ipv4`
- `public_ipv6`

---

## 10. TCP 检测协议设计

## 10.1 Remote Agent 测试监听器

Remote Agent 在 `47211`（默认）监听一个 TCP Echo 服务，用于接受 Local Agent 的探测。

### 建议协议

- 协议：纯 TCP 自定义帧协议
- 数据帧很简单，JSON 行协议或长度前缀二进制皆可
- MVP 推荐：**JSON 行协议**，AI 更容易生成

#### 请求示例

```json
{"type":"probe","task_id":"task_123","ts":1711111111111,"seq":1,"payload":"ping"}
```

#### 响应示例

```json
{"type":"probe_ack","task_id":"task_123","ts":1711111111119,"seq":1,"payload":"pong"}
```

### 测试流程

1. Local Agent 建立 TCP 连接；
2. 发送 5~10 次小包 probe；
3. Remote Agent 原样快速回 echo；
4. Local Agent 记录 connect、首字节、应用层 RTT 等指标；
5. 将聚合结果上报 VPS Server。

---

## 10.2 Local Agent 逻辑回程接入器

由于 Local Agent 可能没有公网 IPv4，不能直接被 Remote Agent 从公网打进来，所以采用：

- Local Agent 与 VPS 保持 1 条常驻控制连接；
- 可额外复用该连接或建立 1 条数据隧道连接；
- VPS 为每个 Local Agent 分配 1 个高位端口，例如 `47321`；
- Remote Agent 对 `VPS_IP:47321` 发起 TCP 测试；
- VPS 将该连接桥接到 Local Agent；
- Local Agent 收到 probe 后快速回 echo；
- Remote Agent 记录逻辑回程指标并上报。

### 关键说明

- 这条路径是 `Remote -> VPS -> Local`；
- 它在产品上代表“Remote 侧主动检测 Local”；
- 不是严格意义上的纯直连互联网回程；
- UI 中建议命名为：
  - **回程（逻辑）** 或
  - **Reverse (via tunnel)**
- 如果未来 Local 可直连，则可自动切换为 **回程（直连）**。

---

## 11. 调度模型

## 11.1 调度粒度

MVP 可采用固定调度周期：

- 资源上报：每 `15s`
- 去程测试：每 `30s`
- 逻辑回程测试：每 `30s`
- 图表聚合：
  - 原始样本保留 7 天
  - 1 分钟聚合保留 30 天

## 11.2 调度策略

Server 端维护一张 `test_plan`：

- 每个 Remote Agent 对应多个 Local Agent；
- 每对 `(local_agent_id, remote_agent_id)` 需要两类测试：
  - `forward_direct_tcp`
  - `reverse_logical_tcp`

### 示例

- 广东电信IPv4 -> Oracle：去程直连
- Oracle -> 广东电信IPv4（经 VPS 隧道）：回程逻辑
- 广东移动IPv6 -> Oracle：去程直连
- Oracle -> 广东移动IPv6（经 VPS 隧道）：回程逻辑

---

## 12. 数据模型设计

以下为推荐表结构（可简化）。

## 12.1 agents

```sql
CREATE TABLE agents (
  id UUID PRIMARY KEY,
  agent_type TEXT NOT NULL CHECK (agent_type IN ('local', 'remote')),
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

## 12.2 agent_sessions

```sql
CREATE TABLE agent_sessions (
  id UUID PRIMARY KEY,
  agent_id UUID NOT NULL REFERENCES agents(id),
  session_kind TEXT NOT NULL,
  connected_at TIMESTAMPTZ NOT NULL,
  last_heartbeat_at TIMESTAMPTZ,
  disconnected_at TIMESTAMPTZ,
  lease_expires_at TIMESTAMPTZ,
  is_active BOOLEAN NOT NULL DEFAULT true,
  close_reason TEXT,
  remote_addr TEXT,
  metadata JSONB DEFAULT '{}'::jsonb
);
```

## 12.3 remote_system_snapshots

```sql
CREATE TABLE remote_system_snapshots (
  id BIGSERIAL PRIMARY KEY,
  agent_id UUID NOT NULL REFERENCES agents(id),
  ts TIMESTAMPTZ NOT NULL,
  cpu_usage_percent DOUBLE PRECISION,
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

## 12.4 test_jobs

```sql
CREATE TABLE test_jobs (
  id UUID PRIMARY KEY,
  test_type TEXT NOT NULL,
  local_agent_id UUID REFERENCES agents(id),
  remote_agent_id UUID REFERENCES agents(id),
  direction TEXT NOT NULL,
  status TEXT NOT NULL,
  scheduled_at TIMESTAMPTZ NOT NULL,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  error_message TEXT,
  metadata JSONB DEFAULT '{}'::jsonb
);
```

## 12.5 tcp_test_results

```sql
CREATE TABLE tcp_test_results (
  id BIGSERIAL PRIMARY KEY,
  job_id UUID REFERENCES test_jobs(id),
  ts TIMESTAMPTZ NOT NULL,
  local_agent_id UUID REFERENCES agents(id),
  remote_agent_id UUID REFERENCES agents(id),
  test_type TEXT NOT NULL,
  direction TEXT NOT NULL,
  path_mode TEXT NOT NULL,
  connect_latency_ms DOUBLE PRECISION,
  app_rtt_ms DOUBLE PRECISION,
  jitter_ms DOUBLE PRECISION,
  success_rate DOUBLE PRECISION,
  timeout_rate DOUBLE PRECISION,
  probe_loss_rate DOUBLE PRECISION,
  retransmissions INTEGER,
  sample_count INTEGER,
  raw JSONB DEFAULT '{}'::jsonb
);
```

### direction 与 path_mode 约定

- `direction`：
  - `forward`
  - `reverse`
- `path_mode`：
  - `direct`
  - `tunnel`

---

## 13. Server 与 Agent 协议

建议控制通道使用 WebSocket，消息统一 JSON。

## 13.1 Agent 注册

```json
{
  "type": "register",
  "agent_id": "uuid",
  "agent_type": "local",
  "name": "广东电信IPv4",
  "token": "plain-token",
  "version": "0.1.0",
  "capabilities": ["tcp-forward-probe", "reverse-tunnel-client"],
  "meta": {
    "os": "linux",
    "arch": "amd64"
  }
}
```

## 13.2 注册响应

```json
{
  "type": "register_ack",
  "session_id": "uuid",
  "heartbeat_interval_sec": 15,
  "lease_ttl_sec": 90,
  "server_time": 1711111111111,
  "reverse_tunnel": {
    "enabled": true,
    "listen_port": 47321
  }
}
```

## 13.3 心跳

```json
{
  "type": "heartbeat",
  "session_id": "uuid",
  "ts": 1711111111111,
  "stats": {
    "cpu": 12.3,
    "mem_used_mb": 1024
  }
}
```

## 13.4 下发测试任务

```json
{
  "type": "run_test",
  "job_id": "uuid",
  "test_type": "forward_direct_tcp",
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

## 13.5 上报测试结果

```json
{
  "type": "test_result",
  "job_id": "uuid",
  "test_type": "forward_direct_tcp",
  "direction": "forward",
  "path_mode": "direct",
  "result": {
    "connect_latency_ms": 82,
    "app_rtt_ms": 96,
    "jitter_ms": 4.3,
    "success_rate": 1,
    "timeout_rate": 0,
    "probe_loss_rate": 0,
    "retransmissions": 0,
    "sample_count": 5
  },
  "raw": {
    "samples": [95, 96, 98, 92, 99]
  }
}
```

---

## 14. Local Agent 设计

## 14.1 核心职责

- 与 Server 建立常驻控制连接；
- 处理 register / heartbeat / task / ack；
- 对 Remote Agent 执行去程 TCP 直连探测；
- 提供一个反向逻辑服务端接口（通过隧道承载），响应 Remote Agent 的 probe；
- 断线后自动重连；
- 连接恢复后恢复接收任务。

## 14.2 模块划分

- `config`
- `logger`
- `control_client`
- `heartbeat_manager`
- `task_worker`
- `tcp_forward_probe`
- `reverse_probe_handler`
- `reconnect_manager`
- `metrics_reporter`

## 14.3 关键实现要求

- 所有测试任务必须设置超时；
- 一个 Local Agent 可以并发测试多个 Remote Agent，但要有并发上限；
- 每次测试建议发 5 个小 probe 包；
- 探测超时与失败要明确上报；
- 在 Linux 上优先读取 `TCP_INFO`；
- 使用 monotonic time 计算耗时，避免系统时钟跳变影响。

---

## 15. Remote Agent 设计

## 15.1 核心职责

- 与 Server 建立常驻控制连接；
- 监听测试端口 `47211`；
- 响应 Local Agent 发来的 probe；
- 执行回程逻辑检测（对 VPS 暴露的隧道端口进行 probe）；
- 上报本机系统资源；
- 支持断线重连。

## 15.2 模块划分

- `config`
- `control_client`
- `resource_collector`
- `tcp_test_listener`
- `reverse_probe_runner`
- `reconnect_manager`

## 15.3 资源采集建议

- CPU：最近采样窗口占用率
- 内存：总量 / 已用
- 磁盘：挂载点合计
- 流量：总收发字节
- 架构：`linux/amd64`、`linux/arm64`
- 虚拟化：尽量识别 `kvm` / `guest` / `container`

---

## 16. 反向逻辑隧道设计（MVP）

## 16.1 目标

在 Local Agent 无公网 IPv4 的情况下，仍然允许 Remote Agent“主动发起一次到 Local 的 TCP 探测”。

## 16.2 简化方案（推荐）

### 方案描述

1. Local Agent 与 Server 建立控制连接；
2. Server 为 Local Agent 分配一个 `reverse_port`；
3. Remote Agent 连接到 `VPS:reverse_port`；
4. Server 将收到的字节流桥接到 Local Agent；
5. Local Agent 将自己表现成一个 echo responder；
6. Remote Agent 计算 RTT、抖动、超时、失败率。

### 设计要求

- 每个 Local Agent 至少一个专属隧道端口；
- Server 需维护 `reverse_port -> local_agent_session` 映射；
- 若 Local Agent session 失效，该端口应标记为不可用；
- 当 Local Agent 重连后，如果 session 更新，需要自动重新绑定。

### 桥接行为

- 一条外部 TCP 连接对应一个内部逻辑 stream；
- 建议使用 stream id 多路复用，避免每个连接都重新拨控制连接；
- MVP 也可以先“一条控制连接 + 多条短数据连接”的简单实现。

---

## 17. API 设计（最小可用集）

## 17.1 节点列表

`GET /api/v1/agents?type=remote`

返回所有 Remote Agent 基本信息。

## 17.2 节点详情

`GET /api/v1/agents/:id`

返回：

- 节点名称
- 在线状态
- 公网 IP
- OS / 架构
- CPU / 内存 / 磁盘 / 流量
- uptime
- 最近连接时间

## 17.3 节点资源图表

`GET /api/v1/agents/:id/resource-series?range=24h&metric=cpu`

支持：

- `cpu`
- `memory`
- `network`
- `disk`

## 17.4 节点连接质量

`GET /api/v1/agents/:id/connectivity?range=24h`

返回某个 Remote Agent 与所有 Local Agent 的测试结果列表：

- Local Agent 名称
- 去程状态
- 去程 RTT / 抖动 / 成功率
- 回程（逻辑）状态
- 回程 RTT / 抖动 / 成功率
- 最近测试时间

## 17.5 某 Local Agent 的时序数据

`GET /api/v1/agents/:id/connectivity/:localAgentId/series?range=24h&direction=forward`

---

## 18. UI 设计说明（参考附件）

## 18.1 风格要求

整体界面风格参考附件：

- 浅色背景
- 大面积留白
- 卡片化布局
- 圆角 `16px ~ 20px`
- 轻阴影 / 细边框
- 图表区域简洁
- 标签 chip 风格轻量
- 字重以 `500 / 600 / 700` 为主
- 不要做花哨炫技，风格尽量克制、清爽、专业

## 18.2 页面结构

### 页面 A：Remote Agent 列表页

- 搜索框
- 状态筛选（全部 / 在线 / 离线）
- Agent 卡片列表
  - 名称
  - 公网 IP
  - OS / Arch
  - 在线时长
  - 状态 badge

### 页面 B：Remote Agent 详情页（核心页）

布局参考附件：

#### 顶部导航

- 返回仪表盘按钮
- 语言切换占位（可选）
- 刷新按钮（可选）
- 主题按钮（可选）

#### 区块 1：节点基础卡片

- 标题：Remote Agent 名称，例如 `Oracle`
- 副标题：节点基础身份和连接信息
- 展示：
  - 公网 IP
  - OS / 架构
  - 在线时长
  - 在线状态 badge

#### 区块 2：硬件 / 系统摘要卡片

五个小卡片：

- 核心 / 线程
- 内存
- 磁盘
- 虚拟化
- 平台

#### 区块 3：时间范围切换条

- `1小时`
- `6小时`
- `24小时`
- `7天`

#### 区块 4：资源图表 Tab

Tab：

- CPU 使用率
- 内存使用率
- 网络流量
- 磁盘使用率

图表为折线图。

#### 区块 5：连接质量面板（重点）

顶部以多个 Local Agent chip 展示：

- 广东电信IPv4
- 广东电信IPv6
- 广东移动IPv4
- 广东移动IPv6
- 广东联通IPv4
- 广东联通IPv6

每个 chip 中可显示：

- 当前去程状态摘要
- 当前回程状态摘要
- 最近均值 RTT

下面图表区展示选中 Local Agent 对该 Remote Agent 的历史连接质量：

可切换指标：

- 去程 RTT
- 去程抖动
- 去程失败率
- 回程 RTT
- 回程抖动
- 回程失败率

### 18.3 UI 文案建议

- `在线`
- `离线`
- `系统概览`
- `时间范围`
- `CPU 使用率`
- `内存使用率`
- `网络流量`
- `磁盘使用率`
- `连接监测`
- `去程（直连）`
- `回程（逻辑）`

---

## 19. 前端组件建议

- `AgentHeaderCard`
- `AgentSummaryStats`
- `TimeRangeSwitch`
- `ResourceTabs`
- `MetricLineChart`
- `LocalAgentChipGroup`
- `ConnectivityOverviewCard`
- `ConnectivitySeriesChart`
- `StatusBadge`

---

## 20. 告警与状态判定（先做基础版）

MVP 可定义一个简化健康度规则：

### 单次测试结果评级

- `healthy`
  - success_rate >= 0.95
  - app_rtt_ms < 150
  - timeout_rate < 0.05

- `degraded`
  - success_rate >= 0.8
  - 或 jitter_ms 偏高
  - 或 app_rtt_ms >= 150 且 < 400

- `critical`
  - success_rate < 0.8
  - 或 timeout_rate >= 0.2
  - 或连接基本失败

### 节点在线状态

- 45 秒内有 heartbeat：`online`
- 45~90 秒：`stale`
- 超过 90 秒：`offline`

---

## 21. 安全设计

## 21.1 Agent 鉴权

- 每个 Agent 有独立 token
- token 只存 hash
- register 时提交明文 token 进行校验
- 支持 token 轮换

## 21.2 传输安全

- 控制通道必须使用 TLS
- 若使用 WSS，则必须使用 `wss://`
- 若用纯 TCP，自定义协议也必须包一层 TLS

## 21.3 权限范围

- Local Agent 只能执行授权范围内的测试
- Remote Agent 只能访问被 Server 分配的反向隧道资源
- 前端 API 做基础 session 鉴权

---

## 22. 开发里程碑（适合 AI 分步实现）

## Milestone 1：Server 基础框架

目标：

- 启动 API 服务
- 启动 WebSocket 控制服务
- PostgreSQL 表初始化
- Agent register / heartbeat 跑通

验收：

- Local Agent / Remote Agent 都能上线
- 前端能显示在线列表

## Milestone 2：Remote Agent 资源上报

目标：

- Remote Agent 采集 CPU / 内存 / 磁盘 / 流量
- Server 接收并存库
- 前端图表展示资源时序

验收：

- Remote 节点详情页顶部与折线图正常显示

## Milestone 3：去程直连 TCP 测试

目标：

- Remote Agent 开启测试监听器
- Local Agent 可执行 `forward_direct_tcp`
- Server 能存储结果
- 前端显示某个 Local Agent 的去程图表

验收：

- Local 到 Remote 的 connect / RTT / jitter 曲线能正常展示

## Milestone 4：回程逻辑隧道

目标：

- Server 为 Local Agent 分配 reverse port
- Remote Agent 可对 reverse port 发起 probe
- Local Agent 能响应 echo
- Server 存储 `reverse_logical_tcp` 结果

验收：

- 前端可展示“回程（逻辑）”曲线

## Milestone 5：连接健壮性

目标：

- 心跳过期处理
- 自动断线重连
- session 热替换
- 数据写入幂等

验收：

- 手动断网 / 杀进程 / 重启 Agent 后能恢复上线

## Milestone 6：UI 打磨

目标：

- 页面布局贴近参考图
- 节点详情页信息更完整
- 图表与 chip 交互顺畅

---

## 23. AI 编码注意事项（强约束）

1. 先做 **可运行 MVP**，不要一开始就过度工程化；
2. 优先保证：
   - 协议清晰
   - 重连可靠
   - 指标可存可查
   - UI 可用
3. 不要一开始上太多中间件；
4. 所有代码需有明确目录结构；
5. 后端与 Agent 代码尽量 Go，前端 TypeScript；
6. 所有端口都放配置；
7. 所有测试任务必须有 timeout；
8. 时间计算使用 monotonic clock；
9. 数据模型与 API 字段命名要统一；
10. 图表接口要直接为前端提供友好格式。

---

## 24. AI 可直接使用的任务提示词（建议复制给 AI 编码助手）

### Prompt A：初始化项目骨架

```text
请基于以下要求创建一个 monorepo 项目骨架：
- apps/web: Next.js + TypeScript + Tailwind + shadcn/ui + ECharts
- apps/server: Go + Fiber + WebSocket + PostgreSQL
- agents/local-agent: Go
- agents/remote-agent: Go
- packages/proto: 共享 JSON 协议定义文档
- deploy: Docker Compose、SQL 初始化脚本、systemd 示例
要求：
- 所有模块都能本地启动
- 提供 README
- 提供基础配置文件示例
- 所有默认端口使用高位不常用端口，例如 47131/47141/47151/47211
```

### Prompt B：实现 Server 控制面

```text
请实现 Go 版 Server 控制面：
- 提供 REST API
- 提供 WebSocket 控制通道
- 支持 agent register / heartbeat / reconnect / session replace
- PostgreSQL 存储 agents 和 agent_sessions
- 心跳间隔 15 秒，lease TTL 90 秒
- 如果 session 过期或断开，标记离线
- 为 Local Agent 分配 reverse tunnel port
要求代码可运行，并附带数据库迁移脚本。
```

### Prompt C：实现 Local Agent

```text
请实现 Go 版 Local Agent：
- 启动后主动连接 VPS Server 的 WebSocket 控制端口
- 支持 register / heartbeat / reconnect
- 支持 forward_direct_tcp 测试任务：对 Remote Agent 的 TCP 测试端口发起连接，测量 connect_latency_ms、app_rtt_ms、jitter_ms、timeout_rate、probe_loss_rate
- 支持作为 reverse logical probe responder，通过与 Server 的连接响应 probe
- 必须实现自动重连和心跳超时断开重建
- 所有配置走 yaml 或 toml
```

### Prompt D：实现 Remote Agent

```text
请实现 Go 版 Remote Agent：
- 主动连接 VPS Server 控制通道
- 上报 CPU、内存、磁盘、流量、uptime、os、arch、virtualization
- 在高位端口（默认 47211）开启 TCP probe listener，收到 JSON 行 probe 后立即返回 probe_ack
- 支持执行 reverse_logical_tcp 测试：连接 VPS 为某 Local Agent 暴露的 reverse port，测量 connect_latency_ms、app_rtt_ms、jitter_ms、timeout_rate、probe_loss_rate
- 结果通过控制通道回传给 Server
```

### Prompt E：实现前端详情页

```text
请实现一个 Next.js 仪表盘页面，风格参考浅色卡片式控制台：
- 页面为 Remote Agent 详情页
- 顶部显示节点名、IP、架构、在线状态、在线时长
- 中间显示五个系统概览卡片：核心/线程、内存、磁盘、虚拟化、平台
- 下面有时间范围切换：1小时、6小时、24小时、7天
- 资源图表 Tab：CPU 使用率、内存使用率、网络流量、磁盘使用率
- 连接监测区域：顶部显示多个 Local Agent chip（如广东电信IPv4、广东移动IPv6等），下方显示该 Local Agent 与当前 Remote Agent 的历史曲线
- 曲线支持：去程（直连）RTT、去程抖动、去程失败率、回程（逻辑）RTT、回程抖动、回程失败率
- 使用 Tailwind + shadcn/ui + ECharts
```

---

## 25. 配置文件示例

## 25.1 Server

```yaml
server:
  web_port: 47131
  api_port: 47141
  control_port: 47151
  reverse_port_start: 47300
  reverse_port_end: 47999
  tls_cert: ./certs/server.crt
  tls_key: ./certs/server.key

database:
  dsn: postgres://monitor:monitor@127.0.0.1:5432/monitor?sslmode=disable

scheduler:
  resource_report_interval_sec: 15
  forward_probe_interval_sec: 30
  reverse_probe_interval_sec: 30

agent:
  heartbeat_interval_sec: 15
  lease_ttl_sec: 90
```

## 25.2 Local Agent

```yaml
agent:
  id: 8b0d9dd2-1111-2222-3333-444444444444
  name: 广东电信IPv4
  type: local
  token: replace-with-token

server:
  control_url: wss://your-vps.example.com:47151/ws

probe:
  connect_timeout_ms: 3000
  read_timeout_ms: 3000
  probe_count: 5
  concurrency: 4

reconnect:
  min_backoff_sec: 1
  max_backoff_sec: 30
  jitter_ratio: 0.2
```

## 25.3 Remote Agent

```yaml
agent:
  id: f54f17be-1111-2222-3333-555555555555
  name: Oracle
  type: remote
  token: replace-with-token

server:
  control_url: wss://your-vps.example.com:47151/ws

listener:
  host: 0.0.0.0
  port: 47211

resource:
  report_interval_sec: 15

probe:
  connect_timeout_ms: 3000
  read_timeout_ms: 3000
  probe_count: 5
```

---

## 26. 测试与验收清单

## 26.1 基础联通

- [ ] Server 可启动
- [ ] Web 可启动
- [ ] PostgreSQL 可连接
- [ ] Local Agent 可上线
- [ ] Remote Agent 可上线

## 26.2 去程测试

- [ ] Local Agent 能直连 Remote Agent 的 `47211`
- [ ] 能拿到 connect_latency_ms
- [ ] 能拿到 app_rtt_ms
- [ ] 能拿到 jitter_ms
- [ ] 测试失败时能上报 timeout / error

## 26.3 回程逻辑测试

- [ ] Local Agent 上线后，Server 分配 reverse port
- [ ] Remote Agent 能连接该 reverse port
- [ ] Local Agent 能通过隧道回 echo
- [ ] Remote Agent 能上报 reverse 指标

## 26.4 长连接恢复

- [ ] 拔网线 / 断网后自动重连
- [ ] 杀掉 Server 后恢复时 Agent 能重新连接
- [ ] 同一 Agent 多个旧 session 不会混乱
- [ ] 心跳过期会被标记为离线

## 26.5 UI

- [ ] Remote 列表页可用
- [ ] Remote 详情页可用
- [ ] 资源图表可用
- [ ] 各 Local Agent 连接质量图表可用
- [ ] 时间范围切换可用

---

## 27. 后续可扩展方向

1. 支持 UDP 检测
2. 支持 ICMP / ping
3. 支持 IPv4 / IPv6 单独视图
4. 支持真实直连回程（当 Local 具备公网可达能力）
5. 支持告警通知（Telegram / 邮件 / Webhook）
6. 支持多用户、多组织
7. 支持站点 / 地域 / 运营商标签
8. 支持链路健康评分
9. 支持更高级别的 tunnel multiplexer
10. 支持分布式调度

---

## 28. 总结（给 AI 的最终要求）

请严格围绕以下目标开发：

- 这是一个 **TCP 双向监控系统**；
- 架构核心是 **VPS Server + Remote Agent + Local Agent**；
- **Local Agent 可能没有公网 IPv4**，因此必须由它主动连接 VPS Server；
- 系统要实现：
  - **去程（直连）**：Local -> Remote
  - **回程（逻辑）**：Remote -> VPS -> Local
- 所有长连接都要支持：
  - 心跳
  - lease TTL
  - 自动重连
  - 新旧 session 替换
- 前端界面风格要接近附件的浅色卡片式控制台；
- 点击某个 Remote Agent 后，要能看到：
  - 该 Remote Agent 的基础参数信息
  - 资源图表
  - 它与所有 Local Agent 的连接质量
- 所有服务端口、监听端口都使用 **高位不常用端口**，并且可配置。

