// The Entry list page (plugins/entries): one row for every model a
// provider serves and every key or account that can take it, grouped
// provider → model → key. Each row is what the entry is: its own
// 6-character code, its provider and model, and the context its model
// takes. Its concurrency lives in the routing groups' boards now (the
// twenty balls a group's page keeps), not here.
//
// Classic script, loaded before app.js: what runs as it loads touches
// neither the page nor app.js; the page registers at once.

// entryGroups is what the page draws: each provider's exposed models, and
// the keys or accounts that can take them. Providers keep the order the
// Providers page has (the user's); models are by name, keys by the order
// requests try them (KeyList).
function entryGroups() {
  const out = [];
  for (const p of providers?.providers || []) {
    if (p.off) continue;
    const models = (p.models || []).filter((m) => m.on)
      .sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id, intlLang()));
    if (!models.length) continue;
    const creds = [];
    (p.keyList || []).forEach((k, i) => {
      if (k.on) creds.push({ pid: p.id, id: k.id, label: k.name || "#" + (i + 1) });
    });
    // a subscription's accounts: each sign-in magpie keeps in use
    for (const l of p.account?.logins || []) if (l.on) creds.push({ pid: p.id, id: "login:" + l.user, label: l.user });
    if (!creds.length && p.account?.user) creds.push({ pid: p.id, id: "account:" + p.account.user, label: p.account.user });
    if (!creds.length) continue;
    out.push({ provider: p, models, creds });
  }
  return out;
}

function renderEntriesLoading() {
  const box = $("#entries");
  box.replaceChildren();
  $("#entriesNote").textContent = "";
  const card = el("div", "entry-card");
  for (let i = 0; i < 4; i++) {
    const row = el("div", "entry-row");
    for (let c = 0; c < ENTRY_COLS.length; c++) {
      const cell = el("div", "entry-cell " + ENTRY_COLS[c][1]);
      cell.append(el("span", "skeleton " + (ENTRY_COLS[c][1] === "c-lane" ? "entry-sk-bar" : "entry-sk-name")));
      row.append(cell);
    }
    card.append(row);
  }
  box.append(card);
}

// ctxWords says the model's largest context window in thousands: 200K,
// 1000K, 1049K; unknown (清单没说) shows the default 200K, the same
// WindowUnknown the gateway assumes.
function ctxWords(m) {
  const n = m.context || m.listed || 0;
  if (!n) return "200K";
  return Math.round(n / 1e3) + "K";
}

// the page's columns: the in-any-group mark, what the entry is (provider,
// model, key, its own 6-character code), and what its model can take
// (context)
const ENTRY_COLS = [
  ["组", "c-mark"], ["ID", "c-code"], ["Provider", "c-prov"], ["Model", "c-model"], ["Key", "c-key"], ["Max context", "c-ctx"],
];

// entryInGroup / entryAllRefs 是路由组收下的条目：plan 成员的"ref@凭据"
// 键形集合（凭据按钉的形态展开：指纹、登录十六进制映回的用户名、login:
// 前缀），和没钉凭据的 ref（"它的哪一条都算"，盖住模型的全部凭据）。
// 一列问号：都不沾的条目就是还没有路由组加入的。
let entryInGroup = new Set(), entryAllRefs = new Set();
async function loadEntryGroupMarks() {
  let g;
  try { g = await api("groups"); } catch { return; }
  const logins = new Map();
  for (const p of providers?.providers || []) for (const l of p.account?.logins || []) if (l.id && l.user) logins.set(String(l.id).toLowerCase(), l.user);
  const set = new Set(), all = new Set();
  const addCred = (ref, cred) => {
    cred = String(cred || "").replace(/^login:/, "");
    if (!cred) { all.add(ref); return; }
    set.add(ref + "@" + cred);
    set.add(ref + "@login:" + cred);
    const u = logins.get(cred.toLowerCase());
    if (u) { set.add(ref + "@" + u); set.add(ref + "@login:" + u); }
  };
  const collect = (n) => {
    if (!n || typeof n !== "object") return;
    if (n.kind === "entry") addCred(n.ref, n.cred);
    if (n.members) n.members.forEach(collect);
  };
  for (const grp of g.groups || []) {
    if (grp.plan) collect(grp.plan);
    for (const m of grp.members || []) if (m) all.add(String(m));
  }
  entryInGroup = set;
  entryAllRefs = all;
}

// markIn 是这一行（pid/model@cred）有没有被路由组收下：没钉凭据的组
// 成员盖住全部凭据；钉了的按行凭据的几种拼法对（login:/account: 前缀
// 的剥掉再试裸的）。
function markIn(pid, mid, cred) {
  const ref = pid + "/" + mid;
  if (entryAllRefs.has(ref)) return true;
  const c = String(cred || "");
  const tries = [c];
  if (c.startsWith("login:") || c.startsWith("account:")) tries.push(c.slice(c.indexOf(":") + 1));
  return tries.some((x) => entryInGroup.has(ref + "@" + x));
}

// entryCode is the entry's own 6-character code ("provider/model@cred"),
// made once by the backend and kept in entry-codes.json: routing groups
// tell their members apart by it later.
function entryCode(c, m) {
  return (providers?.entryCodes || {})[c.pid + "/" + (m?.id || "") + "@" + (c.id || "")] || "";
}

// entryRow is one grid row: a cell per column, in the order above. The
// grid, not a table, keeps the columns inside the card: a table's fixed
// layout still takes its min-content width as a floor and blows past the
// view it sits in. cols is the page's columns (ENTRY_COLS unless another
// page draws fewer, as plugins/suppliers does).
function entryRow(cls, cells, cols = ENTRY_COLS) {
  const row = el("div", "entry-row " + cls);
  row.setAttribute("role", "row");
  cells.forEach((node, i) => {
    const cell = el("div", "entry-cell " + cols[i][1]);
    if (node != null) cell.append(node);
    cell.setAttribute("role", cls === "head" ? "columnheader" : "cell");
    row.append(cell);
  });
  return row;
}

// entryText is a cell's plain text, with the whole of it in the tooltip
// when the column cuts it short.
function entryText(text, title) {
  const span = el("span", "", text);
  if (title) span.title = title;
  return span;
}

function renderEntries() {
  const box = $("#entries");
  if (!box) return;
  box.replaceChildren();
  const groups = entryGroups();
  const total = groups.reduce((n, g) => n + g.models.length * g.creds.length, 0);
  $("#entriesNote").textContent = total ? t(total === 1 ? "1 entry" : "{n} entries", { n: total }) : "";
  if (!groups.length) {
    const empty = el("div", "empty-state");
    empty.append(el("b", "", t("No entries yet")), t("Add a model and a key under Providers; each model and key pair shows here."));
    box.append(empty);
    return;
  }
  const grid = el("div", "entry-grid");
  grid.setAttribute("role", "table");
  grid.setAttribute("aria-label", t("Entry list"));
  grid.append(entryRow("head", ENTRY_COLS.map(([label]) => document.createTextNode(t(label)))));
  for (const g of groups) {
    let first = true;
    for (const m of g.models) {
      for (const c of g.creds) {
        const pname = g.provider.name || g.provider.id;
        const mname = m.name || m.id;
        const prov = document.createDocumentFragment();
        prov.append(icon(g.provider.icon || "generic"), entryText(pname, pname));
        const model = document.createDocumentFragment();
        model.append(entryText(mname, m.id + " @ " + (c.label || "")));
        const code = entryCode(c, m);
        // 凭据列：密钥显示用户起的名字（没起名给 #序号），账号显示
        // 登录名；悬停给全名（provider/model@cred）
        const key = entryText(c.label || "—", (g.provider.id || "") + "/" + m.id + " @ " + (c.label || ""));
        // 组列：没有任何路由组收下这条的，打一个问号
        const grouped = markIn(g.provider.id, m.id, c.id);
        const mark = entryText(grouped ? "" : "？", grouped ? "" : "还没有路由组加入这个条目");
        grid.append(entryRow(first ? "group-start" : "", [
          mark, entryText(code, code), prov, model, key, entryText(ctxWords(m)),
        ]));
        first = false;
      }
    }
  }
  const card = el("div", "entry-card");
  card.append(grid);
  box.append(card);
}

magpie.plugin("entries", (ctx) => {
  ctx.slots.add("page", {
    key: "entries",
    title: "Entry list",
    // what the page draws from: the shell reads /api/providers for it
    loads: "providers",
    html: `
<div class="row-head"><span class="label" data-t>Entry list</span><span class="grow"></span><span class="note" id="entriesNote"></span></div>
<div id="entries"></div>
`,
    loading: renderEntriesLoading,
    draft() {
      if (!$("#entries").childElementCount) renderEntries();
      loadEntryGroupMarks().then(renderEntries);
    },
    render() {
      renderEntries();
      loadEntryGroupMarks().then(renderEntries);
    },
  });
});
