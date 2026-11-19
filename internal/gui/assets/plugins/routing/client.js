// Routing, live: the gateway's own trace of each request, played as it
// happens. A magpie carries each request from its agent through magpie to
// the account routing put first; another brings the answer back — or,
// from one that can't answer, the failure back to magpie, and the first
// takes the request on to the next. Every agent sending at once plays at
// once, each from its own place on the left. Every row, number and sentence comes from what the gateway
// recorded while deciding (see internal/gateway/trace.go) — the order it
// weighed the accounts in, what it weighed them by, what each answered,
// how long a failed one rests. Nothing here is worked out again or made up.
//
// A page the Routing plugin adds to the window (kernel.js), its words in
// i18n.js beside this file. Classic script, loaded before app.js: what
// runs as it loads touches neither the page nor app.js. Its markup is the
// html below; mount, at magpie.start, draws the stage and starts reading
// the gateway's trace, and from then on the page keeps itself (it watches
// its own hidden, the language and the window). dirty and discard are a
// routing group being edited, asked of it before the reader leaves.
magpie.plugin("routing", (ctx) => {
  // the requests page went with the live trace; the context is kept so
  // app.js can add one page per routing-group tab (routeTabs, app.js)
  window.routeTabsCtx = ctx;
});

// The routing groups' section, drawn into the gateway page's host
// (plugins/gateway) from that page's mount, once app.js is set up.
//
// opts.single names one group: the section then draws that group's editor
// alone, which is what a sidebar tab is (app.js's routeTabs). opts.host is
// the page it draws into; a tab of no group yet opens a new group's editor.
function routingGroups(opts) {
  opts = opts || {};
  const only = typeof opts.only === "string" ? opts.only : "";
  const single = !!opts.single;
  const host = opts.host || document.getElementById("rtGsecHost");
  const pageOf = () => (host ? host.closest(".view") : document.getElementById("view-gateway"));
  const shownG = () => { const v = pageOf(); return !!v && !v.hidden && !document.hidden; };
  const still = calm;
  // ---------- routing groups ----------
  // The groups agents can pick as one model (group/<id>), made, changed or
  // removed here. Each runs one plan: its entries in the order it tries
  // them, several asked at once. Found groups, the agents'-list name
  // setting and the providers' own account/key pools went with the old
  // strategies (the doc's §5.1).
  const gsec = el("div", "rt-gsec");
  const gHead = el("div", "row-head"), gList = el("div", "list rt-groups");
  // the groups go with the routing groups built-in plugin
  for (const n of [gHead, gList]) n.dataset.feature = "groups";
  // and off (the Essentials preset leaves it off), where they were
  // the page says so and turns it on (bigbenniu on X: past the requests
  // there was nothing, and no word of where New group had gone). The
  // groups magpie finds route all the same.
  const gOff = el("div", "rt-goff");
  gOff.dataset.featureOff = "groups";
  const gOn = el("button", "text rt-goffon", t("Turn on routing groups"));
  gOn.type = "button";
  gOn.onclick = async () => {
    gOn.disabled = true;
    try {
      const next = await api("plugins/builtin", { id: "groups", on: true });
      if (state.settings) state.settings.builtins = next;
      applyFeatures(next);
      status(t("Routing groups are on"), "ok", 1500);
      loadGroups();
    } catch (err) {
      status(t(err.message), "err");
    } finally {
      gOn.disabled = false;
    }
  };
  const gOffBox = el("div", "list rt-goffbox");
  gOffBox.append(el("div", "rt-goffwhy", t("Routing groups are off in Plugins › Built-in: your own can't be made or changed here.")), gOn);
  gOff.append(el("div", "row-head", el("span", "label", t("Routing groups"))), gOffBox);
  gsec.append(gHead, gList, gOff);
  // the groups have their own page since (the gateway plugin's host): not
  // under the requests any more
  host.append(gsec);
  // whether agents' model lists show the group's models too (#1783): no
  // longer set here, but what a group saved is still said on its row
  const LIST_HINT = {
    "": "Agents' model lists and the gateway's /v1/models show the group and each of its models.",
    unlists: "Agents' model lists and the gateway's /v1/models show the group, not the models it names or its patterns match: they are used through it. A signed-in subscription's models stay in agents' lists. A request naming one is still answered.",
  };
  const EFFORTS = ["low", "medium", "high", "xhigh", "max"]; // provider.Efforts
  // what a rule matches, in words
  function ruleText(r) {
    const bits = [];
    if (r.tokens) bits.push(t("≥ {n} tokens", { n: r.tokens.toLocaleString(intlLang()) }));
    if (r.images) bits.push(t("has an image"));
    if (r.effort) bits.push(r.effort === "on" ? t("reasoning on") : t("reasoning ≥ {level}", { level: r.effort }));
    if (r.agents?.length) bits.push(r.agents.map((id) => (state.clients || state.agents || []).find((a) => a.id === id)?.name || id).join(" / "));
    if (r.intent) bits.push(t("asks for “{intent}”", { intent: r.intent }));
    if (r.compact) bits.push(t("compacting"));
    if (r.time) bits.push(timeText(r.time));
    if (r.folders?.length) bits.push(t("in {folders}", { folders: r.folders.join(" / ") }));
    if (r.exceptFolders?.length) bits.push(t("not in {folders}", { folders: r.exceptFolders.join(" / ") }));
    return bits.join(" · ");
  }
  const slug = (s) => String(s || "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  let groups = null, gEdit = null; // gEdit: { id: "" for a new one, draft }
  // groupsFrom: /api/groups as the page draws it (#1585, dbydd). A group of
  // patterns that match nothing now — a plugin behind on updates lists none
  // of its models — came with its members as null, and that one group threw
  // out every group's card while each still routed. Every list is a list
  // here, and an entry that isn't a group at all is left out.
  function groupsFrom(x) {
    x = x && typeof x === "object" ? x : {};
    const objs = (v) => Array.isArray(v) ? v.filter((o) => o && typeof o === "object") : [];
    const ids = (v) => Array.isArray(v) ? v.filter((s) => typeof s === "string") : [];
    // the lists Go leaves out when empty stay out; null or a non-list is none
    const opt = (v, f) => v === undefined ? v : f(v);
    // a member's weight is a whole number; anything else is none
    const weightsOf = (v) => Object.fromEntries(Object.entries(v && typeof v === "object" && !Array.isArray(v) ? v : {}).filter(([, n]) => Number.isInteger(n)));
    const group = (g) => ({ ...g, id: String(g.id ?? ""), name: typeof g.name === "string" && g.name ? g.name : String(g.id ?? ""),
      members: ids(g.members), memberInfo: objs(g.memberInfo), holds: ids(g.holds), offers: ids(g.offers), shared: ids(g.shared), patterns: objs(g.patterns),
      match: opt(g.match, ids), matched: opt(g.matched, ids), off: opt(g.off, ids), fast: opt(g.fast, ids), levels: opt(g.levels, ids), weights: opt(g.weights, weightsOf),
      rules: opt(g.rules, (rs) => objs(rs).map((r) => ({ ...r, agents: opt(r.agents, ids), folders: opt(r.folders, ids), exceptFolders: opt(r.exceptFolders, ids) }))) });
    const refs = (v) => objs(v).filter((m) => typeof m.id === "string");
    return { ...x, groups: objs(x.groups).filter((g) => typeof g.id === "string" && g.id).map(group),
      models: refs(x.models), deciders: refs(x.deciders) };
  }
  // quiet: the window focused again; the same groups aren't drawn again
  async function loadGroups(quiet) {
    let next;
    try { next = groupsFrom(await api("groups")); } catch { return; }
    if (quiet === true && groups && JSON.stringify(next) === JSON.stringify(groups)) return;
    groups = next;
    if (!gEdit && !gsec.contains(document.activeElement)) renderGroups(); // not under someone's hands
  }
  const groupDirty = () => !!gEdit && gEdit.was !== undefined && JSON.stringify(gEdit.draft) !== gEdit.was;
  async function cancelGroup() {
    if (groupDirty() && !(await confirmDiscard())) return false;
    gEdit = null;
    renderGroups();
    return true;
  }
  // what app.js asks before the reader leaves the page. A sidebar tab has
  // its own instance and its own answer (the returned pair), so two tabs
  // never ask after one another's edit; only the groups page keeps the
  // window's, which show() asks of it.
  if (!single) {
    window.routingGroupsDirty = groupDirty;
    window.routingGroupsDiscard = () => { gEdit = null; renderGroups(); };
  }
  async function groupAction(action, body, ok) {
    if (action === "delete" && !await confirmRemoval(groups?.groups.find((g) => g.id === body.id)?.name || body.id,
      "This routing group will no longer be available to agents.")) return;
    try {
      groups = groupsFrom(await api("groups/" + action, body));
      if (action === "save" && gEdit) { gMorph = [gEdit.id, body.id]; gSaved = body.id; }
      gEdit = null;
      renderGroups();
      // a tab of a new group takes the group's id as it is saved (app.js's
      // routeTabs), and its label becomes the group's name
      if (action === "save") {
        const id = body.id || body.from || "";
        // the tab follows its group, and the editor that saved is its own
        if (id) tabId = id;
        if (gEdit) gEdit.id = id;
        opts.onSaved?.(id);
      }
      if (action === "delete") opts.onDeleted?.(body.id || "");
      // The editor closed on its Save, and the page is shorter by its height: the
      // click-hold then has nothing of the editor left to hold, and left to the
      // browser's own clamp the reader lands at the foot of the list — rows away
      // from the group saved, which was where they clicked. Its row comes back
      // into sight, as the reader's own scroll — and only when it really is out
      // of sight: readerScrolls lets go of the held element for a second, so
      // calling it on a row already in view would drop the hold the next editor
      // opens under (backToReader, app.js, guards the same way).
      const saved = body.id || body.from || "";
      const row = saved && gList.querySelector(`.rt-group[data-id="${CSS.escape(saved)}"]`);
      const view = row && gFolded !== saved && row.closest(".view");
      if (view) {
        const rr = row.getBoundingClientRect(), vb = view.getBoundingClientRect();
        if (rr.bottom > vb.bottom || rr.top < vb.top) { readerScrolls(1000); row.scrollIntoView({ block: "nearest" }); }
      }
      if (ok) status(ok, "ok");
      load(); // the gateway's model list, the agents' pickers
    } catch (e) { status(e.message, "err"); }
  }
  // a member may be a model at an effort of its own, "provider/model:low" —
  // unless the whole is a model's own id (a :free, a :7b); see
  // provider.MemberEffort. Several levels joined by "+" ("…:low+high",
  // Garydesu on X) are those the agent picks among (provider.MemberLevels),
  // kept each once, lowest first
  const LEVELS = ["none", "minimal", "low", "medium", "high", "xhigh", "max"]; // provider.MemberEfforts
  function splitMember(id) {
    id = id || "";
    if (id.startsWith("group/") || groups?.models.some((m) => m.id === id) || groups?.drawers?.some((m) => m.id === id)) return [id, ""];
    const m = /^(.+):((?:none|minimal|low|medium|high|xhigh|max)(?:\+(?:none|minimal|low|medium|high|xhigh|max))*)$/i.exec(id);
    if (!m) return [id, ""];
    const parts = m[2].toLowerCase().split("+");
    return [m[1], LEVELS.filter((v) => parts.includes(v)).join("+")];
  }
  // a decision model (Jev) is a member of a group of decision models only,
  // asked at /v1/systemone (provider/decide_group.go): it is in
  // groups.deciders, not groups.models
  const deciderOf = (id) => groups?.deciders?.find((m) => m.id === splitMember(id)[0]);
  // an image model (jzw690 on X) is in groups.drawers: a group holding it
  // draws on it at the images API, and it takes no reasoning
  const drawerOf = (id) => groups?.drawers?.find((m) => m.id === id);
  const modelOf = (id) => groups?.models.find((m) => m.id === id) || groups?.models.find((m) => m.id === splitMember(id)[0]) || deciderOf(id) || drawerOf(id);
  // a group's patterns (#766, provider.IsPattern): a glob, * any run of
  // characters over the provider/model id, or re:<regexp> over the whole
  // id, both in any case (provider.compilePattern). The group keeps them,
  // and the models they match now follow those it names, in the catalog's
  // order (Group.Matched)
  function patternRe(p) {
    try {
      if (p.startsWith("re:")) return goRe(p.slice(3));
      return new RegExp("^" + p.split("*").map((s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join(".*") + "$", "i");
    } catch { return null; }
  }
  // goRe is a re: pattern's regexp as Go reads it, in any case: Go's
  // leading flag group, "(?-i)" to make case matter, is JS's flags (U,
  // ungreedy, changes no whole-id match). null when it has none or JS
  // can't read it.
  function goRe(src) {
    if (!src.trim()) return null;
    const flags = new Set(["i"]);
    const lead = /^\(\?([imsU]*)(?:-([imsU]*))?\)/.exec(src);
    if (lead) {
      if (!lead[1] && !lead[2]) return null;
      for (const f of lead[1]) if (f !== "U") flags.add(f);
      for (const f of lead[2] || "") flags.delete(f);
      src = src.slice(lead[0].length);
    }
    return new RegExp("^(?:" + src + ")$", [...flags].join(""));
  }
  // the models served now the patterns match, but those the group names
  // (at any reasoning): as provider.matchesIn finds them
  function matchedBy(patterns, named) {
    const res = patterns.map(patternRe).filter(Boolean);
    const own = new Set(named.map((id) => splitMember(id)[0]));
    return (groups?.models || []).filter((m) => !own.has(m.id) && res.some((r) => r.test(m.id))).map((m) => m.id);
  }
  const patternWords = (n) => n ? t(n === 1 ? "1 model" : "{n} models", { n }) : t("matches nothing now");
  // what a group keeps of its members: those it names, not those its
  // patterns matched when it was read — they are found again each time
  const namedOf = (g) => (g.members || []).filter((x) => !(g.matched || []).includes(x));
  const fixedOf = (id) => subOf(id) ? "" : splitMember(id)[1];
  const fixedWords = (level) => t("{level} reasoning", { level: level.split("+").join(" / ") });
  // a routing group among a group's members: group/<id>
  const subOf = (id) => id?.startsWith("group/") ? groups?.groups.find((x) => "group/" + x.id === id && !x.hidden) : null;
  // a group's editor, its draft being the group as it is now. The group's own
  // card opens it, and so does a group that is a member of another: such a
  // member takes no reasoning of its own (provider.SaveGroup refuses
  // "group/x:high"), so its row says where its reasoning is set and goes
  // there, rather than leaving the slot empty.
  // what a new race group starts at: three entries asked at once, at most
  // eight streams held open (the ones timed out and still waiting
  // included) — the same pair an old group with a first-token timeout
  // migrates to (internal/provider synthPlan).
  const RACE_SEATS = 3, RACE_MAX = 8;
  // planRaceOf reads a group's saved scheduling tree (Group.Plan): the
  // order it tries its entries in, its concurrency, its cap on streams open
  // at once and its give-up seconds, or null when it has no race root. A
  // group from before the order was kept reads as quickest first, and one
  // from before the cap with the defaults a new one starts at.
  const planRaceOf = (g) => {
    const root = g?.plan;
    return root?.kind === "race" ? {
      orderBy: root.orderBy === "list" ? "list" : "speed",
      seats: root.seats || RACE_SEATS,
      maxInFlight: root.maxInFlight || RACE_MAX,
      totalWait: Math.round((root.totalWaitMs || 30000) / 1000),
      sticky: root.sticky !== false,
    } : null;
  };
  // what each entry answered last, read once a minute at most while a
  // group is edited: the gateway's own memory, keyed "provider/model@cred"
  // lowercased — the same key a member's id is. A restart forgets it, so
  // the columns say nothing until the entry answers again.
  let entrySpeeds = {}, entrySpeedsRead = 0;
  const readEntrySpeeds = () => {
    if (Date.now() - entrySpeedsRead < 60000) return Promise.resolve(false);
    entrySpeedsRead = Date.now();
    return api("entry-speeds").then((s) => {
      const had = JSON.stringify(s || {}) !== JSON.stringify(entrySpeeds);
      entrySpeeds = s || {};
      return had;
    }).catch(() => { entrySpeedsRead = 0; return false; });
  };
  const speedOfEntry = (id) => entrySpeeds[String(id).toLowerCase()] || null;
  // firstWords says an entry's last first token: the dash when nothing
  // tells yet, which is what the race ranks last.
  const firstWords = (ms) => !ms ? "—" : ms < 1000 ? t("{n} ms", { n: Math.round(ms) }) : t("{n} s", { n: +(ms / 1000).toFixed(1) });
  const speedWords = (tps) => !tps ? "—" : t("{n} tok/s", { n: Math.round(tps) });
  // planEntryRefsOf is a plan group's member refs (its entries), in
  // order: a plan group's old member fields are empty.
  // planEntryRefsOf is one key per plan entry, "ref@cred": the entry
  // table shows the credential, and the save pins it (a credless ref of
  // a single-credential entry saves as every-credential)
  const planEntryRefsOf = (g) => (g?.plan?.members || []).filter((m) => m.kind === "entry").map((m) => m.ref + (m.cred ? "@" + m.cred : ""));
  // planFtsOf is each entry's first-token timeout, by the entry's own id
  // ("ref@cred", what the rows key their inputs by), in seconds. Keyed by
  // the bare ref it read back nothing — every per-credential entry showed
  // an empty box, and the next save wiped the timeouts it couldn't see.
  const planFtsOf = (g) => {
    const out = {};
    for (const m of g?.plan?.members || []) if (m.kind === "entry" && m.firstTokenMs) out[m.ref + (m.cred ? "@" + m.cred : "")] = Math.round(m.firstTokenMs / 1000);
    return out;
  };
  const openGroupEditor = async (g) => {
    // a sidebar tab of its own, where the window has them: the group's
    // editor is the tab, and the groups page keeps its list (app.js's
    // routeTabs)
    if (!single && window.routeTabOpen) return void window.routeTabOpen(g.id, g.name);
    if (groupDirty() && !(await confirmDiscard())) return false;
    gEdit = { id: g.id, draft: { name: g.name, members: [...(g.members?.length ? g.members : planEntryRefsOf(g))], match: [...(g.match || [])], matched: [...(g.matched || [])], fast: [...(g.fast || [])], off: [...(g.off || [])], weights: { ...(g.weights || {}) }, unlists: !!g.unlists, context: g.context || 0, classifier: g.classifier || "", effort: g.effort || "", levels: [...(g.levels || [])], rules: (g.rules || []).map((r) => ({ ...r, ...(r.more?.length ? { more: [...r.more] } : {}), intent: r.intent || "", agents: [...(r.agents || [])], folders: [...(r.folders || [])], exceptFolders: [...(r.exceptFolders || [])], time: r.time ? { ...r.time, days: [...(r.time.days || [])] } : null })), race: planRaceOf(g) || { orderBy: "speed", seats: RACE_SEATS, maxInFlight: RACE_MAX, totalWait: 30, sticky: true }, fts: planFtsOf(g) } };
    renderGroups();
    return true;
  };
  // offIn: the members of a group magpie found whose provider is switched
  // off (XYenon on Discord). magpie keeps them in the group so agents on it
  // aren't moved while one is off (#2671), but the gateway skips them, so a
  // found group's card and logos leave them out, as every model list leaves
  // a provider switched off out, and say how many in a tag; the editor still
  // has them, greyed with why. A group of the user's lists what they put in.
  const offIn = (g) => g.auto ? (g.memberInfo || []).filter((i) => i.providerOff && g.members.includes(i.id)) : [];
  const shownMembers = (g) => { const off = offIn(g).map((i) => i.id); const left = g.members.filter((id) => !off.includes(id)); return left.length ? left : g.members; };
  const groupIcons = (g) => { const shown = shownMembers(g); return [...new Map((g.memberInfo || []).filter((i) => i.icon && shown.includes(i.id)).map((i) => [i.provider || i.icon, i.icon])).values()]; };
  // offOf: a member whose provider is switched off (groupsState's
  // providerOff), said by its provider's name and model rather than a bare
  // id: the group skips it until the provider is on again
  const offOf = (id) => { for (const g of groups?.groups || []) { const i = g.memberInfo?.find((x) => x.id === id && x.providerOff); if (i) return i; } };
  const offWords = (i) => t("{name} is switched off: the group skips it until it is on again", { name: i.name });
  const memberIcon = (id) => { const s = subOf(id); return s ? stackIcon(groupIcons(s)) : icon(modelOf(id)?.icon || offOf(id)?.icon || "generic"); };
  // models of one provider that would read alike (several "DeepSeek-V4.1-Flash"
  // of a plugin's sources) are said with what tells them apart (app.js's
  // modelTells), worked out once for each list the groups' state brings
  let tellsFor = null, tellsOf = null;
  const tellOf = (m) => {
    if (!m) return "";
    if (tellsFor !== groups?.models) { tellsFor = groups?.models; tellsOf = modelTells(tellsFor || [], (x) => x.name || x.id, (x) => x.id, (x) => x.provider || x.providerName || ""); }
    return tellsOf.get(m.id) || "";
  };
  const modelName = (m) => { const tell = tellOf(m); return (m.name || m.id) + (tell ? " · " + tell : ""); };
  const memberName = (id) => { const s = subOf(id), m = modelOf(id), o = !m && offOf(id); return s ? s.name : m ? modelName(m) : o ? o.model : id; };
  const memberNote = (id) => { const o = !subOf(id) && !modelOf(id) && offOf(id); return subOf(id) ? t("routing group") : [modelOf(id)?.providerName || o && `${o.name} · ${t("switched off")}`, drawerOf(id) && t("Image model"), fixedOf(id) && fixedWords(fixedOf(id))].filter(Boolean).join(" · "); };
  // a member the group sends in its vendor's fast mode (Group.Fast)
  const fastIn = (g, id) => !!g?.fast?.includes(id) && !subOf(id);
  // a member Free first and Cheapest first count as costing nothing: its
  // provider is Free for me, or its vendor serves it free (memberJSON.Free),
  // said whatever the group's routing, so it shows before Free first is
  // picked (jiabubu on Discord)
  const freeIn = (g, id) => !!g?.memberInfo?.find((x) => x.id === id)?.free;
  function memberLabel(g, id) {
    const i = g.memberInfo?.find((x) => x.id === id), m = modelOf(id), s = subOf(id), f = fixedOf(id);
    if (s) return `${t("routing group")} · ${s.name}`;
    const at = (f ? ` · ${fixedWords(f)}` : "") + (fastIn(g, id) ? ` · ${t("fast")}` : "") + (freeIn(g, id) ? ` · ${t("free")}` : "");
    if (m) return `${m.providerName} · ${modelName(m)}${at}`;
    if (i?.providerOff) return `${i.name} · ${i.model}${at} (${t("switched off")})`;
    return i?.name ? `${i.name} · ${i.model}${at}` : id;
  }
  // steady coalesces the redraws: one a frame, the latest wins
  let steadyRaf = 0;
  const steady = (fn) => { if (!steadyRaf) steadyRaf = requestAnimationFrame(() => { steadyRaf = 0; fn(); }); };
  function renderGroups() {
    if (!groups) return;
    // not under a row being dragged: it is drawn when let go
    if (groupArranging) { groupRenderPending = true; return; }
    steady(drawGroups);
  }
  // rowOrFlag: a group's row, or when drawing it fails, a row that says so
  // with its name, while every other group is drawn as ever (#1585)
  function rowOrFlag(g, draw) {
    try { return draw(); } catch (e) {
      console.error("routing group", g.id, e);
      const row = el("div", "rt-group off rt-gbad");
      row.dataset.id = g.id;
      const main = el("div", "main"), nm = el("div", "nm");
      nm.append(el("b", "", String(g.name || g.id)), el("code", "mdl", g.ref || "group/" + g.id));
      main.append(nm, el("div", "mem", t("This group can't be shown: {error}", { error: e?.message || String(e) })));
      row.append(el("span", "ics"), main);
      return row;
    }
  }
  // the groups' rows say what is left of their accounts (groupQuota):
  // drawn again when the Usage page's readings are new, unless a group is
  // under the reader's hands
  let groupsQuotas;
  function groupsReadAgain() {
    if (groups && typeof quotas !== "undefined" && quotas !== groupsQuotas && !gEdit && !gsec.contains(document.activeElement)) renderGroups();
  }
  document.addEventListener("magpie-costs-changed", groupsReadAgain);
  new MutationObserver(renderGroups).observe(document.documentElement, { attributes: true, attributeFilter: ["lang", "class"] });
  // groupQuota: a line a provider of the group's members, what is left of
  // the accounts the group can send it through, together (CryptoSarom on
  // X): "WorkBuddy AI, 3 accounts: Credits 1,145 / 1,500 credits · 76%
  // left". From the Usage page's readings (mergeQuotas), nothing read
  // for it; a member that is a group, or whose provider is off, has none.
  // Each account's own is on the Usage page.
  function groupQuota(g) {
    if (!Array.isArray(quotas)) return [];
    const shown = shownMembers(g), seen = new Set(), lines = [];
    for (const i of g.memberInfo || []) {
      if (!shown.includes(i.id) || i.group || i.providerOff || !i.provider || seen.has(i.provider)) continue;
      seen.add(i.provider);
      const subs = quotas.filter((q) => !q.from && q.provider === i.provider && (q.error || q.windows?.length));
      if (!subs.length) continue;
      // mergeQuotas is quotaall.js's: once it is in, the groups are drawn again
      if (!window.mergeQuotas) { quotaAllJS().then(() => { groupsQuotas = null; }); return []; }
      const all = mergeQuotas(subs);
      const ws = (familyWindows(all.windows) || []).slice(0, 3);
      if (!ws.length) continue;
      const figs = ws.map((w) => t(w.name) + (w.plan ? " · " + w.plan : "") + " " + quotaText(w)).join(" · ");
      const text = t("{name}, {n} accounts: {windows}", { name: i.name || i.provider, n: subs.length, windows: figs });
      const line = el("div", "mem rt-gquota", all.unknown ? text + " · " + t("{n} couldn't be read", { n: all.unknown }) : text);
      line.dataset.provider = i.provider;
      line.title = t("What is left of this provider's accounts the group can use, together: amounts of one unit added up, a share the mean of the accounts on one plan. Each account is on the Usage page.");
      lines.push(line);
    }
    return lines;
  }
  function drawGroups() {
    groupsQuotas = typeof quotas === "undefined" ? undefined : quotas;
    // a sidebar tab draws one group's editor alone (app.js's routeTabs):
    // the list, the hidden groups and New group are the groups page's
    if (single) return drawOne();
    const all = groups.groups.filter((g) => !g.hidden), hidden = groups.groups.filter((g) => g.hidden);
    // the group being changed stays, whatever the filter
    const shown = all;
    const newBtn = el("button", "text rt-gnew");
    newBtn.type = "button";
    newBtn.append(svg(PLUS, 11, 1.8), el("span", "", t("New group")));
    // one kind of group now: it opens on the doc's defaults — quickest
    // first, three asked at once, eight streams open, 30s to give up — and
    // the order is a pick inside the editor (the doc's §5.2)
    newBtn.onclick = (ev) => newGroup(ev);
    const head = [el("span", "label", t("Routing groups")), el("span", "grow"), el("span", "note", t("models agents pick as one"))];
    head.push(newBtn);
    // the filter stays in the page as the rest of the row is drawn again,
    // so the key being typed into it lands once (#1055)
    replaceKeeping(gHead, head);
    const rows = [];
    if (gEdit && !gEdit.id) rows.push(groupEditor(null));
    for (const g of shown) rows.push(rowOrFlag(g, () => gEdit?.id === g.id ? groupEditor(g) : groupRow(g)));
        if (!rows.length) rows.push(el("div", "none rt-gnone", t("No group yet. New group makes one of any models you like.")));
    // a found group removed stays removed and magpie makes none again:
    // since 10-17 it makes none at all, so only one still saved can be
    // brought back
    if (hidden.length) {
      const h = el("div", "rt-ghidden");
      const b = el("button", "text rt-gremoved");
      b.type = "button";
      b.setAttribute("aria-haspopup", "menu");
      b.append(el("span", "", t(hidden.length === 1 ? "1 found group removed" : "{n} found groups removed", { n: hidden.length })), svg(CHEV, 11, 1.6));
      b.title = t("magpie doesn't make them again. Click to bring one back.");
      b.onclick = (e) => {
        e.stopPropagation();
        const again = agentMenu?.anchor === b;
        closeAgentMenu();
        if (again) return;
        openRowMenu(b, hidden.map((g) => ({ name: g.id.replace(/^auto-/, ""), icon: REAPPLY, tip: t("Bring it back"),
          run: () => groupAction("show", { id: g.id }, t("{name} is back", { name: g.id })) })));
      };
      h.append(b);
      rows.push(h);
    }
    // the import box's text stays in the page as the list is drawn again
    const was = slotsOf();
    replaceKeeping(gList, rows);
    morphSlots(was);
  }
  // drawOne is a tab's whole page: the group's editor, open, with no list
  // around it. A tab of a group that is gone says so; a tab whose group
  // hasn't come yet waits.
  // tabId is the group this tab draws: the one it was opened for, or the
  // one its own editor just saved (a new tab's group gets an id then, and
  // the tab is that group's from then on)
  let tabId = only;
  function drawOne() {
    const g = tabId ? groups.groups.find((x) => x.id === tabId) : null;
    if (tabId && !g && (!gEdit || gEdit.id !== tabId)) {
      replaceKeeping(gList, [el("div", "none rt-gnone", t("This group is gone. Close the tab."))]);
      return;
    }
    // a tab whose group was just saved keeps the editor it has: the draft's
    // id is the saved group's (groupAction sets it), so it isn't rebuilt
    // under the reader's hands
    if (g && gEdit?.id === g.id) { replaceKeeping(gList, [groupEditor(g)]); opts.onGroups?.(groups); return; }
    if (!gEdit || gEdit.id !== (g ? g.id : "")) {
      if (g) {
        gEdit = { id: g.id, draft: { name: g.name, members: [...(g.members?.length ? g.members : planEntryRefsOf(g))], match: [...(g.match || [])], matched: [...(g.matched || [])], fast: [...(g.fast || [])], off: [...(g.off || [])], weights: { ...(g.weights || {}) }, unlists: !!g.unlists, context: g.context || 0, classifier: g.classifier || "", effort: g.effort || "", levels: [...(g.levels || [])], rules: (g.rules || []).map((r) => ({ ...r, ...(r.more?.length ? { more: [...r.more] } : {}), intent: r.intent || "", agents: [...(r.agents || [])], folders: [...(r.folders || [])], exceptFolders: [...(r.exceptFolders || [])], time: r.time ? { ...r.time, days: [...(r.time.days || [])] } : null })), race: planRaceOf(g) || { orderBy: "speed", seats: RACE_SEATS, maxInFlight: RACE_MAX, totalWait: 30, sticky: true }, fts: planFtsOf(g) } };
        gEdit.was = JSON.stringify(gEdit.draft);
      } else {
        gEdit = { id: "", draft: { name: "", members: [], match: [], matched: [], fast: [], off: [], routing: "", affinity: "", rules: [], race: { orderBy: "speed", seats: RACE_SEATS, maxInFlight: RACE_MAX, totalWait: 30, sticky: true }, fts: {} } };
      }
    }
    const rows = [];
    if (!rows.length) rows.push(rowOrFlag(g || { id: "" }, () => groupEditor(g)));
    replaceKeeping(gList, rows);
    opts.onGroups?.(groups);
  }
  // A group's row opens into its editor and folds back into it, as a cell
  // does on iOS (the owner, 10-23): its slot goes from the height it had to
  // its new one, on a spring as it opens and on iOS's ease as it folds, the
  // list below moving with it, and what is in it comes in as it does. A
  // group saved shows it with a check by its name.
  // gMorph: the editor's slot to the group's row after a Save, which may
  // name the group anew (a new group, an id changed); gSaved: the group just
  // saved
  // gFolded: the slot whose fold keeps itself in sight, as groupAction
  // would after a Save
  let gMorph = null, gSaved = null, gFolded = null;
  const slotKey = (n) => n.dataset?.gslot;
  function slotsOf() {
    const m = new Map(), vt = gsec.closest(".view").getBoundingClientRect().top;
    for (const n of gList.children) {
      const k = slotKey(n);
      if (k === undefined) continue;
      const b = n.getBoundingClientRect();
      m.set(k, { n, h: b.height, top: b.top - vt, ed: n.classList.contains("rt-gedit") });
    }
    return m;
  }
  function morphSlots(was) {
    const alias = gMorph, saved = gSaved;
    gMorph = gSaved = gFolded = null;
    if (still() || !shownG()) return;
    for (const n of gList.children) {
      const k = slotKey(n);
      if (k === undefined) continue;
      const ed = n.classList.contains("rt-gedit");
      const old = was.get(k) || (alias && alias[1] === k ? was.get(alias[0]) : undefined);
      if (old?.n === n) continue;
      // a new group's editor grows out of nothing at the list's top
      const from = old ? (old.ed === ed ? null : old.h) : ed && k === "" ? 0 : null;
      if (from != null) slotMorph(n, from, ed, old?.top);
      if (!ed && saved === k) savedMark(n);
    }
  }
  function slotMorph(n, from, opening, top) {
    const to = n.getBoundingClientRect().height;
    if (Math.abs(to - from) < 1) return false;
    const shape = opening ? { duration: 570, easing: SPRING } : { duration: 420, easing: IOS_EASE };
    n.style.overflow = "hidden";
    const grow = n.animate([{ height: from + "px" }, { height: to + "px" }], shape);
    grow.finished.then(() => { n.style.overflow = ""; }, () => { n.style.overflow = ""; });
    if (!opening && top < 0) foldInSight(n, grow, from, to, top);
    if (opening) {
      // its parts come in from just above, one after another; those past
      // the first dozen together
      let i = 0;
      for (const c of n.children) {
        c.animate([{ opacity: 0, transform: "translateY(-6px)" }, { opacity: 1, transform: "none" }],
          { duration: 420, delay: 40 + Math.min(i++, 12) * 18, easing: IOS_EASE, fill: "backwards" });
      }
    } else {
      for (const c of n.children) c.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 300, delay: 90, easing: "ease-out", fill: "backwards" });
    }
  }
  // foldInSight: an editor folded from its foot (Save and Cancel are there)
  // would leave the page shorter under the reader, and the browser's clamp
  // would land them at the list's foot with the row above the window. The
  // page goes up with the fold instead, step for step with the slot's
  // height, until the row's top is the window's — as the reader's own
  // scroll, since it answers their click. top is how far the editor's top
  // was above the window's; the page's scroll is read once the redraw
  // has put it back (steady).
  function foldInSight(n, grow, from, to, top) {
    const view = document.getElementById("view-gateway");
    readerScrolls(1000);
    let s0, target;
    const step = () => {
      if (s0 === undefined) { s0 = view.scrollTop; target = s0 + top; }
      if (!n.isConnected) return;
      const p = grow.playState === "finished" ? 1 : Math.min(1, Math.max(0, (from - n.getBoundingClientRect().height) / (from - to)));
      view.scrollTop = s0 + (target - s0) * p;
      if (p < 1 && grow.playState === "running") requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
    gFolded = n.dataset.gslot;
  }
  // savedMark: the row of the group just saved lights up and a check pops
  // in by its name, then both go
  function savedMark(row) {
    row.animate([{ backgroundColor: "color-mix(in srgb, var(--accent) 16%, transparent)" }, { backgroundColor: "color-mix(in srgb, var(--accent) 16%, transparent)", offset: 0.35 }, { backgroundColor: "transparent" }],
      { duration: 1400, easing: "ease-out" });
    const nm = row.querySelector(".nm");
    if (!nm) return;
    const ok = el("span", "rt-gsaved");
    ok.setAttribute("aria-hidden", "true");
    ok.append(svg(CHECK, 11, 2));
    nm.append(ok);
    ok.animate([{ opacity: 0, transform: "scale(.4)" }, { opacity: 1, transform: "none" }], { duration: 570, easing: SPRING });
    const path = ok.querySelector("path");
    const len = path.getTotalLength?.() || 14;
    path.animate([{ strokeDasharray: len, strokeDashoffset: len }, { strokeDasharray: len, strokeDashoffset: 0 }], { duration: 360, delay: 120, easing: IOS_EASE, fill: "backwards" });
    ok.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 400, delay: 1500, easing: "ease-in", fill: "forwards" })
      .finished.then(() => ok.remove(), () => ok.remove());
  }
  function groupRow(g) {
    const row = el("div", "rt-group" + (g.ready ? "" : " off") + (g.disabled ? " disabled" : ""));
    row.dataset.id = row.dataset.gslot = g.id;
    const ics = groupHandle(g, row);
    const main = el("div", "main");
    const nm = el("div", "nm");
    nm.append(el("b", "", g.name), el("code", "mdl", g.ref || "group/" + g.id));
    if (g.auto) nm.append(el("small", "auto", t("found by magpie")));
    const mem = el("div", "mem", shownMembers(g).map((id) => memberLabel(g, id) + (g.off?.includes(id) ? ` (${t("off")})` : "")).join(" · "));
    main.append(nm, mem);
    if (!g.decides) main.append(...groupQuota(g));
    for (const p of g.patterns || []) {
      const line = el("div", "mem pat" + (p.models ? "" : " none"));
      line.append(el("code", "", p.pattern), document.createTextNode(" · " + patternWords(p.models)));
      line.title = p.models ? t("Every model this pattern matches is in the group, as providers list them") : t("No model magpie serves matches this pattern now");
      main.append(line);
    }
    // its classifier couldn't say the last time it was asked (#1929,
    // versun): said here, not only in each request's details, since its
    // rules with an intent match nothing until it answers
    const cf = g.classifierFailed;
    if (cf && typeof cf.error === "string" && cf.error) {
      const line = el("div", "mem clsfail");
      line.append(el("i", "dot"), el("span", "", t("Classifier failed {when}: {error}", { when: ago(cf.at), error: cf.error })));
      line.title = t("{by} couldn't say which intent or effort a turn is the last time it was asked: {error}. Until it answers, no rule with an intent matches and turns reason as the agent asked. Each request's details say what it was asked.", { by: g.classifier || t("the classifier"), error: cf.error });
      main.append(line);
    }
    const tags = el("span", "tags");
    // one executor: the row says the order its entries are tried in and how
    // many are asked at once (the doc's §5.1) — the old strategy, staying
    // and rate-limited tags described settings nothing reads any more
    const race = planRaceOf(g);
    if (race) {
      const list = race.orderBy === "list";
      const o = el("span", "tag", t(list ? "In order" : "Race"));
      o.title = t(list ? "Tries its entries in the list's order, the first until it can't answer, then the next."
        : "Asks several entries at once and keeps the one that answers first; the quickest is asked again first next time.");
      tags.append(o, el("span", "tag", t("{n} concurrent", { n: race.seats })));
    }
    if (g.unlists) {
      const k = el("span", "tag unlists", t("Its models unlisted"));
      k.title = t(LIST_HINT.unlists);
      tags.append(k);
    }
    if (g.rules?.length) {
      const r = el("span", "tag", t(g.rules.length === 1 ? "1 rule" : "{n} rules", { n: g.rules.length }));
      r.title = g.rules.map((x, i) => `${i + 1}. ${ruleText(x)} → ${x.pause ? t("pause") + " " : ""}${rUses(x).map((u) => memberLabel(g, u)).join(", ")}${x.useEffort ? " · " + t("{level} reasoning", { level: x.useEffort }) : ""}`).join("\n");
      tags.append(r);
    }
    const offs = offIn(g);
    if (offs.length && shownMembers(g).length < g.members.length) {
      const k = el("span", "tag idle rt-goff", t("{n} switched off", { n: offs.length }));
      k.title = offs.map(offWords).join("\n");
      tags.append(k);
    }
    if (g.disabled) tags.append(el("span", "tag idle", t("switched off")));
    else if (!g.ready) { const n = (g.plan?.members || []).filter((m) => m.kind === "entry").length; tags.append(el("span", "tag bad", n ? t("{n} entries", { n }) : t("no member ready"))); }
    const edit = el("button", "text", t("Edit"));
    edit.onclick = (e) => { e.stopPropagation(); open(); };
    const open = () => openGroupEditor(g);
    row.onclick = open;
    row.oncontextmenu = (e) => { e.preventDefault(); groupMenu(ics, g); };
    row.append(ics, main, tags, edit);
    // a group of the user's switches off as a whole, kept as it is (PAMI
    // on Discord); one magpie found is removed rather (provider.SwitchGroup)
    if (!g.auto) {
      const sw = el("button", "lib-switch rt-gon" + (g.disabled ? "" : " on"));
      sw.type = "button";
      sw.setAttribute("role", "switch");
      sw.setAttribute("aria-checked", String(!g.disabled));
      sw.setAttribute("aria-label", g.name);
      sw.title = g.disabled ? t("Off: agents aren't offered {name} and a request to it is turned away. Click to switch it on", { name: g.name })
        : t("On: agents may pick {name}. Click to switch it off and keep it as it is", { name: g.name });
      sw.append(el("i"));
      sw.onclick = async (e) => {
        e.stopPropagation(); // the card opens the editor; this switches
        const held = document.activeElement === sw;
        await groupAction("switch", { id: g.id, on: !!g.disabled }, t(g.disabled ? "{name} switched on" : "{name} switched off", { name: g.name }));
        if (held) gList.querySelector(`.rt-group[data-id="${CSS.escape(g.id)}"] .rt-gon`)?.focus({ preventScroll: true });
      };
      row.append(sw);
    }
    return row;
  }
  // groupHandle is a group row's logos, which are also its handle, as an
  // agent row's logo is (#779): drag it to move the row, click it (or
  // right-click the row) for Move up and Move down, and Alt+↑/↓ moves it
  // from the keyboard. The grip that says so is drawn in the row's margin
  // only on hover (routing.css), as on the Agents page. The order is the
  // one /v1/models lists the groups in too, which agents' pickers show.
  let groupArranging = false, groupRenderPending = false, groupArrangeSeq = 0;
  // a drag let go: the groups drawn as they came in under it
  const arranged = () => {
    groupArranging = false;
    if (groupRenderPending) { groupRenderPending = false; renderGroups(); }
  };
  const shownGroups = () => groups.groups.filter((g) => !g.hidden).map((g) => g.id);
  function groupHandle(g, row) {
    const b = el("button", "ics ag-handle rt-ghandle");
    b.type = "button";
    b.setAttribute("aria-label", t("Arrange {agent}", { agent: g.name }));
    b.setAttribute("aria-haspopup", "menu");
    b.title = t("Drag to reorder — agents' model lists show the groups in this order · Alt+↑/↓ to move");
    b.append(stackIcon(groupIcons(g)));
    b.onkeydown = (e) => {
      if (!e.altKey || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) return;
      e.preventDefault();
      e.stopPropagation();
      moveGroup(g.id, nextTo(g.id, e.key === "ArrowUp" ? -1 : 1), true);
    };
    // the card opens the editor; the handle its menu, but not for the
    // click that ends a drag
    b.onclick = (e) => { e.stopPropagation(); if (!b.dataset.dragged) groupMenu(b, g); delete b.dataset.dragged; };
    b.onpointerdown = (e) => {
      if (groupArranging) return;
      // an editor open in the list is no row to move past: the rows
      // dragged among are the cards, and where one is let go is named by
      // the card it lands on
      const rows = [...gList.children].filter((r) => r.classList.contains("rt-group"));
      groupArranging = dragRows(e, b, row, gList, rows, (to) => moveGroup(g.id, shownGroups().indexOf(rows[to].dataset.id)), closeAgentMenu, arranged);
    };
    return b;
  }
  function groupMenu(anchor, g) {
    const again = agentMenu?.anchor === anchor;
    closeAgentMenu();
    if (again) return;
    const up = nextTo(g.id, -1), down = nextTo(g.id, 1);
    openRowMenu(anchor, [
      { name: "Move up", icon: MOVE_UP, key: ALT + "↑", off: up < 0, run: () => moveGroup(g.id, up, true) },
      { name: "Move down", icon: MOVE_DOWN, key: ALT + "↓", off: down < 0, run: () => moveGroup(g.id, down, true) },
      // the group as it is, models, routing and rules, listed after it, to
      // change without touching the one agents use (lc on Discord)
      { name: "Duplicate", icon: COPY_ICON, sep: true, tip: t("A copy of {name} with its models, routing and rules, listed after it", { name: g.name }),
        run: async () => { if (!groupDirty() || await confirmDiscard()) groupAction("copy", { id: g.id, name: t("{name} copy", { name: g.name }) }, t("Duplicated {name}", { name: g.name })); } },
      // the group as text for someone else's magpie (laincatx on X)
    ]);
  }
  // nextTo: where a group moved one up (-1) or down goes, among them all:
  // the place of the card listed next to it, so a filtered list moves it
  // past the one the reader sees; -1 at either end
  function nextTo(id, by) {
    const seen = [...gList.querySelectorAll(".rt-group[data-id]")].map((r) => r.dataset.id);
    const n = seen[seen.indexOf(id) + by];
    return n === undefined || !seen.includes(id) ? -1 : shownGroups().indexOf(n);
  }
  // moveGroup puts the group at index `to` among the ones listed; the
  // removed ones keep their places after them. The list is drawn in its new
  // order at once and put back if the save fails.
  async function moveGroup(id, to, keep) {
    const prev = groups;
    const ids = shownGroups();
    const from = ids.indexOf(id);
    if (from < 0 || to < 0 || to >= ids.length || to === from) return;
    ids.splice(to, 0, ...ids.splice(from, 1));
    const order = [...ids, ...prev.groups.filter((g) => g.hidden).map((g) => g.id)];
    const byId = new Map(prev.groups.map((g) => [g.id, g]));
    const handle = () => gList.querySelector(`.rt-group[data-id="${CSS.escape(id)}"] .rt-ghandle`);
    groups = { ...prev, groups: order.map((x) => byId.get(x)) };
    renderGroups();
    // the rows were drawn anew: keep the keyboard on this one
    if (keep) handle()?.focus({ preventScroll: true });
    const seq = ++groupArrangeSeq;
    let next;
    try {
      next = groupsFrom(await api("groups/arrange", { order }));
      status(t("Group order saved"), "ok");
    } catch (e) {
      next = prev;
      status(e.message, "err");
    }
    // a later move, made while this one was saved, has the last word
    if (seq !== groupArrangeSeq) return;
    const held = document.activeElement === handle();
    groups = next;
    renderGroups();
    if (held) handle()?.focus({ preventScroll: true });
    load(); // the gateway's model list, the agents' pickers
  }
  function groupEditor(g) {
    const d = gEdit.draft;
    // whoever opened it, a draft has what the editor and Add read: a group
    // made from a model (newGroupWith) had no fast, and Add threw on it
    // and did nothing (悠悠哥 on Discord)
    for (const k of ["members", "match", "matched", "fast", "off", "rules", "accounts"]) if (!Array.isArray(d[k])) d[k] = [];
    if (!d.weights || typeof d.weights !== "object") d.weights = {};
    // one executor: every draft has a plan, whichever way it was opened —
    // a template's had none, and the editor read d.race.seats on it
    if (!d.race) d.race = { orderBy: "speed", seats: RACE_SEATS, maxInFlight: RACE_MAX, totalWait: 30, sticky: true };
    if (!d.fts || typeof d.fts !== "object") d.fts = {};
    const ed = el("div", "editor rt-gedit");
    ed.dataset.gslot = g ? g.id : "";
    const h = el("div", "ehead");
    h.append(el("b", "", g ? g.name : t("New group")));
    if (g?.auto) h.append(el("span", "note", t("found by magpie — saving a change makes it yours")));
    ed.append(h);
    const keys = (i) => { i.onkeydown = (e) => { e.stopPropagation(); if (e.key === "Escape") { cancelGroup(); } else if (e.key === "Enter" && i === name) saveBtn.onclick(); }; return i; };
    const name = keys(input(d.name, t("e.g. Opus anywhere")));
    const idHint = el("div", "hint");
    // an existing group's id can change (an auto- one found by magpie too);
    // a new one's is made from its name
    if (g && d.id === undefined) d.id = g.id;
    // an open group folds back into its row from its heading, as it opened
    // from the row (ARNO on Discord) — only while nothing in it has
    // changed, so a stray click never throws an edit away; Cancel does that.
    // Its own class, not .fold: that is the Models head's chip, whose
    // padding and corners made the heading's line curve at its ends and
    // its space above differ from below (the owner, 10-21)
    if (g) {
      h.classList.add("folds");
      h.setAttribute("role", "button");
      h.tabIndex = 0;
      h.setAttribute("aria-expanded", "true");
      h.title = t("Click to fold it");
      const fold = () => {
        if (JSON.stringify(d) !== gEdit.was) return status(t("Save or Cancel your changes first"), "warn");
        gEdit = null;
        renderGroups();
      };
      h.onclick = fold;
      h.onkeydown = (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); fold(); } };
    }
    const idIn = g ? keys(input(d.id, g.id)) : null;
    const idOf = () => {
      if (g) return groupSlug(d.id) || g.id;
      // a template's draft: its own id when the name makes none (a name
      // in Chinese)
      let id = groupSlug(d.name) || d.slug || "group", n = 1;
      const base = id;
      while (groups.groups.some((x) => x.id === id)) id = `${base}-${++n}`;
      return id;
    };
    const showId = () => {
      idHint.textContent = t("Agents pick it as {id}", { id: g?.ref && idOf() === g.id ? g.ref : "group/" + idOf() }) +
        (g && idOf() !== g.id ? " · " + t("an agent set to {id} needs setting again", { id: g.ref || "group/" + g.id }) : "");
    };
    // what the agents' lists will call it (#868: an agent's narrow menu
    // the name is the name: a group's label in agents' lists is its name
    // now, no "· routing group" suffix and no setting for it (the doc's
    // §4.4 and Task 7)
    name.oninput = () => { d.name = name.value; showId(); };
    const nw = el("div");
    nw.append(name);
    if (!g) nw.append(idHint);
    ed.append(el("label", "", t("Name")), nw);
    if (idIn) {
      idIn.oninput = () => { d.id = idIn.value; showId(); };
      idIn.onblur = () => { d.id = idIn.value = idOf(); showId(); };
      const iw = el("div");
      iw.append(idIn, idHint);
      ed.append(el("label", "", t("ID")), iw);
    }
    showId();

    // members, in order: the first is what an agent is told the model can do.
    // More are picked with the model picker the agents use.
    const infoOf = (id) => g?.memberInfo?.find((x) => x.id === id);
    const pickFrom = (label, anchor, ev, options, onPick, value = "") => openPicker({ id: "", name: "", fields: [] }, { key: "rule", label, value, menu: true, options, onPick }, anchor, ev);
    // the agents magpie heard from in the last 30 days that it doesn't
    // know, by the names the gateway gave them (usage.AgentOf), asked for
    // once while the editor is open; none when usage can't be read
    let heardFrom = null;
    const heard = () => heardFrom || (heardFrom = api("usage?period=30d")
      .then((u) => (u?.agents || []).map((a) => uaName(a.id)).filter((id) => id && id !== "other" && id !== "magpie"))
      .catch(() => []));
    const box = el("div", "fallback");
    const list = el("div", "fbl rt-etable");
    // the entry table's header: switch / # / provider / model / credential /
    // what it answered last / its own first-token timeout. The five name
    // cells share a row's own wrapper, so they sit where their values do.
    const ehead = el("div", "fbrow rt-ehead");
    ehead.append(el("i"), el("i"), el("i"),
      el("span", "ec-p", t("Provider")), el("span", "ec-m", t("Model")), el("span", "ec-c", t("Credential")),
      el("span", "ec-ttft", t("Last first token")), el("span", "ec-tps", t("Last speed")),
      el("span", "ec-x"), el("span", "ec-x", t("Reasoning")), el("span", "ec-x"),
      el("span", "ec-ft", t("First-token")), el("i"), el("i"), el("i"));
    list.append(ehead);
    const addBtn = el("button", "rt-gadd");
    addBtn.append(svg(PLUS, 11, 1.8), el("span", "", t("Add an entry")));
    // moveMember puts the member at `from` at `to`, a pattern's as well as
    // one named (usrname1833 on Discord: provider.Group.Placed), and keeps
    // the keyboard on its handle when it was there
    const moveMember = (from, to, keep) => {
      if (to < 0 || to >= d.members.length || to === from) return;
      d.members.splice(to, 0, d.members.splice(from, 1)[0]);
      draw();
      if (keep) list.querySelector(`.rt-mhandle[data-at="${to}"]`)?.focus({ preventScroll: true });
    };
    // memberHandle is a member's place in the order, which is also
    // its handle (PAMI on Discord: only Up, one step a click): drag it to
    // move the model, or Alt+↑/↓ from the keyboard; Up and Down stay
    // beside it. The grip that says so shows only on hover, as on the
    // groups' cards.
    const memberHandle = (id, i, row) => {
      const b = el("button", "i ag-handle rt-mhandle", String(i + 1));
      b.type = "button";
      b.dataset.at = i;
      b.setAttribute("aria-label", t("Move {name}", { name: memberName(id) }));
      b.title = t("Drag to reorder · Alt+↑/↓ to move");
      b.onkeydown = (e) => {
        if (!e.altKey || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) return;
        e.preventDefault();
        e.stopPropagation();
        moveMember(i, i + (e.key === "ArrowUp" ? -1 : 1), true);
      };
      b.onclick = (e) => { e.stopPropagation(); delete b.dataset.dragged; };
      b.onpointerdown = (e) => {
        // the groups aren't drawn again under it, the editor with them (yuancjun)
        if (groupArranging) return;
        const rows = [...list.children].filter((r) => r.querySelector(".rt-mhandle"));
        groupArranging = dragRows(e, b, row, list, rows, (to) => moveMember(i, to), () => {}, arranged);
      };
      return b;
    };
    let syncRace = () => {};
    const draw = () => {
      syncRace();
      // redrawn whole, the header first: replaceChildren would else drop
      // the column labels the rows are read against
      list.replaceChildren(ehead);
      d.members.forEach((id, i) => {
        // an entry row "provider/model@cred": the model is looked up by
        // its ref, the credential shown apart
        const at = id.lastIndexOf("@");
        const ref = at > 0 && !id.startsWith("group/") ? id.slice(0, at) : id;
        const cred = at > 0 && !id.startsWith("group/") ? id.slice(at + 1) : "";
        const m = modelOf(ref), s = subOf(ref);
        // a member that is itself a group is in none of groups.models
        // (groupsState leaves groups out of it), so it is read from its own
        // memberInfo instead: without this its row carried no chips at all,
        // and a group inside a group said nothing of what it takes
        const info = infoOf(ref);
        const chips = m || (s ? info : undefined);
        const row = el("div", "fbrow");
        // the entry table's columns: provider / model / credential, then
        // what it answered last and the timeout it races under — one grid
        // the header shares, so the columns line up; a cell a row has
        // nothing for is a hidden placeholder, which keeps its column
        const ghost = () => el("span", "ghost");
        const pcell = el("span", "ec-p", m ? m.providerName || ref.split("/")[0] : (info?.name || ref.split("/")[0]));
        pcell.title = pcell.textContent;
        const nm = el("span", "ec-m", s ? s.name : m ? (m.name || m.id) : ref.split("/")[1] || ref);
        nm.title = nm.textContent;
        const ccell = el("span", "ec-c", cred || t("any credential"));
        ccell.title = ccell.textContent;
        // what this entry answered last, from the gateway's own memory:
        // the race tries the quicker ones first, and says so here rather
        // than leaving the order to be found out by waiting.
        const sp = speedOfEntry(id);
        const ttftCell = el("span", "ec-ttft", firstWords(sp?.lastTtft));
        const tpsCell = el("span", "ec-tps", speedWords(sp?.lastTps));
        const spNote = sp?.lastTtft ? t("Its last answer: the gateway times each one, and forgets them on a restart") : t("Nothing measured from it yet: the race tries it after the ones it has timed");
        ttftCell.title = spNote;
        tpsCell.title = spNote;
        if (!sp?.lastTtft) { ttftCell.classList.add("none"); tpsCell.classList.add("none"); }
        // badges share the row's tail slot: several of them in one flex
        // cell keeps the entry on one line (a grid would put each on its
        // own implicit column)
        const tags = el("span", "ec-x");
        if (s || drawerOf(ref)) tags.append(el("small", "", subOf(ref) || memberNote(ref)));
        else if (info?.providerOff) tags.append(el("small", "", `${info.name} · ${t("switched off")}`));
        if (info?.free && !s) {
          const fr = el("small", "rt-free", t("free"));
          fr.title = t("Free first uses it before paid ones, and Cheapest first counts it as costing nothing");
          tags.append(fr);
        }
        const matched = d.matched.includes(id);
        if (matched) tags.append(el("small", "", t("by pattern")));
        // switched off, it keeps its place and its rules but is sent
        // nothing: trying the group without it takes no removing and
        // adding back (Group.Off)
        const off = d.off.includes(id);
        const sw = el("button", "lib-switch rt-mon" + (off ? "" : " on"));
        sw.type = "button";
        sw.setAttribute("role", "switch");
        sw.setAttribute("aria-checked", String(!off));
        sw.setAttribute("aria-label", memberName(ref));
        sw.title = off ? t("Off: kept in its place, sent nothing. Click to switch it on") : t("On: requests may go to it. Click to switch it off and keep its place");
        sw.append(el("i"));
        sw.onclick = () => { d.off = off ? d.off.filter((x) => x !== id) : [...d.off, id]; draw(); };
        if (off) row.classList.add("muted");
        row.append(sw, memberHandle(id, i, row), memberIcon(ref), pcell, nm, ccell, ttftCell, tpsCell, tags);
        // what the model is, in the same chips the Gateway page's list says it
        // in (modelInfo): its reasoning levels, whether it sees images and the
        // window it holds. A group mixing members that see images with ones
        // that don't had nothing here to tell one from another, and which
        // member a picture would reach was only found out by sending one.
        // A member whose list says nothing of images is marked unknown rather
        // than text-only: magpie counts it text-only for a describer
        // (gateway.blindTo), which is not the same as its list saying so.
        if (chips) tags.append(modelInfo({ ...chips, group: !!s, images: info?.images ?? chips.images, imagesUnknown: info?.imagesUnknown ?? chips.imagesUnknown }));
        // the reasoning the model is sent at in this group: the group's
        // (blank), or one of its own whatever the agent asks. A group in
        // it reasons as it says.
        // a decision model reasons at no level of the group's or its own
        if (!s && !deciderOf(ref) && !drawerOf(ref)) {
          const [base, fixed] = splitMember(ref);
          const several = fixed.includes("+");
          const fx = el("button", "rt-cond rt-fixed" + (fixed ? " on" : ""), fixed ? fixedWords(fixed) : t("Group's reasoning"));
          fx.title = several ? t("The agent picks among {levels} reasoning, each sent at the model's nearest level", { levels: fixed.split("+").join(" / ") })
            : fixed ? t("Sent at {level} reasoning whatever the agent asks, at the model's nearest level", { level: fixed }) : t("Reasons as the group's effort says");
          const levels = m?.efforts?.length ? m.efforts.filter((v) => LEVELS.includes(v)) : LEVELS.filter((v) => v !== "none" && v !== "minimal");
          // the member under another id: its rules, fast, off, weight and
          // pick go with it
          const moveTo = (to) => {
            const from = d.members[i];
            if (to === from) return true;
            if (d.members.includes(to)) { status(t("{name} at that reasoning is in the group already", { name: memberName(from) }), "err"); return false; }
            d.members[i] = to;
            d.matched = d.matched.filter((x) => x !== from); // named now, at its own reasoning
            for (const r of d.rules) setUses(r, rUses(r).map((x) => x === from ? to : x));
            d.fast = d.fast.map((x) => x === from ? to : x);
            d.off = d.off.map((x) => x === from ? to : x);
            if (d.weights[from]) { d.weights[to] = d.weights[from]; delete d.weights[from]; }
            return true;
          };
          // several levels the agent picks among (provider.MemberLevels): a
          // menu that stays open, each level ticked or not; one left is a
          // fixed effort again, none follows the group
          const pickSeveral = (ev) => {
            let have = fixed ? fixed.split("+") : [];
            openPicker({ id: "", name: "", fields: [] }, { key: "rule", label: "reasoning", value: "", menu: true, multi: true,
              options: levels.map((v) => ({ value: v, label: v })), picked: (v) => have.includes(v), onPick: (v) => {
                const next = have.includes(v) ? have.filter((x) => x !== v) : LEVELS.filter((x) => x === v || have.includes(x));
                const at = next.join("+");
                if (!moveTo(at ? `${base}:${at}` : base)) return;
                have = next;
                draw();
              } }, fx, ev);
          };
          fx.onclick = (ev) => pickFrom("reasoning", fx, ev, [
            { value: "", label: t("Follow the group"), note: t("as the group's effort says") },
            ...levels.map((v) => ({ value: v, label: fixedWords(v), note: t("whatever the agent asks") })),
            ...(levels.length > 1 ? [{ value: "+", label: t("Several levels…"), note: t("the agent picks among them") }] : []),
          ], (v) => {
            if (v === "+") { pickSeveral(ev); return; }
            if (moveTo(v ? `${base}:${v}` : base)) { draw(); }
          }, several ? "+" : fixed);
          row.append(fx);
          // its vendor's fast mode, where the model has one: priority
          // processing, Claude's fast mode (provider.CanFast)
          if (m?.canFast) {
            const on = d.fast.includes(id);
            const fb = el("button", "rt-cond rt-fixed rt-fast" + (on ? " on" : ""), t(on ? "Fast" : "Standard speed"));
            fb.type = "button";
            fb.setAttribute("aria-pressed", String(on));
            // on a provider the user added (provider.OwnFast, #1615) it is
            // only asked for: magpie can't know the provider serves it
            fb.title = m.ownFast
              ? (on ? t("Asked for OpenAI's fast mode (service_tier \"priority\") whatever the agent asks: quicker, at a higher price, where your provider serves it. One that turns it away is asked without it") : t("Sent as the agent asks; click to ask your provider for OpenAI's fast mode (service_tier \"priority\")"))
              : (on ? t("Sent in its vendor's fast mode whatever the agent asks: quicker, at a higher price") : t("Sent at its vendor's usual speed; click to send it in fast mode"));
            fb.onclick = () => { d.fast = on ? d.fast.filter((x) => x !== id) : [...d.fast, id]; draw(); };
            row.append(fb);
          } else row.append(ghost());
        } else if (s) {
          // a group in the group takes no reasoning of its own: its models
          // reason as it says (provider.SaveGroup refuses "group/x:high", and
          // the gateway has nowhere to put one). So the slot a model's own
          // picker sits in says where it is set instead, and goes there.
          const hx = el("button", "rt-cond rt-fixed rt-inner");
          hx.type = "button";
          // in a span, so a long group's name ellipsizes instead of widening
          // the row: the desktop app's member row doesn't wrap
          hx.append(el("span", "", t("Follows {name}", { name: s.name })));
          hx.title = t("Its models reason as {name} says: set that up on {name}'s own card, not here", { name: s.name });
          hx.onclick = async (ev) => {
            if (!(await openGroupEditor(s))) return;
            const ed = gList.querySelector(".rt-gedit");
            if (ed && window.scrollOnPurpose?.(ev)) ed.scrollIntoView({ block: "center", behavior: "smooth" });
          };
          row.append(hx);
        }
        // its own first-token timeout: each stream is timed on its own,
        // in either order (the doc's §5.2)
        const fin = input(String((d.fts || {})[id] ?? ""), "10");
        fin.type = "number";
        fin.min = "1";
        fin.className = "rt-fts-in rt-fts";
        fin.setAttribute("aria-label", t("First-token timeout of {name}", { name: memberName(ref) }));
        fin.title = t("First-token timeout of {name}", { name: memberName(ref) });
        fin.oninput = () => { const n = parseInt(fin.value, 10); if (!d.fts) d.fts = {}; if (n > 0) d.fts[id] = n; else delete d.fts[id]; };
        row.append(fin);
        if (s) row.title = s.members.map((x) => memberLabel(s, x)).join(" → ");
        if (!m && !s) { row.classList.add("off"); row.title = info?.providerOff ? offWords(info) : t("No provider serves {id} now; it is skipped", { id: ref }); }
        const up = el("button", "rt-eico");
        up.type = "button";
        up.title = t("Up");
        up.setAttribute("aria-label", t("Up"));
        up.append(svg(CHEV, 12, 1.8));
        up.style.transform = "rotate(180deg)";
        if (i) up.onclick = () => moveMember(i, i - 1); else up.classList.add("ghost");
        row.append(up);
        const down = el("button", "rt-eico");
        down.type = "button";
        down.title = t("Down");
        down.setAttribute("aria-label", t("Down"));
        down.append(svg(CHEV, 12, 1.8));
        if (i < d.members.length - 1) down.onclick = () => moveMember(i, i + 1); else down.classList.add("ghost");
        row.append(down);
        const rm = el("button", "rt-eico");
        rm.type = "button";
        rm.title = t("Remove");
        rm.setAttribute("aria-label", t("Remove"));
        rm.append(svg("M4 4l8 8M12 4l-8 8", 10, 1.8));
        rm.onclick = () => { d.members.splice(i, 1); rematch(); draw(); };
        row.append(rm);
        if (matched) {
          // a pattern's: it follows the catalog, so it is switched off
          // rather than taken out — the pattern would find it again
          row.title = t("In the group by a pattern: switch it off to send it nothing");
          rm.classList.add("ghost");
          rm.onclick = () => {};
        }
        list.append(row);
      });
      addBtn.querySelector("span").textContent = t(d.members.length ? "Add another entry" : "Add an entry");
    };
    // the picker stays open for as many models as are wanted, each ticked
    // once in, and says which other groups a model is in already (PAMI on
    // Discord: a group was made one model at a time, blind)
    const inGroups = (id) => groups.groups.filter((x) => !x.hidden && x.id !== g?.id && x.members.some((m) => m === id || splitMember(m)[0] === id)).map((x) => x.name);
    const noted = (note, id) => {
      const gs = inGroups(id);
      return gs.length ? [note, t("in {groups}", { groups: gs.join(", ") })].filter(Boolean).join(" · ") : note;
    };
    // entryOptions is the picker's list: one option per entry — the same
    // rows the Provider list page shows (provider × model × key/account).
    // A model with several credentials is one option each, "ref@cred"; one
    // with a single credential is one option without a cred, which the
    // plan reads as every credential.
    const entryOptions = () => {
      const out = [];
      for (const m of groups.models || []) {
        const pid = (m.id || "").split("/")[0];
        const creds = (groups.accounts?.[pid] || []).map((a) => (a.id || "").slice(pid.length + 1)).filter(Boolean);
        const mk = (cred) => ({ value: m.id + (cred ? "@" + cred : ""), label: m.name || m.id, note: cred || m.providerName || "", icon: m.icon, group: m.providerName || "", ref: m.id });
        if (creds.length > 1) for (const c of creds) out.push(mk(c));
        else out.push(mk(""));
      }
      return out;
    };
    addBtn.onclick = (ev) => {
      // what is added is always an entry — the Provider list page's row
      // (provider × model × key/account) — so the picker is that list,
      // grouped by provider, whatever the group's kind
      const options = entryOptions();
      // the member this entry is in the group as: the same credential
      // (ref@cred), or a ref whose reasoning is fixed (deepseek/x:max,
      // #907) — ticked as that and taken out by it, not added twice
      const holding = (id) => {
        const at = id.lastIndexOf("@");
        const ref = at > 0 && !id.startsWith("group/") ? id.slice(0, at) : id;
        const cred = at > 0 && !id.startsWith("group/") ? id.slice(at + 1) : "";
        return d.members.find((m) => {
          // the member's own credential comes off before the models are
          // compared: it was compared against ref whole, so an entry with
          // a key was never ticked and was added a second time at will
          const mat = m.lastIndexOf("@");
          const mref = mat > 0 && !m.startsWith("group/") ? m.slice(0, mat) : m;
          const mcred = mat > 0 && !m.startsWith("group/") ? m.slice(mat + 1) : "";
          if (splitMember(mref)[0] !== ref) return false;
          if (!cred) return true; // a model of one credential: any entry of it
          // a member that pins no credential covers every key and account
          // of that model: it is ticked as this entry rather than added
          // beside it a second time; a fixed effort rides after the key
          return !mcred || mcred === cred || mcred.startsWith(cred + ":");
        });
      };
      openPicker({ id: "", name: "", fields: [] }, { key: "entry", label: "entry", placeholder: t("Filter entries…"), value: "", options, multi: true, rail: true, picked: (id) => !!holding(id), onPick: (id) => {
        if (!id) return;
        const m = holding(id);
        if (!m) d.members.splice(d.members.findLastIndex((x) => !d.matched.includes(x)) + 1, 0, id); // after the last named
        else if (d.matched.includes(m)) return status(t("In the group by a pattern: switch it off to send it nothing"), "warn");
        else if (m.lastIndexOf("@") <= 0 && id.lastIndexOf("@") > 0) {
          // the whole model is in the group, every key and account of it:
          // adding one key beside that would ask the same member twice
          return status(t("{name} is in the group with all its credentials: remove it there to add one of them alone", { name: memberName(id.slice(0, id.lastIndexOf("@"))) }), "warn");
        }
        else { d.members.splice(d.members.indexOf(m), 1); rematch(); }
        draw();
      } }, addBtn, ev);
    };
    const adds = el("div", "rt-madds");
    adds.append(addBtn);
    box.append(list, adds);
    draw();
    const mw = el("div");
    mw.append(box, el("div", "hint", t("The first answers for what the entry's model can do. In order, they are tried top first.")));
    ed.append(el("label", "", t("Entries")), mw);
    // infoTip: a part's how-to, said under an ⓘ beside it rather than in
    // a paragraph under it: on hover, and read out with focus
    const infoTip = (text) => {
      const s = el("span", "rt-info");
      s.tabIndex = 0;
      s.setAttribute("role", "img");
      const set = (v) => { s.title = v; s.setAttribute("aria-label", v); };
      set(text);
      s.set = set;
      s.append(svg(INFO_I, 14, 1.5));
      return s;
    };
    // one executor: the order its entries are tried in, how many it asks
    // at once, how many streams it holds open and its give-up cap; each
    // entry's own first-token timeout is on its row (the doc's §5.2)
    const raceRow = el("div", "rt-grace");
    const numIn = (v, min, label, note) => {
      const i = input(String(v ?? ""), String(min));
      i.type = "number";
      i.min = String(min);
      i.className = "rt-grace-in";
      i.setAttribute("aria-label", label);
      i.title = note;
      return i;
    };
    const seatsIn = numIn(d.race.seats, 1, t("Asked at once"), t("How many entries are asked together. The one that answered last takes the first seat; the rest go in the order picked beside."));
    seatsIn.oninput = () => { d.race.seats = Math.max(1, parseInt(seatsIn.value || "0", 10) || 1); };
    const seatsF = el("label", "rt-grace-f");
    seatsF.append(el("span", "", t("Asked at once")), seatsIn);
    const maxIn = numIn(d.race.maxInFlight, 1, t("Streams at once"), t("How many streams are held open at once, the timed-out answers waiting in the background included. With no room left a timed-out stream is cut and the next entry takes its seat; with room it waits on and may still win."));
    maxIn.oninput = () => { d.race.maxInFlight = Math.max(1, parseInt(maxIn.value || "0", 10) || 1); };
    const maxF = el("label", "rt-grace-f");
    maxF.append(el("span", "", t("Streams at once")), maxIn);
    const waitIn = numIn(d.race.totalWait ?? 30, 1, t("Give up after (s)"), t("The group gives up when nothing has said a word by then: every stream is cut and the group fails."));
    waitIn.oninput = () => { d.race.totalWait = Math.max(1, parseInt(waitIn.value || "0", 10) || 30); };
    const waitF = el("label", "rt-grace-f");
    waitF.append(el("span", "", t("Give up after (s)")), waitIn);
    // the order alone differs: quickest first (the ratio of its first
    // token to what it writes a second) or the list's own. Both ask as
    // many at once as the two boxes say — 1 and 1 is one at a time in the
    // order picked, 3 and 8 is a race in the list's order (the doc's §1)
    const orderBtns = segs([["speed", t("Race")], ["list", t("In order")]], d.race.orderBy, (v) => {
      d.race.orderBy = v;
      draw();
    });
    const raceOpts = el("div", "rt-grace-opts");
    raceOpts.append(seatsF, maxF, waitF);
    // built once for the editor: they say what the draft holds
    syncRace = () => {
      seatsIn.value = String(d.race.seats ?? "");
      maxIn.value = String(d.race.maxInFlight ?? "");
      waitIn.value = String(d.race.totalWait ?? "");
      stickySw.classList.toggle("on", d.race.sticky !== false);
      stickySw.setAttribute("aria-checked", String(d.race.sticky !== false));
    };
    // the session keeping: on, the conversation stays with the entry that
    // answered it last (the vendor's prompt cache holds); off, every
    // request starts from the top of the order (the doc's 会话保持)
    const stickySw = el("button", "lib-switch rt-mon" + (d.race.sticky === false ? "" : " on"));
    stickySw.type = "button";
    stickySw.setAttribute("role", "switch");
    stickySw.setAttribute("aria-checked", String(d.race.sticky !== false));
    stickySw.setAttribute("aria-label", t("Session keeping"));
    stickySw.append(el("i")); // the knob: without it the switch is an empty, invisible button
    stickySw.title = d.race.sticky === false ? t("Sticky off: every request starts from the top of the order") : t("Session keeping on: the conversation stays with the entry that answered it last, so the vendor's prompt cache holds");
    stickySw.onclick = () => {
      d.race.sticky = d.race.sticky === false;
      stickySw.classList.toggle("on", d.race.sticky !== false);
      stickySw.setAttribute("aria-checked", String(d.race.sticky !== false));
      stickySw.title = d.race.sticky === false ? t("Sticky off: every request starts from the top of the order") : t("Session keeping on: the conversation stays with the entry that answered it last, so the vendor's prompt cache holds");
    };
    const stickyF = el("label", "rt-grace-f");
    stickyF.append(el("span", "", t("Session keeping")), stickySw);
    raceOpts.append(stickyF);
    raceRow.append(el("span", "rt-grace-l", t("Try in")), orderBtns, raceOpts);
    ed.append(raceRow);
    // what its entries answered last, read once a minute at most: the two
    // columns fill themselves in as the reply comes
    readEntrySpeeds().then((had) => { if (had) draw(); });

    // patterns: models found by their ids, now and as providers list new
    // ones (#766): after the models named, in the catalog's order, until
    // the user puts them in another; then each keeps its place, one gone
    // drops out and one new goes last
    function rematch() {
      const named = d.members.filter((x) => !d.matched.includes(x)), was = [...named, ...d.matched];
      const placed = d.members.some((x, i) => x !== was[i]);
      d.matched = matchedBy(d.match, named);
      d.members = placed ? [...d.members.filter((x) => named.includes(x) || d.matched.includes(x)), ...d.matched.filter((x) => !d.members.includes(x))] : [...named, ...d.matched];
      // a rule keeps the models still here; it goes with its last
      d.rules = d.rules.filter((r) => { setUses(r, rUses(r).filter((x) => d.members.includes(x))); return !!r.use; });
      d.fast = d.fast.filter((x) => d.members.includes(x));
      d.off = d.off.filter((x) => d.members.includes(x));
    }
    // the patterns a group finds models by, whether its models stay out of
    // agents' lists and the levels it offers are no longer edited here:
    // what a group has saved still goes back with it, and rematch() above
    // still keeps the models its patterns matched among its entries

    // the window agents are told: the largest member's, the smallest's, or
    // one the user names (Mikan on Discord)
    const ctxHint = el("div", "hint"), ctxW = el("div", "ctx-wrap");
    const ctxIn = keys(input(d.context > 0 ? ctxShort(d.context).toLowerCase() : "", t("e.g. 200k")));
    ctxIn.classList.add("ctx-in");
    ctxIn.setAttribute("aria-label", t("Context"));
    ctxHint.id = "groupContextHint";
    ctxIn.setAttribute("aria-describedby", ctxHint.id);
    const ctxOf = (id) => groups.models.find((x) => x.id === id)?.context || infoOf(id)?.context || 0;
    const ctxs = () => d.members.map(ctxOf).filter((n) => n > 0);
    const ctxMode = () => d.context === -1 ? "smallest" : d.context > 0 ? "custom" : "largest";
    let ctxErr = "";
    const drawCtx = () => {
      const ns = ctxs(), mode = ctxMode();
      ctxIn.hidden = mode !== "custom";
      ctxIn.setAttribute("aria-invalid", String(!!ctxErr));
      const big = ns.length ? Math.max(...ns) : 0, small = ns.length ? Math.min(...ns) : 0;
      ctxHint.textContent = ctxErr || (mode === "custom"
        ? t("Agents are told the group takes {n} tokens; a longer conversation still goes on to a member with room for it.", { n: (d.context || 0).toLocaleString(intlLang()) })
        : mode === "smallest"
          ? (small ? t("Agents are told {n} tokens, its smallest model's, so they compact before any member would turn the conversation away.", { n: small.toLocaleString(intlLang()) }) : t("Agents are told its smallest model's window, so they compact before any member would turn the conversation away."))
          // members whose windows differ (windy on Discord: a 1M group
          // with a 128K member) are named with the length that skips them,
          // by provider too, as one model's members differ by it alone
          : small && small < big ? t("Agents are told {n} tokens, its largest model's; a conversation longer than {m} skips {who} for a member with room for it.", { n: big.toLocaleString(intlLang()), m: small.toLocaleString(intlLang()), who: d.members.filter((id) => ctxOf(id) === small).map((id) => memberLabel(g || {}, id)).join(", ") })
          : (big ? t("Agents are told {n} tokens, its largest model's; a conversation too long for one member goes on to one with room for it.", { n: big.toLocaleString(intlLang()) }) : t("Agents are told its largest model's window; a conversation too long for one member goes on to one with room for it.")));
    };
    const CTX_OPTS = [["largest", "Largest model's"], ["smallest", "Smallest model's"], ["custom", "Custom"]];
    ctxW.append(segs(CTX_OPTS.map(([id, n]) => [id, t(n)]), ctxMode(), (v) => {
      ctxErr = "";
      if (v === "largest") d.context = 0;
      else if (v === "smallest") d.context = -1;
      else { const ns = ctxs(); d.context = d.context > 0 ? d.context : (ns.length ? Math.min(...ns) : 200000); ctxIn.value = ctxShort(d.context).toLowerCase(); }
      drawCtx();
      if (v === "custom") ctxIn.focus();
    }), ctxIn, ctxHint);
    ctxIn.oninput = () => {
      const m = ctxIn.value.trim().toLowerCase().replace(/_/g, "").match(/^(\d+(?:\.\d+)?)\s*([km]?)$/);
      const n = m ? Math.round(+m[1] * (m[2] === "k" ? 1e3 : m[2] === "m" ? 1e6 : 1)) : 0;
      ctxErr = n > 0 ? "" : t("{v} is not a length of tokens like 200k or 1m", { v: ctxIn.value.trim() });
      if (n > 0) d.context = n;
      drawCtx();
    };
    ed.append(el("label", "", t("Context")), ctxW);
    drawCtx();

    const bar = el("div", "bar view-foot");
    if (g) {
      const del = el("button", "text danger", t("Remove"));
      del.onclick = () => groupAction("delete", { id: g.id }, t("{name} removed", { name: g.name }));
      bar.append(del);
    }
    bar.append(el("span", "grow"));
    const cancel = el("button", "text", t("Cancel"));
    cancel.onclick = cancelGroup;
    const saveBtn = el("button", "text primary", t(g ? "Save" : "Add"));
    const save = () => {
      if (!d.members.length && !d.match.length) { addBtn.focus({ preventScroll: true }); return status(t("A group needs a model in it"), "warn"); }
      // the rules, hours and folders rows went with the group's old
      // strategy editor (dee1235); a plan group carries its behavior in
      // the tree, so only the members are checked here — the vestigial
      // checks referenced bareRule/COND_KINDS, which are gone, and threw
      // "bareRule is not defined" on every Save
      saveBtn.classList.add("busy");
      // refused, Add can be pressed again (busy, it takes no clicks)
      const raceEntryNodes = (m) => {
        const ft = ((d.fts || {})[m] || 0) * 1000;
        const at = m.lastIndexOf("@");
        if (at > 0) {
          return [{ kind: "entry", ref: m.slice(0, at), cred: m.slice(at + 1), firstTokenMs: ft }];
        }
        const pid = m.split("/")[0];
        const creds = (groups?.accounts?.[pid] || []).map((a) => (a.id || "").slice(pid.length + 1)).filter(Boolean);
        return creds.length ? creds.map((c) => ({ kind: "entry", ref: m, cred: c, firstTokenMs: ft })) : [{ kind: "entry", ref: m, firstTokenMs: ft }];
      };
      const seats = Math.max(1, d.race.seats | 0);
      const plan = {
        kind: "race",
        // quickest first or the list's own: the only difference between
        // what the two old group kinds did (the doc's §1)
        orderBy: d.race.orderBy === "list" ? "list" : "speed",
        seats,
        // never below what it asks at once: the timed-out ones waiting
        // on are on top of those (schedule.Node.MaxInFlight)
        maxInFlight: Math.max(seats, d.race.maxInFlight | 0),
        totalWaitMs: Math.max(1, d.race.totalWait || 30) * 1000,
        members: d.members.flatMap(raceEntryNodes),
      };
      if (d.race.sticky === false) plan.sticky = false; // left out when on: unset is the keeping
      if (!d.members.length) return status(t("A group needs a model in it"), "warn");
      groupAction("save", { id: idOf(), from: g?.id, name: d.name.trim() || idOf(), plan,
        // what the tree does not carry and a save would else reset: the
        // context agents are told, whether its models stay listed, and the
        // family the page groups by
        // off 一并透传：漏了它，savePlanGroup 整体替换会把条目的停用
        // 列表静默清空，停用的成员保存后复活接流量
        context: d.context || 0, unlists: !!d.unlists, family: g?.family || "",
        off: [...(d.off || [])] }, t(g ? "{name} saved" : "{name} added", { name: d.name.trim() || idOf() }))
        .then(() => saveBtn.classList.remove("busy"));
    };
    // what goes wrong is said where it is seen, never a click that does nothing
    saveBtn.onclick = () => {
      try { save(); } catch (e) { saveBtn.classList.remove("busy"); status(t("Couldn't save the group: {error}", { error: e.message }), "err"); }
    };
    bar.append(cancel, saveBtn);
    ed.append(bar);
    if (!g) setTimeout(() => name.focus({ preventScroll: true }), 0); // WebKit would scroll the page to put it mid-view
    if (gEdit.was == null) gEdit.was = JSON.stringify(d);
    return ed;
  }
  window.addEventListener("focus", () => { if (shown() || shownG()) loadGroups(true); });
  // newGroupWith: a new group's editor, opened with the model in it — a
  // model of a provider kept for routing groups that no group has, which
  // agents can reach no other way. The picker and the provider's editor
  // ask it (and the tray panel, by ?newgroup= on the window it opens).
  window.newGroupWith = async (id, name, ev) => {
    // app.js's show, not the page's own show (the stage's caption): the
    // groups' page is where the editor is
    if (document.body.classList.contains("window") && document.getElementById("view-gateway")?.hidden && !(await window.show("gateway"))) return;
    if (!groups) await loadGroups();
    if (!groups) return;
    if (groupDirty() && !(await confirmDiscard())) return;
    gEdit = { id: "", draft: { name: name || modelOf(id)?.name || id.split("/").pop(), members: [id], fast: [], routing: "", affinity: "", rules: [], race: { orderBy: "speed", seats: RACE_SEATS, maxInFlight: RACE_MAX, totalWait: 30, sticky: true }, fts: {} } };
    renderGroups();
    const ed = gList.querySelector(".rt-gedit");
    if (ed && window.scrollOnPurpose?.(ev)) ed.scrollIntoView({ block: "center", behavior: "smooth" });
    ed?.querySelector("input")?.focus({ preventScroll: true });
  };
  // newGroup: a new group's editor, from the groups' page's head, on the
  // defaults the doc settled on (3 asked at once, 8 streams open, 30s to
  // give up, quickest first)
  async function newGroup(ev) {
    if (groupDirty() && !(await confirmDiscard())) return;
    gEdit = { id: "", draft: { name: "", members: [], match: [], matched: [], fast: [], off: [], routing: "", affinity: "", rules: [], race: { orderBy: "speed", seats: RACE_SEATS, maxInFlight: RACE_MAX, totalWait: 30, sticky: true }, fts: {} } };
    renderGroups();
    if (!ev) return;
    const ed = gList.querySelector(".rt-gedit");
    if (ed && window.scrollOnPurpose?.(ev)) ed.scrollIntoView({ block: "center", behavior: "smooth" });
    ed?.querySelector("input")?.focus({ preventScroll: true });
  }
  const askedGroup = !single && document.body.classList.contains("window") && params.get("newgroup");
  if (askedGroup) loadGroups().then(() => window.newGroupWith(askedGroup, ""));
  else loadGroups();
  return { dirty: groupDirty, discard: () => { gEdit = null; renderGroups(); }, reload: loadGroups };
}



// —— 路由组编辑器（侧栏每个路由组标签页一个，app.js 的 routeTabs 在
// mount 里调进来）。页面上：暴露名、条目板、添加条目，就这三件。
// 条目板一行一个条目：识别码 / 供应商 / 模型 /（与供应商列表同步的
// 五格并发）/ 上下文 / 图片支持。调度语义：条目从上到下依次尝试，
// 这个暴露模型的并发上限 20 —— 保存时写成 list / 1 / 20、关掉会话
// 保持。加条目的选择器是老编辑器（上面的 routingGroups）那条路原样
// 迁来的，只是账号凭据改按用户名给（网关按它对账）。
window.routeGroupEditor = function (main, tab, hooks, onSaved) {
  const host = main.querySelector(".rt2") || main;
  let groups = null; // /api/groups 的返回：组、模型、各供应商的凭据
  let prov = null;   // /api/providers 的返回：识别码表、各供应商的密钥与登录
  let draft = null;  // { name, entries: [{ref, cred, firstTokenMs}], context, unlists, family }
  let root = null;   // 组的竞速根参数，保留它的放弃上限
  let was = "";      // 上次读/存时的草稿，改动与否拿它比
  let gone = false;
  let saveBtn = null;
  let board = null;  // 条目板容器
  const rowNums = new Map(); // 每行条目的并行等待数框，先占位（键 "ref@cred"），做什么后面说

  // flatEntries 把调度树摊平：条目按顺序列出来，小组大组都不留
  const flatEntries = (node, out) => {
    out = out || [];
    if (!node || typeof node !== "object") return out;
    if (node.kind === "entry") out.push({ ref: node.ref || "", cred: node.cred || "", firstTokenMs: node.firstTokenMs || 0, lanes: node.lanes || 1 });
    else for (const m of node.members || []) flatEntries(m, out);
    return out;
  };

  // 与老编辑器一致："provider/model:low" 里的固定推理档拆出来
  const LEVELS = ["none", "minimal", "low", "medium", "high", "xhigh", "max"];
  const splitMember = (id) => {
    id = id || "";
    if (id.startsWith("group/")) return [id, ""];
    const m = /^(.+):((?:none|minimal|low|medium|high|xhigh|max)(?:\+(?:none|minimal|low|medium|high|xhigh|max))*)$/i.exec(id);
    if (!m) return [id, ""];
    return [m[1], LEVELS.filter((v) => m[2].toLowerCase().split("+").includes(v)).join("+")];
  };
  const modelOf = (ref) => groups?.models.find((m) => m.id === ref);
  // offKeyOf 是条目在组 Off 列表里的键："ref@cred"（ref 带固定档）
  const offKeyOf = (en) => en.ref + (en.cred ? "@" + en.cred : "");
  const slug = (s) => String(s || "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");

  // 凭据的几种叫法对齐：plan 钉的是稳定 ID（老组）或用户名（选择器新
  // 给的），网关的槽位键和识别码键按用户名（login:user）或密钥指纹。
  // credUserOf 把稳定 ID 映回用户名（/api/providers 的 logins 自带 id）。
  const credUserOf = (cred) => {
    cred = String(cred || "").replace(/^login:/, "").toLowerCase();
    for (const p of prov?.providers || []) {
      for (const l of p.account?.logins || []) {
        if (l.id && String(l.id).toLowerCase() === cred) return l.user;
      }
    }
    return null;
  };
  // credOnTheWire 是条目凭据上网关对账的形态：剥 login: 前缀、稳定 ID
  // 映回用户名（密钥指纹原样过）。plan 成员的 cred 和 Off 的键必须是
  // 同一形态，否则 IsOff 的精确包含永远查不到，停用保存后静默失效。
  const credOnTheWire = (cred) => {
    cred = String(cred || "").replace(/^login:/, "");
    return cred ? (credUserOf(cred) || cred) : "";
  };
  // rowCreds 是行上钉的凭据能是的几种样子（小写、去 login: 前缀）——
  // 配对用；查识别码表要用原样大小写，codeOf 自己拼
  const rowCreds = (en) => {
    const cred = String(en.cred || "").replace(/^login:/, "");
    if (!cred) return [];
    const out = new Set([cred.toLowerCase()]);
    const user = credUserOf(cred);
    if (user) out.add(String(user).toLowerCase());
    return [...out];
  };
  // codesOf 是条目覆盖的识别码列表：钉了凭据且解析得到的就一条；没钉
  // （cred 为空，"它的哪一条都算"）或钉的凭据已不在今天的登录/密钥表
  // 里的，落回这条模型当前全部凭据的识别码——识别码不只是显示，也是
  // 行与模型对账的键（走 entryCodes）。
  const codesOf = (en) => {
    const base = splitMember(en.ref)[0], table = prov?.entryCodes || {};
    const cred = String(en.cred || "").replace(/^login:/, "");
    const tries = [cred];
    const user = credUserOf(cred);
    if (user) tries.push(user);
    for (const c of tries) {
      const hit = table[base + "@" + c] || table[base + "@login:" + c];
      if (hit) return [hit];
    }
    const out = new Set();
    for (const k of Object.keys(table)) {
      if (k.startsWith(base + "@")) out.add(table[k]);
    }
    return [...out];
  };
  const codeOf = (en) => codesOf(en)[0] || "";
  const ctxOf = (en) => modelOf(splitMember(en.ref)[0])?.context || 0;
  const imgOf = (en) => { const m = modelOf(splitMember(en.ref)[0]); return { yes: !!m?.images, unknown: !!m?.imagesUnknown }; };

  async function load() {
    try {
      // shell 的 providers 全局没读过供应商页时是个空壳，只有真有货才用
      const gp = window.providers;
      const gpOk = gp && ((gp.providers || []).length || Object.keys(gp.entryCodes || {}).length);
      [groups, prov] = await Promise.all([api("groups"), gpOk ? Promise.resolve(gp) : api("providers").catch(() => null)]);
    } catch (e) { status(e.message, "err"); return; }
    const g = tab.id ? groups.groups.find((x) => x.id === tab.id) : null;
    if (tab.id && !g) { gone = true; draw(); return; }
    const plan = g?.plan && typeof g.plan === "object" ? g.plan : null;
    root = { totalWaitMs: plan?.totalWaitMs || 30000 };
    const offs = new Set(g?.off || []);
    draft = { name: g?.name || tab.name || "", entries: flatEntries(plan).map((en) => ({ ...en, lanes: en.lanes || 1, off: offs.has(offKeyOf(en)) })), context: g?.context || 0, unlists: !!g?.unlists, family: g?.family || "" };
    was = JSON.stringify(draft);
    draw();
  }

  function draw() {
    if (gone) {
      host.replaceChildren(el("div", "none rt2-none", "这个路由组已经不存在，把标签页关掉就行。"));
      return;
    }
    if (!draft) { host.replaceChildren(el("div", "none rt2-none", "读取中…")); return; }
    const nameIn = input(draft.name, "比如：日常对话");
    nameIn.setAttribute("aria-label", "暴露名");
    nameIn.oninput = () => { draft.name = nameIn.value; };
    saveBtn = el("button", "text primary", tab.id ? "保存" : "创建");
    saveBtn.type = "button";
    saveBtn.onclick = save;
    nameIn.onkeydown = (e) => { if (e.key === "Enter") { e.preventDefault(); saveBtn.onclick(); } };
    const nameRow = el("div", "rt2-namerow");
    nameRow.append(el("label", "", "暴露名"), nameIn, saveBtn);
    if (!tab.id) setTimeout(() => nameIn.focus({ preventScroll: true }), 0);

    const addBtn = el("button", "text rt2-add");
    addBtn.type = "button";
    addBtn.append(svg(PLUS, 11, 1.8), el("span", "", "添加条目"));
    addBtn.onclick = (ev) => pickEntries(addBtn, ev);
    const secRow = el("div", "rt2-secrow");
    secRow.append(el("label", "", "条目"), el("span", "rt2-note", "按顺序调用，从上到下依次尝试"), el("span", "grow"), addBtn);

    board = el("div", "fbl rt-etable rt2-etable");
    drawBoard();

    // 页脚：左下角"删除路由组"，二级确认——点一次变红进入待删，再点
    // 一次才真删，5 秒不点自动复位。删完连配置一起删，关掉标签页
    const foot = el("div", "rt2-foot");
    if (tab.id) {
      const del = el("button", "text rt2-delgroup", "删除路由组");
      del.type = "button";
      del.title = "删除整个路由组（连同它的条目配置），Agent 立刻不可用。点两次确认";
      del.onclick = async () => {
        if (!del.dataset.armed) {
          del.dataset.armed = "1";
          del.classList.add("armed");
          del.textContent = "再点一次，确认删除";
          setTimeout(() => { if (del.dataset.armed) { delete del.dataset.armed; del.classList.remove("armed"); del.textContent = "删除路由组"; } }, 5000);
          return;
        }
        try {
          await api("groups/delete", { id: tab.id });
          was = JSON.stringify(draft); // 删干净了，关页不再问丢弃
          routeTabClose(tab.key);
        } catch (e) { status(e.message, "err"); }
      };
      foot.append(del);
    }
    foot.append(el("span", "grow"));

    const card = el("div", "list editor rt-gedit rt2-card");
    card.append(nameRow, secRow, board, foot);
    host.replaceChildren(card);
  }

  // —— 条目板 ——

  // 图片支持：上下文旁的小图标。收的画绿图，清单没说的画半透图，
  // 不收的不画——没能力的条目不该挂图片标识
  const IMG_D = "M2.5 3.5h11v9h-11z M2.5 10.8l3.2-3.2 2.4 2.4 1.8-1.8 3.6 3.6 M6.1 6.3a.9.9 0 1 1-.01.01";
  const imgIcon = (en) => {
    const im = imgOf(en);
    const s = el("span", "rt2-img" + (im.yes ? " yes" : im.unknown ? " unk" : ""));
    if (im.yes || im.unknown) s.append(svg(IMG_D, 13, 1.4));
    s.title = im.yes ? "收图片输入" : im.unknown ? "它的清单没说收不收图片" : "不收图片输入";
    return s;
  };
  // 上下文未知（清单没说）的显示按默认 200K（后端的 WindowUnknown）；网关
  // 对它的上下文不设限，显示归显示，别让板子说谎
  const ctxWords = (n) => (n ? Math.round(n / 1e3) + "K" : "200K");

  // 条目表：备份里路由组条目的交互原样搬来 —— 拖拽把手（拖动/Alt+↑/↓
  // 换位，拖拽中不重画）、上下按钮；列是这一版的：识别码 / 供应商 /
  // 模型 / 上下文 / 图。
  let arranging = false, renderPending = false;
  const arranged = () => {
    arranging = false;
    if (renderPending) { renderPending = false; drawBoard(); }
  };
  function moveMember(from, to, keep) {
    if (!draft || to < 0 || to >= draft.entries.length || to === from) return;
    draft.entries.splice(to, 0, draft.entries.splice(from, 1)[0]);
    draw();
    if (keep) board.querySelector(`.rt-mhandle[data-at="${to}"]`)?.focus({ preventScroll: true });
  }
  function memberHandle(i, row) {
    const b = el("button", "i ag-handle rt-mhandle", String(i + 1));
    b.type = "button";
    b.dataset.at = i;
    b.title = "拖动排序 · Alt+↑/↓";
    b.onkeydown = (e) => {
      if (!e.altKey || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) return;
      e.preventDefault();
      e.stopPropagation();
      moveMember(i, i + (e.key === "ArrowUp" ? -1 : 1), true);
    };
    b.onclick = (e) => { e.stopPropagation(); delete b.dataset.dragged; };
    b.onpointerdown = (e) => {
      // 拖拽期间板的每两秒一轮重画让路（arranging），拖完再补——原版
      // groupArranging 那套，没有它拖到一半 DOM 被换掉，拖拽就断
      if (arranging) return;
      const rows = [...board.children].filter((r) => r.querySelector(".rt-mhandle"));
      arranging = !!dragRows(e, b, row, board, rows, (to) => moveMember(i, to), () => {}, arranged);
    };
    return b;
  }
  function drawBoard() {
    if (!board || !draft) return;
    // 拖拽进行中不重画，拖完由 arranged() 补
    if (arranging) { renderPending = true; return; }
    const head = el("div", "fbrow rt-ehead");
    // 把手列也要占位：少一个 i，整个标题行就错一格
    head.append(el("i"), el("i"), el("span", "ec-code", "识别码"), el("i"), el("span", "ec-p", "供应商"), el("span", "ec-m", "模型"),
      el("span", "ec-ctx", "上下文"), el("span", "ec-img", "图"), el("i"));
    board.replaceChildren(head);
    if (!draft.entries.length) {
      board.append(el("div", "rt2-none", "还没有条目，点「添加条目」把供应商列表里的模型放进来。"));
      return;
    }
    // 行按识别码认 try：没成的是红，还在线路里的是橙（状态列已删，
    // 一次只试一条，从上到下，没有并发可看）

    draft.entries.forEach((en, i) => {
      const base = splitMember(en.ref)[0], fixed = splitMember(en.ref)[1];
      const m = modelOf(base);
      const row = el("div", "fbrow" + (m && !en.off ? "" : " off"));
      // 启停：关着的条目留在原位，网关跳过它
      const sw = el("button", "lib-switch rt-mon" + (en.off ? "" : " on"));
      sw.type = "button";
      sw.setAttribute("role", "switch");
      sw.setAttribute("aria-checked", String(!en.off));
      sw.setAttribute("aria-label", (m ? m.name || m.id : en.ref) + (en.off ? "（已停用）" : "（启用中）"));
      sw.title = en.off ? "停用中：请求跳过它，位置留着。点此启用" : "启用中：请求按顺序试它。点此停用";
      sw.append(el("i"));
      sw.onclick = () => { en.off = !en.off; draw(); };
      row.append(sw, memberHandle(i, row));
      const codes = codesOf(en);
      const code = el("span", "ec-code", codes[0] || "—");
      if (codes.length > 1) code.title = "覆盖 " + codes.length + " 条凭据的识别码：" + codes.join("、");
      else if (!codes.length) code.title = "还没有识别码";
      const ic = el("i", "ec-ico");
      ic.append(icon(m?.icon || "generic"));
      const pcell = el("span", "ec-p", m ? m.providerName || base.split("/")[0] : base.split("/")[0] || "—");
      pcell.title = pcell.textContent;
      const nm = el("span", "ec-m", m ? m.name || m.id : en.ref);
      nm.title = base + (en.cred ? " @ " + en.cred : "");
      if (fixed) { const f = el("small", "", fixed); nm.append(f); }
      const ctxN = ctxOf(en);
      const ctx = el("span", "ec-ctx", ctxWords(ctxN));
      if (!ctxN) ctx.title = "上下文未知：清单没说，按默认 200K 显示；网关对它不设限";
      const rm = el("button", "rt2-del");
      rm.type = "button";
      rm.title = "移出条目（保存后生效）";
      rm.setAttribute("aria-label", "移出条目");
      // 垃圾桶：M3.2 4.7h9.6 盖沿 / M6.3 4.7V3.2h3.4v1.5 提手 / 桶身两壁
      // / M6.8 7.2v3.6 M9.2 7.2v3.6 桶内两道
      rm.append(svg("M3.2 4.7h9.6 M6.3 4.7V3.2h3.4v1.5 M4.4 4.7l.6 8.3h6l.6-8.3 M6.8 7.2v3.6 M9.2 7.2v3.6", 14, 1.4));
      rm.onclick = () => { rowNums.delete(base + "@" + (en.cred || "")); draft.entries.splice(i, 1); draw(); };
      row.append(code, ic, pcell, nm, ctx, imgIcon(en), rm);
      if (!m) row.title = "现在没有供应商提供这个模型；保存后它原样留着。";
      else if (en.off) row.title = "停用中：请求跳过这条，位置留着。";
      board.append(row);
    });
  }

  // 加条目：选择器从老编辑器原样迁来 —— 供应商列表的行（供应商 × 模型
  // × 凭据），多凭据的模型一条一格；已在组里的勾着，再点不进去。凭据
  // 按网关对账的形态给：密钥是指纹，账号是用户名。
  const holding = (id) => {
    const at = id.lastIndexOf("@");
    const ref = at > 0 ? id.slice(0, at) : id;
    const cred = at > 0 ? id.slice(at + 1) : "";
    return draft.entries.find((en) => {
      if (splitMember(en.ref)[0] !== ref) return false;
      if (!cred) return true; // 单凭据的模型：它的哪一条都算
      // 不钉凭据的条目盖住这个模型的全部凭据，算同一条
      return !en.cred || rowCreds(en).includes(cred.toLowerCase());
    });
  };
  const entryOptions = () => {
    const out = [];
    for (const m of groups.models || []) {
      const pid = (m.id || "").split("/")[0];
      const creds = (groups.accounts?.[pid] || []).map((a) => (a.key ? (a.id || "").slice(pid.length + 1) : (a.user || (a.id || "").slice(pid.length + 1))));
      const mk = (cred) => ({ value: m.id + (cred ? "@" + cred : ""), label: m.name || m.id, note: cred || m.providerName || "", icon: m.icon, group: m.providerName || "", ref: m.id });
      if (creds.length > 1) for (const c of creds) out.push(mk(c));
      else out.push(mk(creds.length ? creds[0] : ""));
    }
    return out;
  };
  function pickEntries(anchor, ev) {
    if (!groups || !draft) return;
    openPicker({ id: "", name: "", fields: [] }, {
      key: "entry", label: "entry", placeholder: "筛选条目…", value: "",
      options: entryOptions(), multi: true, rail: true,
      picked: (id) => !!holding(id),
      onPick: (id) => {
        if (!id) return;
        if (holding(id)) return status("这个条目已经在组里", "warn");
        const at = id.lastIndexOf("@");
        draft.entries.push({ ref: at > 0 ? id.slice(0, at) : id, cred: at > 0 ? id.slice(at + 1) : "", firstTokenMs: 0 });
        draw();
      },
    }, anchor, ev);
  }

  async function save() {
    if (!draft || !groups) return;
    if (!draft.entries.length) return status("路由组至少要有一个条目，先「添加条目」。", "warn");
    saveBtn.classList.add("busy");
    try {
      let id = tab.id;
      if (!id) {
        id = slug(draft.name) || "group";
        const base = id;
        let n = 1;
        while (groups.groups.some((x) => x.id === id)) id = `${base}-${++n}`;
      }
      const members = draft.entries.map((en) => {
        // 老组的凭据存的是稳定 ID，网关按用户名对账：保存时换成用户名，
        // 试轨迹、识别码、槽位键从此都对得上
        // lanes 不再有界面：一次只试一条（网关把 seats/在途恒压成 1），
        // 保存恒为 1，只为兼容旧 plan 的字段
        const e = { kind: "entry", ref: en.ref, cred: credOnTheWire(en.cred), lanes: 1 };
        if (en.firstTokenMs) e.firstTokenMs = en.firstTokenMs;
        return e;
      });
      // 这个暴露模型的语义：条目从上到下依次尝试（list、一次一个），
      // 并发上限 20，不要会话保持；放弃上限沿用组里原有的
      const plan = { kind: "race", orderBy: "list", seats: 1, maxInFlight: 20,
        totalWaitMs: root.totalWaitMs || 30000, sticky: false, members };
      // off 的键与 plan 成员同形（credOnTheWire）：IsOff 是精确包含，
      // 键形不一致时停用保存后静默失效、重载后标记也丢
      const off = draft.entries.filter((en) => en.off).map((en) => {
        const cred = credOnTheWire(en.cred);
        return en.ref + (cred ? "@" + cred : "");
      });
      const next = await api("groups/save", { id, from: tab.id || "", name: draft.name.trim() || id, plan, off,
        context: draft.context, unlists: draft.unlists, family: draft.family });
      if (next?.groups) groups = next;
      const g = groups.groups.find((x) => x.id === id);
      draft.name = g?.name || draft.name.trim();
      was = JSON.stringify(draft);
      tab.id = id;
      tab.name = draft.name;
      onSaved?.(id, draft.name);
      status("已保存：" + draft.name, "ok");
      draw();
    } catch (e) {
      status(e.message, "err");
    } finally {
      saveBtn.classList.remove("busy");
    }
  }

  hooks.dirty = () => !!draft && JSON.stringify(draft) !== was;
  hooks.discard = () => { gone = false; load(); };

  load();
};
