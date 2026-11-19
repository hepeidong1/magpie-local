// 请求日志（plugins/requests）：网关正在服务、以及刚服务完的每一条请求，
// 一条一行。数据来自路由轨迹（/api/requests-log，读 gateway.Trace）——
// 请求一进来轨迹里就有它，Order 是该组整条链路，Tries 随每次尝试实时
// 更新，所以页面能显示“回答中”，并把每个条目按已试过的顺序着色。
//
// 500ms 一拍（每秒 2 次）。经典脚本，在 app.js 之前加载：注册即可，
// mount 在页面显示时运行，离开页面停止轮询。

const REQ_TICK = 500; // 每秒刷新 2 次

// 日志的列：时间 / Agent / Key / 请求模型 / 应答供应商 / 应答模型 /
// 状态 / 输入 / 输出 / 缓存 / 首字 / 总耗时 / 速度 /（展开）
const REQ_COLS = ["时间", "Agent", "Key", "请求模型", "应答供应商", "应答模型",
  "状态", "输入", "输出", "缓存", "首字", "总耗时", "速度", ""];

let reqLog = [], reqTick = 0;
// reqOpen 是展开着的请求 id 集合：每 500ms 重画时保持展开，不自己关掉
const reqOpen = new Set();

function reqClock(t) {
  const d = new Date(t);
  return isNaN(d) ? "—" : d.toLocaleTimeString("zh-CN", { hour12: false });
}

// reqState 是一行请求的状态：还在跑就是“回答中”，否则成功/失败。
function reqState(r) {
  if (!r.done) return { cls: "waiting", label: "回答中" };
  if ((r.status || 0) >= 400 || r.error) return { cls: "fail", label: "失败" };
  return { cls: "ok", label: r.status ? String(r.status) : "成功" };
}

function renderReqLog() {
  const box = $("#reqLog");
  if (!box) return;
  box.replaceChildren();
  const table = el("table", "led req-led");
  const thead = el("thead"), tbody = el("tbody");
  const head = el("tr");
  for (const name of REQ_COLS) head.append(el("th", "", name));
  thead.append(head);
  // 显示全部（服务端最多回 100 条+在途）：表体超出滚动，表头吸顶
  for (const r of reqLog) {
    const row = reqLogRow(r);
    tbody.append(row);
    if (row._detail) tbody.append(row._detail);
  }
  table.append(thead, tbody);
  box.append(table);
}

function reqLogRow(r) {
  const st = reqState(r);
  const tr = el("tr", "led-row req-" + st.cls);
  const td = (child, cls, title) => {
    const c = el("td", cls || "");
    if (typeof child === "string") c.textContent = child; else c.append(child);
    if (title) c.title = title;
    tr.append(c);
    return c;
  };
  td(reqClock(r.time), "when", new Date(r.time).toLocaleString(intlLang()));
  // Agent：认得到的用它的图标
  const who = el("span", "who");
  who.append(icon(reqAgentIcon(r.agent)), el("span", "", r.agent || "—"));
  td(who, "");
  // Key：列窄，最多显示 3 个字（名字一般就两三个），全名放 title；
  // 用 req-key 类，不借用 app.css 的 .key（那会让它带上圆点+圆角卡片）
  const keyName = r.key || "";
  td(keyName.length > 3 ? keyName.slice(0, 3) : (keyName || "—"),
    "req-key" + (keyName ? "" : " req-key-none"), keyName || "");
  // 请求模型：agent 问的名字（组的暴露名）
  td(r.model || "—", "model");
  // 应答供应商：名字 + 图标
  const face = (providers?.providers || []).find((p) => p.id === r.answeredProvider);
  const line = el("div", "where-line");
  line.append(...(face ? [icon(face.icon || "generic")] : []),
    el("div", "where-name", r.answeredProvider ? (face?.name || r.answeredProvider) : "—"));
  td(line, "where" + (r.answeredProvider ? "" : " faint"), r.answeredProvider || "");
  // 应答模型：回复里自称的那个；没有就 —
  td(r.answeredModel || "—", "model" + (r.answeredModel ? "" : " faint"));
  // 状态：圆点 + 文字（回答中橙 / 失败红 / 成功绿）；回答中的圆点转圈
  const stc = el("span", "st st-" + st.cls);
  const dot = el("i", "dot");
  if (st.cls === "waiting") dot.classList.add("spin");
  stc.append(dot, document.createTextNode(st.label));
  td(stc, "", r.error || "");
  td(r.input ? ledNum(r.input) : "—", "n" + (r.input ? "" : " faint"));
  td(r.output ? ledNum(r.output) : "—", "n" + (r.output ? "" : " faint"));
  td(r.cache ? ledNum(r.cache) : "—", "n" + (r.cache ? "" : " faint"));
  td(r.ttft > 0 ? ledTook(r.ttft) : "—", "n ttft" + (r.ttft > 0 ? "" : " faint"));
  const untimed = !(r.ms > 0);
  const band = untimed ? "faint" : r.ms <= 10000 ? "duration-fast" : r.ms <= 30000 ? "duration-slow" : "duration-long";
  td(untimed ? "—" : ledTook(r.ms), "n duration " + band);
  const v = r.flow > 0 && r.output > 0 ? r.output / (r.flow / 1000)
    : !untimed && r.output > 0 ? r.output / (r.ms / 1000) : 0;
  td(v ? ledNum(Math.round(v)) + " tok/s" : "—", "n speed" + (v ? "" : " faint"));
  // 展开：这条请求所请求组的全部条目。点击整行任意位置都能展开/收起
  const path = Array.isArray(r.path) ? r.path : [];
  if (path.length) {
    const caret = el("button", "req-open");
    caret.type = "button";
    caret.title = "展开组内全部条目（" + path.length + " 条）";
    caret.setAttribute("aria-label", "展开组内全部条目");
    caret.append(svg("M4 6l4 4 4-4", 10, 1.7));
    const detail = el("tr", "req-path-open");
    const cell = el("td", "");
    cell.colSpan = REQ_COLS.length;
    cell.append(reqPathFlow(path));
    detail.append(cell);
    const open = reqOpen.has(r.id);
    detail.hidden = !open;
    caret.setAttribute("aria-expanded", String(open));
    caret.classList.toggle("open", open);
    const toggle = () => {
      const now = detail.hidden;
      detail.hidden = !now;
      caret.setAttribute("aria-expanded", String(now));
      caret.classList.toggle("open", now);
      if (now) reqOpen.add(r.id); else reqOpen.delete(r.id);
    };
    // 整行可点（但别拦下展开按钮自己的点击、也别拦下选中文字）
    tr.classList.add("req-clickable");
    tr.addEventListener("click", (e) => {
      if (e.target.closest(".req-open")) return; // 按钮自己处理
      if (window.getSelection && String(window.getSelection())) return; // 正在选字
      toggle();
    });
    caret.addEventListener("click", (e) => { e.stopPropagation(); toggle(); });
    td(caret, "req-open-cell");
    tr._detail = detail;
  }
  return tr;
}

// reqPathFlow 是这条请求所请求组的全部条目，按组的顺序排成紧凑的一段
// （不是表格，也不是一条一行）：条目一个接一个地流动排布，每个条目是
//   序号（长方形：失败红底白字、成功绿底白字、未轮到/跳过灰底黑字黑框）
//   [图标]供应商 模型（灰字） 原因（失败红字；成功 ·200·成功 绿字；
//   跳过/未轮到无字）
function reqPathFlow(path) {
  const box = el("div", "req-flow");
  path.forEach((s, i) => {
    const item = el("span", "req-flow-item req-flow-" + (s.result || "skip"));
    // 序号徽标
    item.append(el("span", "req-no", String(i + 1)));
    // 供应商：图标 + 名字（providerJSON 里有 name/icon）；成功/失败时
    // 供应商与模型也跟着徽标一起变绿/变红
    const face = (providers?.providers || []).find((p) => p.id === s.provider);
    const name = face?.name || s.provider || "";
    const ref = el("span", "req-ref");
    if (face) ref.append(icon(face.icon || "generic"));
    ref.append(document.createTextNode((name || "—") + " " + reqShortModel(s.model, s.provider)));
    item.append(ref);
    // 原因
    let why = "";
    let cls = "req-why";
    if (s.result === "ok") {
      why = "·" + (s.status || 200) + "·成功";
      cls += " req-why-ok";
    } else if (s.result === "fail") {
      why = (s.status ? s.status + " · " : "") + (s.why || "失败");
      cls += " req-why-fail";
    } else if (s.result === "waiting") {
      why = "·回答中";
      cls += " req-why-waiting";
    }
    // 跳过/未轮到：无字（原因放 title，悬停可见）
    if (why) item.append(el("span", cls, why));
    if (s.who) item.title = (name || "") + " " + (s.model || "") + " @ " + s.who;
    else if (s.result === "skip" && s.why) item.title = s.why;
    box.append(item);
  });
  return box;
}

// reqShortModel: 模型名去掉前面的供应商段（"provider/model" → "model"），
// 供应商列/图标已说明了是谁
function reqShortModel(model, provider) {
  const m = model || "";
  if (provider && m.startsWith(provider + "/")) return m.slice(provider.length + 1);
  return m;
}

// reqAgentIcon 是请求页认识的 Agent 图标：Agents 页里有的用它的，
// 没有的退回通用形
function reqAgentIcon(agent) {
  const a = (state.agents || []).find((x) => x.id === agent || x.name === agent);
  return a?.icon || "generic";
}

// 500ms 一拍，取请求日志。失败时留着上一拍的数据照常显示——网关重启
// 或一次超时不至于把日志清成白屏；只有真的取到列表才替换。
async function reqRefresh() {
  let rows;
  try {
    rows = await api("requests-log");
  } catch { return; }
  if (!Array.isArray(rows)) return;
  reqLog = rows;
  // 展开着的请求已经不在了（被轨迹挤掉）：把 id 也清掉，别越攒越多
  const live = new Set(rows.map((r) => r.id));
  for (const id of [...reqOpen]) if (!live.has(id)) reqOpen.delete(id);
  renderReqLog();
}

function reqWatch() {
  if (reqTick) return;
  reqTick = setInterval(async () => {
    if (document.hidden || view !== "requests") {
      clearInterval(reqTick);
      reqTick = 0;
      return;
    }
    await reqRefresh();
  }, REQ_TICK);
}

function renderRequests() {
  renderReqLog();
  reqWatch();
}

magpie.plugin("requests", (ctx) => {
  ctx.slots.add("page", {
    key: "requests",
    title: "请求",
    loads: "providers",
    html: `
<div class="row-head"><span class="label">请求日志</span><span class="grow"></span></div>
<div id="reqLog" class="req-card req-log"></div>
`,
    mount() {
      renderRequests();
    },
    draft() {
      // 回到页面时轮询可能已自清（离开时 view 不对就 clearInterval），
      // 重进必须重新武装，不然日志就不动了；再立即拉一次，页面不空等
      renderRequests();
      reqRefresh();
    },
    render() {
      renderRequests();
    },
  });
});