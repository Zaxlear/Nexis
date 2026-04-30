const API_BASE = `${location.protocol}//${location.hostname || "127.0.0.1"}:47141/api/v1`;

const state = {
  nodes: [],
  currentNode: null,
  range: "1h",
  resourceMetric: "cpu",
  selectedProbe: null,
  connectivityMetric: "app_rtt",
  connectivityDirection: "forward",
  query: "",
};

const labels = {
  cpu: "CPU 使用率",
  memory: "内存使用率",
  network: "网络流量",
  disk: "磁盘使用率",
  app_rtt: "RTT",
  jitter: "抖动",
  probe_loss_rate: "失败率",
};

const ranges = [
  ["1h", "1小时"],
  ["6h", "6小时"],
  ["24h", "24小时"],
  ["7d", "7天"],
];

const resourceTabs = [
  ["cpu", "CPU 使用率"],
  ["memory", "内存使用率"],
  ["network", "网络流量"],
  ["disk", "磁盘使用率"],
];

const connectivityTabs = [
  ["forward:app_rtt", "去程 RTT"],
  ["forward:jitter", "去程抖动"],
  ["forward:probe_loss_rate", "去程失败率"],
  ["reverse:app_rtt", "回程逻辑 RTT"],
  ["reverse:jitter", "回程逻辑抖动"],
  ["reverse:probe_loss_rate", "回程逻辑失败率"],
];

document.getElementById("homeButton").addEventListener("click", () => {
  location.hash = "";
});
document.getElementById("refreshButton").addEventListener("click", () => route());
document.getElementById("themeButton").addEventListener("click", () => document.body.classList.toggle("compact"));
window.addEventListener("hashchange", route);
route();

async function route() {
  const nodeMatch = location.hash.match(/^#\/nodes\/(.+)$/);
  if (nodeMatch) {
    await renderNodeDetail(decodeURIComponent(nodeMatch[1]));
  } else {
    await renderNodeList();
  }
}

async function api(path) {
  const response = await fetch(`${API_BASE}${path}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`${response.status} ${response.statusText}`);
  }
  return response.json();
}

async function renderNodeList() {
  const app = document.getElementById("app");
  app.innerHTML = skeleton("加载 Nexis Node...");
  try {
    state.nodes = await api("/nodes");
    app.innerHTML = `
      <section class="toolbar">
        <button class="back-button" disabled>Nexis Node</button>
        <div class="spacer"></div>
        <input class="search" id="searchInput" placeholder="搜索 Node、IP 或平台" value="${escapeHTML(state.query)}" />
      </section>
      <section class="stack">
        <div class="card card--pad">
          <div class="section-head">
            <div>
              <div class="eyebrow">节点列表</div>
              <h2>Nexis Node</h2>
              <p>所有远端被测节点与最近 TCP 质量摘要。</p>
            </div>
          </div>
          <div class="node-grid" id="nodeGrid"></div>
        </div>
      </section>
    `;
    document.getElementById("searchInput").addEventListener("input", (event) => {
      state.query = event.target.value;
      renderNodeCards();
    });
    renderNodeCards();
  } catch (error) {
    app.innerHTML = errorView(error);
  }
}

function renderNodeCards() {
  const grid = document.getElementById("nodeGrid");
  const query = state.query.trim().toLowerCase();
  const nodes = state.nodes.filter((node) => {
    if (!query) return true;
    return [node.name, node.public_ipv4, platform(node)].join(" ").toLowerCase().includes(query);
  });
  if (!nodes.length) {
    grid.innerHTML = `<div class="empty">没有匹配的 Nexis Node</div>`;
    return;
  }
  grid.innerHTML = nodes
    .map((node) => {
      const summary = node.summary || {};
      return `
        <article class="card node-card" data-node="${node.id}">
          <div class="pill-row"><span class="${badgeClass(node.status)}">${statusText(node.status)}</span></div>
          <h3>${escapeHTML(node.name)}</h3>
          <div class="node-card__meta">
            <span>公网 IP：${escapeHTML(node.public_ipv4 || node.public_ipv6 || "-")}</span>
            <span>平台：${escapeHTML(platform(node))}</span>
            <span>最近心跳：${relativeTime(node.last_seen_at)}</span>
          </div>
          <div class="pill-row">
            <span class="pill">去程 ${formatMs(summary.forward_rtt_ms)}</span>
            <span class="pill">回程逻辑 ${formatMs(summary.reverse_rtt_ms)}</span>
          </div>
        </article>
      `;
    })
    .join("");
  grid.querySelectorAll("[data-node]").forEach((card) => {
    card.addEventListener("click", () => {
      location.hash = `#/nodes/${encodeURIComponent(card.dataset.node)}`;
    });
  });
}

async function renderNodeDetail(nodeID) {
  const app = document.getElementById("app");
  app.innerHTML = skeleton("加载 Node 详情...");
  try {
    const [node, resource, connectivity] = await Promise.all([
      api(`/nodes/${nodeID}`),
      api(`/nodes/${nodeID}/resource-series?range=${state.range}&metric=${state.resourceMetric}`),
      api(`/nodes/${nodeID}/connectivity?range=${state.range}`),
    ]);
    state.currentNode = node;
    if (!state.selectedProbe && connectivity.items?.length) {
      state.selectedProbe = connectivity.items[0].probe.id;
    }
    const selected = connectivity.items?.find((item) => item.probe.id === state.selectedProbe)
      || connectivity.items?.find((item) => item.forward || item.reverse)
      || connectivity.items?.[0];
    if (selected) state.selectedProbe = selected.probe.id;
    const series = selected
      ? await api(`/nodes/${nodeID}/probes/${selected.probe.id}/series?range=${state.range}&metric=${state.connectivityMetric}&direction=${state.connectivityDirection}`)
      : { points: [] };

    app.innerHTML = `
      <section class="toolbar">
        <button class="back-button" id="backToNodes">← 返回仪表盘</button>
      </section>
      <section class="stack">
        ${nodeHero(node)}
        ${systemOverview(node)}
        ${rangeSwitch()}
        ${resourceSection(resource)}
        ${connectivitySection(connectivity, series)}
      </section>
    `;
    bindDetailEvents(nodeID);
    drawChart("resourceChart", resource.points || [], chartOptions(state.resourceMetric));
    drawChart("connectivityChart", series.points || [], chartOptions(state.connectivityMetric));
  } catch (error) {
    app.innerHTML = errorView(error);
  }
}

function nodeHero(node) {
  return `
    <section class="card card--pad hero">
      <span class="${badgeClass(node.status)}">${statusText(node.status)}</span>
      <div class="hero__content">
        <div>
          <div class="eyebrow">节点</div>
          <h1 class="title">${escapeHTML(node.name)}</h1>
          <p class="subtitle">这个 Nexis Node 的基础身份和连接信息。</p>
        </div>
        <div class="pill-row">
          <span class="pill">公网 ${escapeHTML(node.public_ipv4 || node.public_ipv6 || "-")}</span>
          <span class="pill">${escapeHTML(platform(node))}</span>
          <span class="pill">在线时长 ${uptimeText(node)}</span>
        </div>
      </div>
    </section>
  `;
}

function systemOverview(node) {
  const stats = node.latest_stats || {};
  const meta = node.meta || {};
  const memoryGB = stats.memory_total_mb ? `${(stats.memory_total_mb / 1024).toFixed(1)} GB` : "-";
  return `
    <section class="card card--pad">
      <div class="section-head">
        <div>
          <div class="eyebrow">硬件</div>
          <h2>${escapeHTML(meta.hardware || "系统概览")}</h2>
          <p>这台机器的硬件信息概览。</p>
        </div>
      </div>
      <div class="overview-grid">
        ${statTile("核心 / 线程", `${stats.cpu_cores || "-"} / ${stats.cpu_cores || "-"}`)}
        ${statTile("内存", memoryGB)}
        ${statTile("磁盘", stats.disk_total_gb ? `${stats.disk_total_gb.toFixed(1)} GB` : "-")}
        ${statTile("虚拟化", stats.virtualization || meta.virtualization || "-")}
        ${statTile("平台", platform(node))}
      </div>
    </section>
  `;
}

function statTile(label, value) {
  return `<div class="stat-tile"><div class="stat-label">${label}</div><div class="stat-value">${escapeHTML(String(value))}</div></div>`;
}

function rangeSwitch() {
  return `
    <section class="card card--pad range-card">
      <div class="copy">
        <div class="eyebrow">时间范围</div>
        <div class="muted">统一控制系统负载和延迟图表的时间窗口。</div>
      </div>
      <div class="segments">
        ${ranges.map(([value, label]) => `<button class="segment ${state.range === value ? "is-active" : ""}" data-range="${value}">${label}</button>`).join("")}
      </div>
    </section>
  `;
}

function resourceSection(resource) {
  return `
    <section class="card card--pad">
      <div class="tabs">
        ${resourceTabs.map(([value, label]) => `<button class="tab ${state.resourceMetric === value ? "is-active" : ""}" data-resource="${value}">${label}</button>`).join("")}
      </div>
    </section>
    <section class="card chart-card">
      <div class="chart-title">${escapeHTML(labels[state.resourceMetric] || resource.metric)}</div>
      <div id="resourceChart" class="chart"></div>
    </section>
  `;
}

function connectivitySection(connectivity, series) {
  const items = connectivity.items || [];
  return `
    <section class="card card--pad">
      <div class="section-head">
        <div>
          <div class="eyebrow">连接监测</div>
          <h2>当前节点与所有 Nexis Probe 的 TCP 连接质量。</h2>
        </div>
      </div>
      <div class="probe-chips">
        ${items.map(probeChip).join("") || `<div class="empty">暂无 Nexis Probe 数据</div>`}
      </div>
      <div class="connectivity-controls">
        <div class="tabs">
          ${connectivityTabs.map(([key, label]) => {
            const [direction, metric] = key.split(":");
            const active = state.connectivityDirection === direction && state.connectivityMetric === metric;
            return `<button class="tab ${active ? "is-active" : ""}" data-connectivity="${key}">${label}</button>`;
          }).join("")}
        </div>
      </div>
      <div class="chart-title">${escapeHTML(seriesTitle(series))}</div>
      <div id="connectivityChart" class="chart"></div>
    </section>
  `;
}

function probeChip(item) {
  const probe = item.probe;
  const forwardLoss = item.forward ? Math.round(item.forward.probe_loss_rate * 100) : "-";
  const reverseLoss = item.reverse ? Math.round(item.reverse.probe_loss_rate * 100) : "-";
  const latency = item.forward?.app_rtt_ms || item.reverse?.app_rtt_ms || 0;
  return `
    <button class="probe-chip ${state.selectedProbe === probe.id ? "is-active" : ""}" data-probe="${probe.id}">
      <strong>${escapeHTML(probe.name)}</strong>
      <span>去程 ${forwardLoss}% 回程 ${reverseLoss}% 延迟 ${formatMs(latency)}</span>
    </button>
  `;
}

function bindDetailEvents(nodeID) {
  document.getElementById("backToNodes").addEventListener("click", () => {
    location.hash = "";
  });
  document.querySelectorAll("[data-range]").forEach((button) => {
    button.addEventListener("click", () => {
      state.range = button.dataset.range;
      renderNodeDetail(nodeID);
    });
  });
  document.querySelectorAll("[data-resource]").forEach((button) => {
    button.addEventListener("click", () => {
      state.resourceMetric = button.dataset.resource;
      renderNodeDetail(nodeID);
    });
  });
  document.querySelectorAll("[data-probe]").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedProbe = button.dataset.probe;
      renderNodeDetail(nodeID);
    });
  });
  document.querySelectorAll("[data-connectivity]").forEach((button) => {
    button.addEventListener("click", () => {
      const [direction, metric] = button.dataset.connectivity.split(":");
      state.connectivityDirection = direction;
      state.connectivityMetric = metric;
      renderNodeDetail(nodeID);
    });
  });
}

function drawChart(containerID, points, options) {
  const container = document.getElementById(containerID);
  if (!container) return;
  if (!points.length) {
    container.innerHTML = `<div class="empty">暂无曲线数据</div>`;
    return;
  }
  const width = container.clientWidth || 900;
  const height = 210;
  const pad = { left: 44, right: 18, top: 18, bottom: 32 };
  const values = points.map((point) => Number(point.value || 0));
  const min = Math.min(0, ...values);
  const max = Math.max(options.max || 0, ...values, 1);
  const span = max - min || 1;
  const x = (index) => pad.left + (index / Math.max(points.length - 1, 1)) * (width - pad.left - pad.right);
  const y = (value) => pad.top + (1 - (value - min) / span) * (height - pad.top - pad.bottom);
  const path = values.map((value, index) => `${index ? "L" : "M"} ${x(index).toFixed(1)} ${y(value).toFixed(1)}`).join(" ");
  const grid = [0, 0.25, 0.5, 0.75, 1].map((ratio) => {
    const gy = pad.top + ratio * (height - pad.top - pad.bottom);
    const label = (max - ratio * span).toFixed(options.decimals ?? 0);
    return `<line x1="${pad.left}" y1="${gy}" x2="${width - pad.right}" y2="${gy}" stroke="#e5e7eb" stroke-dasharray="3 4"/><text x="8" y="${gy + 4}" fill="#64748b" font-size="11">${label}</text>`;
  }).join("");
  const labels = sampleLabels(points, 8).map(({ point, index }) => `<text x="${x(index)}" y="${height - 8}" text-anchor="middle" fill="#64748b" font-size="11">${timeLabel(point.ts)}</text>`).join("");
  container.innerHTML = `
    <svg width="100%" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-label="${options.label}">
      ${grid}
      <path d="${path}" fill="none" stroke="${options.color}" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/>
      ${values.map((value, index) => `<circle cx="${x(index)}" cy="${y(value)}" r="2.2" fill="${options.color}" opacity="${index % Math.ceil(points.length / 32 || 1) === 0 ? 1 : 0}"/>`).join("")}
      ${labels}
    </svg>
  `;
}

function sampleLabels(points, count) {
  if (points.length <= count) {
    return points.map((point, index) => ({ point, index }));
  }
  const step = (points.length - 1) / (count - 1);
  return Array.from({ length: count }, (_, slot) => {
    const index = Math.round(slot * step);
    return { point: points[index], index };
  });
}

function chartOptions(metric) {
  const percent = ["cpu", "memory", "disk", "success_rate", "timeout_rate", "probe_loss_rate"].includes(metric);
  const palette = {
    cpu: "#2563eb",
    memory: "#10b981",
    network: "#7c3aed",
    disk: "#f59e0b",
    app_rtt: "#2563eb",
    jitter: "#f59e0b",
    probe_loss_rate: "#ef4444",
  };
  return {
    label: labels[metric] || metric,
    color: palette[metric] || "#2563eb",
    max: percent ? 100 : 0,
    decimals: percent ? 0 : 1,
  };
}

function skeleton(text) {
  return `<div class="card card--pad empty">${escapeHTML(text)}</div>`;
}

function errorView(error) {
  return `<div class="card card--pad empty">无法加载 Nexis Console API：${escapeHTML(error.message)}</div>`;
}

function badgeClass(status) {
  const name = status || "offline";
  const health = ["healthy", "degraded", "critical"].includes(name);
  return `badge badge--${health ? name : name}`;
}

function statusText(status) {
  return { online: "在线", stale: "延迟", offline: "离线", healthy: "健康", degraded: "波动", critical: "异常" }[status] || "未知";
}

function platform(node) {
  const stats = node.latest_stats || {};
  const meta = node.meta || {};
  const os = stats.os || meta.os || "linux";
  const arch = stats.arch || meta.arch || "unknown";
  return `${os}/${arch}`;
}

function uptimeText(node) {
  const seconds = node.latest_stats?.uptime_seconds;
  if (seconds) return durationText(seconds);
  if (!node.created_at) return "-";
  return durationText((Date.now() - new Date(node.created_at).getTime()) / 1000);
}

function durationText(seconds) {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}天 ${hours}小时 ${minutes}分`;
  if (hours > 0) return `${hours}小时 ${minutes}分`;
  return `${minutes}分`;
}

function relativeTime(value) {
  if (!value) return "-";
  const diff = Math.max(0, (Date.now() - new Date(value).getTime()) / 1000);
  if (diff < 60) return `${Math.round(diff)}秒前`;
  if (diff < 3600) return `${Math.round(diff / 60)}分钟前`;
  return `${Math.round(diff / 3600)}小时前`;
}

function formatMs(value) {
  const number = Number(value || 0);
  if (!number) return "-";
  return `${number.toFixed(number >= 100 ? 0 : 1)}ms`;
}

function timeLabel(value) {
  const date = new Date(value);
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function seriesTitle(series) {
  const direction = series.direction === "reverse" ? "回程（逻辑）" : "去程";
  return `${direction} ${labels[series.metric] || series.metric}`;
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
