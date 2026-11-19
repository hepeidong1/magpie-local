// Settings' gateway card and its keys (keys.js) and connect examples
// (client.js): the card has the address, the stop/start switch and a
// restart; the keys every request to the gateway must carry, and how to
// connect a client, follow it. Run after app.js, whose helpers it shares
// (el, segs, input, api, copy, status), and before start.js draws Settings.

// renderPort: the gateway's address on this computer: only its port,
// 1024 up, is typed here; magpie moves its gateway there, or says why it
// can't (another program has it). MAGPIE_ADDR, a whole host:port, comes
// first and the field shows it, off.
let portDraft = { value: null, err: "" };
// portSays is the gateway's word on a port it couldn't take, in the page's
// language: it says the port, so it is matched rather than looked up
function portSays(m) {
  let x;
  if ((x = /^port (\d+) is in use by another program/.exec(m))) return t("Port {p} is in use by another program: pick another one", { p: x[1] });
  if ((x = /^another magpie serves the gateway at (\S+):/.exec(m))) return t("Another magpie serves the gateway at {url}: set its port there, or quit it first", { url: x[1] });
  if (/from 1024 to 65535/.test(m)) return t("A port is a number from 1024 to 65535");
  return t(m);
}
function renderPort(s) {
  const box = $("#portList");
  const r = el("div", "row pref port-row");
  const who = el("div", "who");
  who.append(el("div", "name", t("Gateway address")));
  const sub = el("div", "sub");
  const d = portDraft;
  const now = s.addrEnv ? "" : String(s.port || 3425);
  const field = input(d.value ?? now, "3425");
  field.className = "words gateway-port";
  field.inputMode = "numeric";
  field.setAttribute("aria-label", t("Gateway address"));
  const save = el("button", "text", t("Apply"));
  if (s.addrEnv) {
    field.value = s.addrEnv;
    field.disabled = save.disabled = true;
    sub.textContent = t("MAGPIE_ADDR={addr} sets the gateway’s whole address, host:port: unset it to set the port here", { addr: s.addrEnv });
  } else if (d.err) {
    sub.textContent = d.err;
    sub.classList.add("err");
  } else sub.textContent = t("Only the port is set here, 1024 to 65535; MAGPIE_ADDR=host:port sets the whole address instead. Agents magpie connected move with it. Now {url}", { url: s.gateway || "" });
  who.append(sub);
  save.onclick = () => {
    const p = Number(field.value.trim());
    if (!Number.isInteger(p) || p < 1024 || p > 65535) {
      d.value = field.value; d.err = t("A port is a number from 1024 to 65535");
      return renderPort(s);
    }
    if (p === (s.port || 3425)) { d.value = null; d.err = ""; return renderPort(s); }
    save.disabled = true;
    writingPrefs(api("settings/port", { port: p }).then((res) => res.settings && (prefs = res.settings, res)))
      .then((res) => {
        portDraft = { value: null, err: "" };
        const n = res.port?.moved?.length || 0;
        if (res.port?.error) status(res.port.error, "err");
        else status(n ? t("Gateway on port {p}; {n} agents moved with it", { p, n }) : t("Gateway on port {p}", { p }), "ok", 2500);
        // the sidebar's gateway line, and the list's address the pages
        // copy, say where it is now, not the port before
        if (res.settings?.gateway) renderSideGw({ ...sideGwAt, url: res.settings.gateway, running: true });
        if (providers) loadProviders().catch(() => {});
        renderSettings();
      })
      .catch((e) => { d.value = field.value; d.err = portSays(e.message); status(d.err, "err"); renderSettings(); });
  };
  field.oninput = () => { d.value = field.value; };
  field.onkeydown = (e) => { e.stopPropagation(); if (e.key === "Enter") save.onclick(); if (e.key === "Escape") { portDraft = { value: null, err: "" }; renderPort(s); } };
  const val = el("div", "val");
  val.append(field, save);
  r.append(who, val);
  box.replaceChildren(r); // swapped whole: the list is never laid out empty
}

// renderGatewayRun: the gateway this app serves, stopped or started from
// Settings (it is what agents' requests go through).
function renderGatewayRun(s) {
  const seg = $("#gwRunSegs");
  if (!seg) return;
  const paint = (running) => {
    seg.replaceChildren(segs([["off", t("Off")], ["on", t("On")]], running ? "on" : "off", async (v) => {
      const out = await api(v === "on" ? "gateway/start" : "gateway/stop", {}).catch((e) => ({ error: e.message }));
      if (!out?.ok) return status(gatewayFixSays(out || {}), "err");
      api("providers").then((r) => { providers = r; paint(!!r?.gateway?.running); }, () => {});
    }));
  };
  api("providers").then((r) => { providers = r; paint(!!r?.gateway?.running); }, () => paint(false));
}

// the restart: the gateway stops and starts again, in the card the
// address and the switch share. Wired once; the page's language follows
// the button's data-t.
$("#gwRestartBtn").onclick = async () => {
  const b = $("#gwRestartBtn");
  if (b.disabled) return;
  b.disabled = true;
  b.textContent = t("Restarting the gateway…");
  const out = await api("gateway/restart", {}).catch((e) => ({ error: e.message }));
  b.disabled = false;
  b.textContent = t("Restart gateway");
  if (!out?.ok) return status(gatewayFixSays(out || {}), "err");
  status(t("Gateway restarted"), "ok", 2500);
  api("providers").then((r) => renderSideGw(r?.gateway), () => {});
  renderGatewayRun(prefs);
};

// the keys and the connect row live under the card: drawn as the settings
// page is (the poll draws it again often, so the keys are read again at
// most every 15s — refreshGatewayKeys)
function renderGatewayKeysAndConnect() {
  if (!providers) return;
  renderGatewayKeys();
  renderConnect();
  if (view === "settings") gatewayKeys === null ? loadGatewayKeys() : refreshGatewayKeys();
}

// renderConnect: the agents' base URL, to copy; five APIs, one address.
function renderConnect() {
  const val = $("#gwConnectVal");
  if (!val || !providers?.gateway) return;
  const url = providers.gateway.url;
  val.replaceChildren(el("code", "", url), copyBtn(url, "Base URL"));
}

// the keys' limits are read again on a timer while the settings page is in
// sight; its coming and going with the view and the window's visibility
document.addEventListener("visibilitychange", gatewayKeysTicks);
