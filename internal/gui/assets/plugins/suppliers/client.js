// The Supplier list page (plugins/suppliers): the same board as the Entry
// list page (plugins/entries) with the keys folded away — one row for
// every model a provider serves, however many keys or accounts can take
// it. It keeps the 组 mark, the provider, the model and its context; the
// entry rows' 识别码 (ID) and 密钥 (Key) columns are not here, so nothing
// of whose key it is shows.
//
// The context is two columns: 原上下文长度 (Original window), what the
// provider's own list reported, "—" when it reported none; and 现上下文
// 长度 (Current window), a box in K (200 = 200K) holding the window in
// effect — the user's, else the provider's, else the fixed 200K
// (provider.WindowUnknown). A window the provider reported locks the box;
// one it didn't is editable, its number grey at the 200K default and green
// at a value the user set.
//
// Classic script, loaded after plugins/entries/client.js: it draws with
// that page's helpers (entryGroups, entryRow, entryText, ctxWords, markIn,
// loadEntryGroupMarks) and registers its own page here.

const SUPPLIER_COLS = [
  ["组", "c-mark"], ["Provider", "c-prov"], ["Model", "c-model"],
  ["Original window", "c-ctx-raw"], ["Current window", "c-ctx"],
];

function renderSuppliersLoading() {
  const box = $("#suppliers");
  box.replaceChildren();
  $("#suppliersNote").textContent = "";
  const card = el("div", "entry-card");
  for (let i = 0; i < 4; i++) {
    const row = el("div", "entry-row");
    for (let c = 0; c < SUPPLIER_COLS.length; c++) {
      const cell = el("div", "entry-cell " + SUPPLIER_COLS[c][1]);
      cell.append(el("span", "skeleton " + (SUPPLIER_COLS[c][1].startsWith("c-ctx") ? "entry-sk-bar" : "entry-sk-name")));
      row.append(cell);
    }
    card.append(row);
  }
  box.append(card);
}

// markModel is a model's 组 mark with its keys folded in: a group holds it
// if it names the provider/model without a credential, or pins any of the
// keys that serve it.
function markModel(pid, mid, creds) {
  const ref = pid + "/" + mid;
  if (entryAllRefs.has(ref)) return true;
  return creds.some((c) => markIn(pid, mid, c.id));
}

// ctxEntry is the 现上下文长度 cell: an editable box in K (200 = 200K).
// 回车或失焦保存到 provider.SetContext；清空恢复默认（200K）。上游能
// 拉到的（原上下文有值）锁定不可改；拉不到的，值 200K 灰、其他值绿。
function ctxEntry(g, m) {
  const shown = () => Math.max(0, Math.round((m.context || 0) / 1e3));
  const known = (m.raw || 0) > 0; // 上游报了的：窗口是它的，锁住
  const state = known ? " locked" : shown() === 200 ? " plain200" : " custom";
  const box = el("span", "sp-ctx-box");
  const input = el("input", "sp-ctx" + state);
  input.type = "text";
  input.inputMode = "numeric";
  input.value = String(shown());
  input.readOnly = known;
  input.title = known
    ? "上游报告了窗口（" + ctxWords({ context: m.raw }) + "），此项锁定；要改请去供应商编辑器"
    : shown() === 200
      ? "上游没报窗口，这是默认 200K（灰色）；输入数字改为用户设置（绿色）"
      : "用户设置的" + shown() + "K 窗口（绿色）；清空恢复默认 200K（灰色）";
  box.append(input, el("span", "sp-ctx-k", "K"));
  const note = $("#suppliersNote");
  const commit = async () => {
    if (known) return;
    const raw = input.value.trim().replace(/[kK]\s*$/, "");
    const k = raw === "" || raw === "0" ? 0 : Number(raw);
    if (raw !== "" && raw !== "0" && (!Number.isFinite(k) || k <= 0 || k > 100000)) {
      input.value = String(shown());
      note.textContent = "上下文长度不是 " + raw + "：应填 K 数（如 200 = 200K）";
      return;
    }
    if (Math.round(k) === shown()) return; // 没改
    input.disabled = true;
    try {
      providers = await api("provider/context", { id: g.provider.id, model: m.id, context: Math.round(k) * 1000 });
      renderSuppliers();
    } catch (e) {
      input.disabled = false;
      input.value = String(shown());
      note.textContent = String(e?.message || e);
      setTimeout(() => { renderSuppliers(); }, 4000);
    }
  };
  input.onchange = commit;
  input.onkeydown = (e) => { if (e.key === "Enter") { e.preventDefault(); input.blur(); } };
  input.onfocus = () => { if (!known) input.select(); };
  return box;
}

function renderSuppliers() {
  const box = $("#suppliers");
  if (!box) return;
  box.replaceChildren();
  const groups = entryGroups();
  const total = groups.reduce((n, g) => n + g.models.length, 0);
  $("#suppliersNote").textContent = total ? t("{n} models", { n: total }) : "";
  if (!groups.length) {
    const empty = el("div", "empty-state");
    empty.append(el("b", "", t("No models yet")), t("Add a model and a key under Providers; each model shows here."));
    box.append(empty);
    return;
  }
  const grid = el("div", "entry-grid");
  grid.setAttribute("role", "table");
  grid.setAttribute("aria-label", t("Provider list"));
  grid.append(entryRow("head", SUPPLIER_COLS.map(([label]) => document.createTextNode(t(label))), SUPPLIER_COLS));
  for (const g of groups) {
    let first = true;
    for (const m of g.models) {
      const pname = g.provider.name || g.provider.id;
      const mname = m.name || m.id;
      const prov = document.createDocumentFragment();
      prov.append(icon(g.provider.icon || "generic"), entryText(pname, pname));
      const model = document.createDocumentFragment();
      model.append(entryText(mname, g.provider.id + "/" + m.id));
      const grouped = markModel(g.provider.id, m.id, g.creds);
      const mark = entryText(grouped ? "" : "？", grouped ? "" : "还没有路由组加入这个模型");
      // 原上下文：供应商自己清单报的，没报就 "—"；现上下文：生效值（用户 >
      // 供应商 > 缺省 200K），可直接编辑，单位 K
      const rawN = m.raw || 0;
      const raw = entryText(rawN ? ctxWords({ context: rawN }) : "—",
        rawN ? "上游清单报的窗口" : "上游没报窗口，原上下文长度未知");
      grid.append(entryRow(first ? "group-start" : "", [
        mark, prov, model, raw, ctxEntry(g, m),
      ], SUPPLIER_COLS));
      first = false;
    }
  }
  const card = el("div", "entry-card");
  card.append(grid);
  box.append(card);
}

magpie.plugin("suppliers", (ctx) => {
  ctx.slots.add("page", {
    key: "suppliers",
    // the same words the old page had: 供应商列表 (Provider list)
    title: "Provider list",
    // what the page draws from: the shell reads /api/providers for it
    loads: "providers",
    html: `
<div class="row-head"><span class="label" data-t>Provider list</span><span class="grow"></span><span class="note" id="suppliersNote"></span></div>
<div id="suppliers"></div>
`,
    loading: renderSuppliersLoading,
    draft() {
      if (!$("#suppliers").childElementCount) renderSuppliers();
      loadEntryGroupMarks().then(renderSuppliers);
    },
    render() {
      renderSuppliers();
      loadEntryGroupMarks().then(renderSuppliers);
    },
  });
});