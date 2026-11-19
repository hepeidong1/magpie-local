// The Routing groups page (plugins/gateway): the page hosts the routing
// groups' section, which the routing plugin draws into it (plugins/routing's
// client.js). What the page had besides — the gateway's state and keys, how
// to connect a client, its models and recent calls — is gone: the keys and
// the address are on Settings, and the gateway serves the groups.
//
// gatewayFixSays and gatewaySkewLine stay here: the warning over the Agents
// list of a magpie of another version serving the gateway is this plugin's.
//
// Classic script, loaded before app.js: what runs as it loads touches
// neither the page nor app.js; the page registers at once, and the routing
// plugin's drawing finds the host in it (magpie.start, app.js).

magpie.plugin("gateway", (ctx) => {
  ctx.slots.add("page", {
    key: "gateway",
    title: "Routing groups",
    html: `<div id="rtGsecHost"></div>`,
    // the groups' section (plugins/routing) is drawn into the host above
    mount() { routingGroups(); },
  });
});

// gatewayFixSays is why a take-over or a restart didn't go, in the page's
// language, naming the process it was
function gatewayFixSays(o) {
  const p = { port: o.port || "", pid: o.pid || "", path: o.path || "", v: o.version || "", error: o.error || "" };
  switch (o.reason) {
    case "not-magpie": return t("Port {port} is held by {path} (pid {pid}), which isn't magpie, so magpie leaves it alone. Quit it yourself, or move the gateway to another port in Settings.", p);
    // leslie_luo on Discord: an old magpie in OrbStack, its helper named
    // as the program to quit
    case "container": return t("Port {port} is forwarded by {path} (pid {pid}) to a container or a VM: the magpie {v} answering there runs inside it, out of this magpie's reach. Stop that container (docker ps lists it, docker stop <name> stops it, or stop it in OrbStack or Docker Desktop), or move this magpie's gateway to another port in Settings.", p);
    case "unseen": return t("magpie can't see which process holds port {port}: it runs as another user or as administrator. Quit it there.", p);
    case "denied": return t("magpie {v} (pid {pid}, {path}) runs as another user or as administrator, so this magpie can't quit it. Quit it there.", p);
    case "stuck": return t("magpie {v} (pid {pid}) didn't quit. End it in your system's process list (Activity Monitor, Task Manager).", p);
    case "respawned": return t("Something started magpie on port {port} again at once ({path}, pid {pid}): a login item or a service keeps it running (a LaunchAgent, a systemd unit, a scheduled task). Remove that, then try again.", p);
    case "taken": return p.pid
      ? t("Port {port} is in use by {path} (pid {pid}): quit it, or move the gateway to another port in Settings.", p)
      : t("Port {port} is in use by another program: quit it, or move the gateway to another port in Settings.", p);
    case "other": return t("magpie {v} serves the gateway, and it isn't older than this one: quit it there to serve the gateway here.", p);
  }
  return t("The gateway couldn't be served here: {error}", p);
}

// gatewaySkewLine: a magpie of another version serves the gateway in this
// one's place (gui/gateway_older.go). matrix0487 on Discord: the app wrote pi
// a model its older gateway didn't know, pi said 404 "add a provider in
// magpie first", and only the Gateway page told of the older magpie. Said
// over the Agents list, in the panel and the window, until it is gone: an
// older copy of the app is taken over by itself once idle; any other is
// the user's to update or quit, here with the Gateway page's own button.
let gwSkewFix = { busy: false, err: "", pid: 0 };
function gatewaySkewLine(list) {
  let box = $("#gwSkew");
  const o = state.gatewayOther;
  if (!o || !o.version) {
    if (box) box.hidden = true;
    return;
  }
  if (!box) {
    box = el("div", "gw-skew");
    box.id = "gwSkew";
    ($("#agentsLead") || list).before(box);
  }
  box.hidden = false;
  if (gwSkewFix.pid !== (o.pid || 0)) gwSkewFix = { busy: false, err: "", pid: o.pid || 0 };
  const p = { v: o.version, mine: o.mine, n: o.busy || 0 };
  let head, why, quit = false;
  if (o.replacing) {
    head = t("magpie {v}, an older copy of this app, still serves the gateway", p);
    why = o.busy > 0
      ? t(o.busy === 1 ? "This magpie ({mine}) takes it over once its {n} request in flight finishes; until then agents' requests go through magpie {v}." : "This magpie ({mine}) takes it over once its {n} requests in flight finish; until then agents' requests go through magpie {v}.", p)
      : o.busy < 0
        ? t("This magpie ({mine}) takes it over once it has no request in flight; until then agents' requests go through magpie {v}.", p)
        : t("This magpie ({mine}) takes it over in a moment.", p);
  } else if (o.older) {
    head = t("The gateway is served by magpie {v}, older than this app ({mine})", p);
    why = o.serves === "serve"
      ? t("It runs as magpie serve, in a terminal or as the service. Agents' requests go through it without this version's fixes, and models set up here may not reach them. Update that magpie and start it again (magpie daemon restart, for the service), or quit it here and this app serves the gateway.", p)
      : t("Agents' requests go through it without this version's fixes, and models set up here may not reach them. Quit it here and this app serves the gateway at once.", p);
    quit = true;
  } else if (o.newer) {
    head = t("The gateway is served by magpie {v}, newer than this app ({mine})", p);
    why = t("Agents' requests go through it, and what this app writes for them may not match it. Update this magpie, or quit the newer one there.", p);
  } else {
    head = t("The gateway is served by magpie {v}, a different build from this app ({mine})", p);
    why = t("Agents' requests go through it, and what this app writes for them may not match it. Quit it there for this app to serve the gateway.", p);
  }
  const card = el("div", "deprecation");
  card.setAttribute("role", "note");
  const body = el("div", "dep-body");
  body.append(el("div", "dep-head", head), el("p", "dep-why", why));
  if (gwSkewFix.err) body.append(el("p", "dep-why gw-skew-err", gwSkewFix.err));
  card.append(el("span", "dep-dot"), body);
  if (quit) {
    const acts = el("div", "dep-acts");
    const b = el("button", "text action", gwSkewFix.busy ? t("Quitting magpie {v}…", p) : t("Quit magpie {v} and take over", p));
    b.disabled = b.classList.toggle("busy", gwSkewFix.busy);
    b.onclick = async () => {
      gwSkewFix = { busy: true, err: "", pid: o.pid || 0 };
      gatewaySkewLine(list);
      let out;
      try { out = await api("gateway/take-over", {}); } catch (e) { out = { reason: "error", error: e.message }; }
      gwSkewFix = { busy: false, err: out?.ok ? "" : gatewayFixSays(out || {}), pid: o.pid || 0 };
      try { state = await api("state"); } catch {}
      renderAgents();
    };
    acts.append(b);
    card.append(acts);
  }
  box.replaceChildren(card);
}

// A recorded call's request and response bodies: shared by Usage's ledger
// and Routing's trace, which read the same calls (callBodyPanel); moved
// out of app.js, the core.
// a streamed reply's body is read as its reply, its events or as it came;
// the pick is one for every call and kept, the events drawn per call
let sseView = "events";
try { sseView = localStorage.getItem("magpie.sseView") || "events"; } catch {}
const sseShown = new Map(); // call id → how many events are drawn

function formatWireBody(raw) {
  if (!raw) return "";
  try { return JSON.stringify(JSON.parse(raw), null, 2); } catch { return raw; }
}

// A JSON body as a tree: each object and array folds at its bracket, its
// parts in the snippets' colours; the copy button a line shows on hover
// copies its value, and a click on a key copies the key. The text is the
// JSON pretty-printed, as before, so what is selected and copied by hand
// still reads as JSON. What is folded is kept by the body's key and the
// node's path, across the redraws a new call brings.
const jsonFolds = new Set();
function jsonTree(v, key) {
  const code = el("code", "jt");
  code.append(jtNode(v, undefined, "", key + "|", ""));
  return code;
}
function jtNode(v, k, indent, path, comma) {
  const f = document.createDocumentFragment();
  const head = el("span", "jt-head");
  f.append(indent, head);
  if (k !== undefined) {
    const ks = el("span", "jt-key", JSON.stringify(k));
    ks.title = t("Click to copy the key");
    // a click copies; a drag selects, as in any text
    ks.onclick = (ev) => {
      ev.stopPropagation();
      if (!String(getSelection())) copy(String(k), String(k), null);
    };
    head.append(ks, ": ");
  }
  const copyV = el("button", "jt-copy");
  copyV.type = "button";
  copyV.title = t("Copy value");
  copyV.append(svg(COPY_ICON, 11, 1.5));
  // a string's value is its text, not the JSON that quotes it
  copyV.onclick = (ev) => {
    ev.stopPropagation();
    copy(typeof v === "string" ? v : JSON.stringify(v, null, 2), k !== undefined ? String(k) : t("Value"), copyV);
  };
  if (v === null || typeof v !== "object") {
    head.append(el("span", typeof v === "string" ? "tk-s" : typeof v === "number" ? "tk-n" : "tk-k", JSON.stringify(v)), copyV);
    f.append(comma);
    return f;
  }
  const arr = Array.isArray(v);
  const keys = arr ? v.map((_, i) => i) : Object.keys(v);
  const [o, c] = arr ? ["[", "]"] : ["{", "}"];
  if (!keys.length) {
    head.append(o + c, copyV);
    f.append(comma);
    return f;
  }
  const tog = el("button", "jt-fold");
  tog.type = "button";
  tog.append(svg(CHEV_R, 10, 1.6));
  const kids = el("span", "jt-kids");
  const sum = el("button", "jt-sum", "…");
  sum.type = "button";
  sum.title = t("Unfold");
  sum.dataset.n = t(keys.length === 1 ? "1 item" : "{n} items", { n: keys.length });
  // the summary is on the head's line: what a click on it holds in place is
  // then its neighbours there, not the line after, which the unfolding pushes down
  head.prepend(tog);
  head.append(o, sum, copyV);
  f.append(kids, c, comma);
  let built = false;
  const set = (folded) => {
    if (!folded && !built) { // its insides are drawn the first time it opens
      built = true;
      keys.forEach((kk, i) => kids.append("\n", jtNode(v[kk], arr ? undefined : kk, indent + "  ", path + "/" + kk, i < keys.length - 1 ? "," : "")));
      kids.append("\n" + indent);
    }
    kids.hidden = folded;
    sum.hidden = !folded;
    tog.classList.toggle("open", !folded);
    tog.setAttribute("aria-expanded", String(!folded));
    tog.title = t(folded ? "Unfold" : "Fold");
    if (folded) jsonFolds.add(path); else jsonFolds.delete(path);
  };
  // the box keeps its height across a fold, so what is below it, and the
  // page, stay where they are
  const keep = () => { const pre = tog.closest("pre"); if (pre) pre.style.minHeight = pre.offsetHeight + "px"; };
  tog.onclick = (ev) => { ev.stopPropagation(); keep(); set(!kids.hidden); };
  sum.onclick = (ev) => { ev.stopPropagation(); keep(); set(false); };
  set(jsonFolds.has(path));
  return f;
}

// A streamed body (server-sent events) as its events: each one's name and
// its data, the data lines joined. Anything else is not one: null.
function parseSSE(raw) {
  if (!raw || !/^\s*(?::[^\n]*\n\s*)*(event|data|id|retry):/.test(raw)) return null;
  const events = [];
  let ev = null;
  const done = () => { if (ev && ev.data.length) events.push({ event: ev.event, data: ev.data.join("\n") }); ev = null; };
  for (const line of raw.split(/\r?\n/)) {
    if (line === "") { done(); continue; }
    if (line[0] === ":") continue; // a comment, a keep-alive
    const i = line.indexOf(":");
    const field = i < 0 ? line : line.slice(0, i);
    let value = i < 0 ? "" : line.slice(i + 1);
    if (value[0] === " ") value = value.slice(1);
    ev = ev || { event: "", data: [] };
    if (field === "event") ev.event = value;
    else if (field === "data") ev.data.push(value);
  }
  done();
  return events.length ? events : null;
}

const jsonOr = (s) => { try { return JSON.parse(s); } catch { return undefined; } };

// What a stream adds up to — its text, reasoning and tool calls, how it
// stopped and what it used — for the Anthropic Messages, OpenAI Responses,
// Chat Completions and Gemini streams; null for one it can't read.
function sseReply(events) {
  const out = {};
  const blocks = [];      // Anthropic content blocks, Chat tool calls, Responses items, in order
  const byKey = new Map();
  const block = (key, init) => { let b = byKey.get(key); if (!b) { b = { ...init }; byKey.set(key, b); blocks.push(b); } return b; };
  let kind = "";
  for (const e of events) {
    const d = jsonOr(e.data);
    if (!d || typeof d !== "object") continue;
    const type = d.type || e.event || "";
    if (d.error) { out.error = d.error; continue; }
    if (type === "message_start" && d.message) {
      kind = "anthropic";
      out.id = d.message.id; out.model = d.message.model;
      if (d.message.usage) out.usage = { ...d.message.usage };
    } else if (type === "content_block_start" && d.content_block) {
      kind = "anthropic";
      const cb = d.content_block;
      const b = block("a" + d.index, { type: cb.type });
      if (cb.type === "tool_use" || cb.type === "server_tool_use") { b.id = cb.id; b.name = cb.name; b.input = ""; }
      else if (cb.type === "text") b.text = cb.text || "";
      else if (cb.type === "thinking") b.thinking = cb.thinking || "";
      else Object.assign(b, cb);
    } else if (type === "content_block_delta" && d.delta) {
      const b = block("a" + d.index, { type: "text" });
      const x = d.delta;
      if (x.type === "text_delta") b.text = (b.text || "") + x.text;
      else if (x.type === "thinking_delta") b.thinking = (b.thinking || "") + x.thinking;
      else if (x.type === "input_json_delta") b.input = (b.input || "") + x.partial_json;
    } else if (type === "message_delta") {
      if (d.delta?.stop_reason) out.stop_reason = d.delta.stop_reason;
      if (d.usage) out.usage = { ...out.usage, ...d.usage };
    } else if (type.startsWith("response.")) {
      kind = "responses";
      if (d.response) {
        out.id = d.response.id; out.model = d.response.model; out.status = d.response.status;
        if (d.response.usage) out.usage = d.response.usage;
        if (d.response.error) out.error = d.response.error;
        if (d.response.incomplete_details) out.incomplete = d.response.incomplete_details;
        if (type === "response.completed" && d.response.output?.length) out.output = d.response.output;
      }
      const key = "r" + (d.item_id || d.item?.id || d.output_index);
      if (type === "response.output_item.added" || type === "response.output_item.done") {
        const it = d.item || {};
        const b = block(key, { type: it.type });
        if (it.type === "function_call" || it.type === "custom_tool_call") { b.call_id = it.call_id; b.name = it.name; }
        if (type === "response.output_item.done") {
          if (it.arguments !== undefined) b.arguments = it.arguments;
          if (it.input !== undefined) b.input = it.input;
          const text = (it.content || []).map((c) => c.text || c.refusal || "").join("");
          if (text) b.text = text;
          const sum = (it.summary || []).map((c) => c.text || "").join("\n\n");
          if (sum) b.summary = sum;
        }
      } else if (type === "response.output_text.delta" || type === "response.refusal.delta") {
        const b = block(key, { type: "message" }); b.text = (b.text || "") + d.delta;
      } else if (type === "response.reasoning_summary_text.delta") {
        const b = block(key, { type: "reasoning" }); b.summary = (b.summary || "") + d.delta;
      } else if (type === "response.reasoning_text.delta") {
        const b = block(key, { type: "reasoning" }); b.text = (b.text || "") + d.delta;
      } else if (type === "response.function_call_arguments.delta") {
        const b = block(key, { type: "function_call" }); b.arguments = (b.arguments || "") + d.delta;
      } else if (type === "response.custom_tool_call_input.delta") {
        const b = block(key, { type: "custom_tool_call" }); b.input = (b.input || "") + d.delta;
      }
    } else if (Array.isArray(d.choices)) {
      kind = "chat";
      if (d.id) out.id = d.id;
      if (d.model) out.model = d.model;
      if (d.usage) out.usage = d.usage;
      for (const c of d.choices) {
        const x = c.delta || c.message || {};
        const msg = block("c" + (c.index || 0), { type: "message" });
        if (typeof x.content === "string") msg.content = (msg.content || "") + x.content;
        const r = x.reasoning_content ?? x.reasoning;
        if (typeof r === "string") msg.reasoning = (msg.reasoning || "") + r;
        for (const tc of x.tool_calls || []) {
          const b = block("c" + (c.index || 0) + ":" + (tc.index ?? tc.id), { type: "tool_call" });
          if (tc.id) b.id = tc.id;
          if (tc.function?.name) b.name = (b.name || "") + tc.function.name;
          if (tc.function?.arguments) b.arguments = (b.arguments || "") + tc.function.arguments;
        }
        if (c.finish_reason) out.finish_reason = c.finish_reason;
      }
    } else if (Array.isArray(d.candidates)) {
      kind = "gemini";
      if (d.modelVersion) out.model = d.modelVersion;
      if (d.usageMetadata) out.usage = d.usageMetadata;
      for (const c of d.candidates) {
        for (const p of c.content?.parts || []) {
          if (typeof p.text === "string") {
            const b = block("g" + (c.index || 0) + (p.thought ? "t" : ""), { type: p.thought ? "thought" : "text" });
            b.text = (b.text || "") + p.text;
          } else if (p.functionCall) blocks.push({ type: "functionCall", ...p.functionCall });
        }
        if (c.finishReason) out.finishReason = c.finishReason;
      }
    }
  }
  if (!kind && !out.error) return null;
  // the Responses API's own finished output is the reply, when it came
  const content = (out.output || blocks).filter((b) => !(kind === "chat" && b.type === "message" && !b.content && !b.reasoning));
  // a tool's arguments read as the object they spell, when they are whole
  for (const b of content) {
    for (const k of ["input", "arguments"]) {
      if (typeof b[k] !== "string") continue;
      const v = b[k] === "" && k === "input" ? {} : jsonOr(b[k]);
      if (v !== undefined) b[k] = v;
    }
  }
  const reply = {};
  for (const k of ["id", "model", "status"]) if (out[k] !== undefined) reply[k] = out[k];
  if (content.length) reply[{ anthropic: "content", responses: "output", gemini: "parts" }[kind] || "choices"] = content;
  for (const k of ["stop_reason", "finish_reason", "finishReason", "incomplete", "usage", "error"]) if (out[k] !== undefined) reply[k] = out[k];
  return reply;
}

// The data of each event, as JSON where it is JSON, under its name.
function sseEventNode(e, key) {
  const box = el("span", "sse-ev");
  if (e.event) box.append(el("span", "sse-name", "event: " + e.event + "\n"));
  const d = jsonOr(e.data);
  box.append(d !== null && typeof d === "object" ? jtNode(d, undefined, "", key + "|", "") : e.data); // [DONE] as it is
  return box;
}
function sseData(data) {
  const d = jsonOr(data);
  return d !== null && typeof d === "object" ? JSON.stringify(d, null, 2) : data; // [DONE] as it is
}
const SSE_PAGE = 200; // events drawn at a time: a long stream stays quick to open

function sseBodyPanel(label, raw, truncated, id) {
  const events = parseSSE(raw);
  if (!events) return null;
  const reply = sseReply(events);
  const panel = el("section", "call-body sse");
  const head = el("div", "call-body-head");
  head.append(el("span", "call-body-label", t(label)));
  if (truncated) head.append(el("span", "call-body-truncated", typeof truncated === "string" ? truncated : t("first 256 KB")));
  const views = [["reply", t("Reply")], ["events", t("Events")], ["raw", t("Raw")]].filter(([v]) => v !== "reply" || reply);
  let view = views.some(([v]) => v === sseView) ? sseView : "events";
  const count = el("span", "call-body-count", t(events.length === 1 ? "1 event" : "{n} events", { n: events.length }));
  const pick = segs(views, view, (v) => { view = sseView = v; try { localStorage.setItem("magpie.sseView", v); } catch {} draw(); });
  pick.classList.add("sse-views");
  const copyB = copyBtn("", t(label)); // what it copies is set below
  head.append(count, el("span", "grow"), pick, copyB);
  panel.append(head);
  const pre = el("pre");
  // a long stream's events come a page at a time; what draws more sits
  // under the box, where it stays as they come in rather than at the end
  // of what is drawn
  const foot = el("div", "call-body-foot");
  const shownNote = el("span");
  const moreB = el("button", "link", "");
  foot.append(shownNote, el("span", "grow"), moreB);
  panel.append(pre, foot);
  let code = null, shown = 0;
  const more = () => {
    const upto = Math.min(events.length, Math.max(sseShown.get(id) || 0, shown + SSE_PAGE));
    for (; shown < upto; shown++) code.append(sseEventNode(events[shown], id + "|ev" + shown));
    if (shown > SSE_PAGE) sseShown.set(id, shown);
    foot.hidden = view !== "events" || shown >= events.length;
    shownNote.textContent = t("{n} of {total} events shown", { n: shown, total: events.length });
    moreB.textContent = t("Show {n} more events", { n: Math.min(SSE_PAGE, events.length - shown) });
  };
  moreB.onclick = (ev) => { ev.stopPropagation(); more(); };
  const draw = () => {
    // the box keeps its height across a switch, so what is below it, and
    // the page, stay where they are
    if (pre.isConnected) pre.style.minHeight = pre.offsetHeight + "px";
    code = view === "reply" ? jsonTree(reply, id + "|reply") : el("code", view === "events" ? "jt" : "");
    pre.replaceChildren(code);
    pre.scrollTop = 0;
    shown = 0;
    if (view === "raw") code.textContent = raw;
    if (view === "events") more(); else foot.hidden = true;
  };
  // what is copied is what is shown: the reply, every event, or the body
  copyB.onclick = (ev) => {
    ev.stopPropagation();
    const text = view === "reply" ? JSON.stringify(reply, null, 2) : view === "raw" ? raw
      : events.map((e) => (e.event ? "event: " + e.event + "\n" : "") + sseData(e.data)).join("\n\n");
    copy(text, t(label), copyB);
  };
  draw();
  return panel;
}

// id, given, is a response's, whose stream reads as its events; key names
// the body for what is folded in it
function callBodyPanel(label, raw, truncated, id, key = id) {
  if (id) {
    const sse = sseBodyPanel(label, raw, truncated, id);
    if (sse) return sse;
  }
  const panel = el("section", "call-body");
  const head = el("div", "call-body-head");
  head.append(el("span", "call-body-label", t(label)));
  if (truncated) head.append(el("span", "call-body-truncated", typeof truncated === "string" ? truncated : t("first 256 KB")));
  const formatted = formatWireBody(raw);
  if (formatted) head.append(el("span", "grow"), copyBtn(raw, t(label)));
  panel.append(head);
  const pre = el("pre");
  const v = formatted ? jsonOr(raw) : undefined;
  if (v !== null && typeof v === "object") pre.append(jsonTree(v, key));
  else {
    const code = el("code", "", formatted || t("No body captured"));
    if (!formatted) code.classList.add("empty");
    pre.append(code);
  }
  panel.append(pre);
  return panel;
}
