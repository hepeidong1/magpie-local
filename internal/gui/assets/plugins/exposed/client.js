// The Routed models page (plugins/exposed): the routing groups page's
// logic over a different list. A row is one name agents ask by; opening
// it draws the editor in its place, where the chain is laid out as the
// group's entries are — a row each, ordered, added by the picker, and
// the whole of one fails before the next is asked. The groups page drew
// this page's shapes: rt-gsec/rt-group/rt-gedit are its classes.
//
// Classic script, loaded before app.js: it registers at once, and its
// mount runs when the page is shown (kernel.js).
magpie.plugin("exposed", (ctx) => {
  ctx.slots.add("page", {
    key: "exposed",
    title: "Routed models",
    html: `
    <div class="rt-gsec">
      <div class="row-head" id="exHead"></div>
      <div class="list rt-groups" id="exList"></div>
    </div>
`,
    mount() {
      const ready = document.readyState === "loading" ? new Promise((go) => document.addEventListener("DOMContentLoaded", go, { once: true })) : Promise.resolve();
      return ready.then(() => exposedPage());
    },
  });
});

function exposedPage() {
  let rows = null;          // the saved list: [{ name, refs: [ref, …] }]
  let exEdit = null;        // { name, draft: { name, refs }, was, isNew } while open
  let gdata = null;         // the groups, for the chain rows' names
  let groupArranging = false; // a row is being dragged: the editor is not drawn under it
  const head = document.getElementById("exHead"), list = document.getElementById("exList");
  if (!head || !list) return;

  async function load() {
    try {
      [rows, gdata] = await Promise.all([api("exposed"), api("groups").catch(() => null)]);
    } catch { rows = []; }
    draw();
  }
  async function save() {
    try {
      rows = await api("exposed/save", { exposed: rows });
      status(t("Saved"), "ok");
      gdata = await api("groups").catch(() => gdata);
    } catch (e) {
      status(e.message, "err");
    }
    draw();
  }
  const groupName = (ref) => {
    if (!ref.startsWith("group/")) return ref;
    const g = (gdata?.groups || []).find((x) => x.id === ref.slice(6));
    return g ? (g.name || g.id) : ref.slice(6);
  };
  const chainSay = (refs) => refs.map(groupName).join(" " + t("then") + " ");

  function draw() {
    const parts = [];
    const newBtn = el("button", "text rt-gnew");
    newBtn.type = "button";
    newBtn.append(svg(PLUS, 11, 1.8), el("span", "", t("New name")));
    newBtn.onclick = () => {
      exEdit = { name: "", draft: { name: "", refs: [] }, was: "", isNew: true };
      draw();
      const first = list.querySelector(".rt-gedit input");
      first?.focus({ preventScroll: true });
    };
    head.replaceChildren(el("span", "label", t("Routed models")), el("span", "grow"), el("span", "note", t("models agents pick as one")), newBtn);
    for (const row of rows || []) {
      const open = exEdit && !exEdit.isNew && exEdit.name === row.name;
      parts.push(open ? editor(row) : rowOf(row));
    }
    if (exEdit?.isNew) parts.push(editor(null));
    if (!parts.length) parts.push(el("div", "none rt-gnone", t("No name yet. New name puts one on the list.")));
    list.replaceChildren(...parts);
  }

  // rowOf is a saved name's row, the group row's shape: name and chain,
  // opened in its place by a click; Remove takes the name off the list,
  // and the row itself is the handle that puts the list in another order
  // — the order /v1/models lists them in, as the Routing page's rows are
  function rowOf(row) {
    const r = el("div", "rt-group");
    r.dataset.id = row.name;
    const main = el("div", "main"), nm = el("div", "nm");
    nm.append(el("b", "", row.name || "—"));
    main.append(nm, el("div", "mem", row.refs?.length ? chainSay(row.refs) : t("nothing yet")));
    // Remove: the name leaves the list, so agents asking by it get nothing
    // (the groups it mapped to stay where they are)
    const rm = el("button", "text danger ex-del", t("Remove"));
    rm.type = "button";
    rm.title = t("Remove {name} from the routed models", { name: row.name });
    rm.onclick = async (e) => {
      e.stopPropagation(); // the row opens the editor; this removes it
      if (!(await confirmAction(t("Remove"), t("Agents asking for {name} will get nothing once it is gone. The groups it mapped to stay as they are.", { name: row.name }), t("Remove")))) return;
      rows = (rows || []).filter((x) => x.name !== row.name);
      if (exEdit && exEdit.name === row.name) exEdit = null;
      save();
    };
    // the grip: the logos are the handle, as a group row's are (#779) —
    // drag the row to move it, Alt+↑/↓ from the keyboard
    const hd = el("button", "ics ag-handle rt-ghandle");
    hd.type = "button";
    hd.tabIndex = -1;
    hd.setAttribute("aria-label", t("Move {name}", { name: row.name }));
    hd.title = t("Drag to reorder · Alt+↑/↓ to move");
    hd.onkeydown = (e) => {
      if (!e.altKey || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) return;
      e.preventDefault();
      e.stopPropagation();
      moveRow(row.name, e.key === "ArrowUp" ? -1 : 1);
    };
    hd.onpointerdown = (e) => {
      if (groupArranging) return;
      const live = [...list.children].filter((x) => x.classList.contains("rt-group"));
      if (live.length < 2) return;
      groupArranging = true;
      // commit gets where the row landed among them: the list is reordered
      // by index, the names being the rows' own
      dragRows(e, hd, r, list, live, (to) => moveTo(row.name, to), () => {}, () => { groupArranging = false; });
    };
    hd.onclick = (e) => e.stopPropagation(); // the grip moves; the row opens the editor
    r.append(hd, main, rm);
    r.onclick = () => {
      exEdit = { name: row.name, draft: { name: row.name, refs: [...row.refs] }, was: JSON.stringify(row), isNew: false };
      draw();
    };
    return r;
  }

  // moveRow puts the name by n places in the list, saving it as it goes:
  // the order is the list's own, and /v1/models follows it
  function moveRow(name, n) {
    const at = (rows || []).findIndex((x) => x.name === name);
    if (at < 0 || n === 0) return;
    moveTo(name, at + n);
  }

  // moveTo puts the name at place to, the rest keeping their order
  function moveTo(name, to) {
    const at = (rows || []).findIndex((x) => x.name === name);
    if (at < 0) return;
    to = Math.max(0, Math.min(rows.length - 1, to));
    if (to === at) return;
    const [moved] = rows.splice(at, 1);
    rows.splice(to, 0, moved);
    save();
  }

  // editor is the name's editor, drawn in the row's place: its name, and
  // the chain as the group's entries are — one row each, ordered
  function editor(row) {
    const d = exEdit.draft;
    if (!Array.isArray(d.refs)) d.refs = [];
    const ed = el("div", "editor rt-gedit");
    ed.dataset.id = exEdit.name;
    const h = el("div", "ehead");
    h.append(el("b", "", row ? row.name : t("New name")));
    // an open editor folds back into its row from its heading, while
    // nothing in it has changed
    if (!exEdit.isNew) {
      h.classList.add("folds");
      h.setAttribute("role", "button");
      h.tabIndex = 0;
      h.title = t("Click to fold it");
      const fold = () => {
        if (JSON.stringify(d) !== exEdit.was) return status(t("Save or Cancel your changes first"), "warn");
        exEdit = null;
        draw();
      };
      h.onclick = fold;
      h.onkeydown = (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); fold(); } };
    }
    ed.append(h);
    const keys = (i) => { i.onkeydown = (e) => { e.stopPropagation(); if (e.key === "Escape") { cancel(); } }; return i; };
    const nameIn = keys(input(d.name, t("a model name")));
    nameIn.setAttribute("aria-label", t("The name an agent asks by"));
    nameIn.oninput = () => { d.name = nameIn.value; };
    const nameWrap = el("div");
    nameWrap.append(nameIn, el("div", "hint", t("one word of letters, digits, dots and dashes — what an agent asks by")));
    ed.append(el("label", "", t("Name")), nameWrap);

    // the chain's rows: order number, the target, its place controls
    const box = el("div", "ex-chain");
    let addBtn, adds;
    const drawChain = () => {
      box.replaceChildren();
      d.refs.forEach((ref, i) => {
        const line = el("div", "ex-mrow");
        line.append(el("span", "n", String(i + 1)));
        line.append(el("span", "nm2", groupName(ref) + (ref.startsWith("group/") ? "" : " (" + t("entry") + ")")));
        const up = el("button", "icon ex-up");
        up.type = "button";
        up.title = t("Try it sooner");
        up.append(svg(CHEV, 11, 1.6));
        up.style.transform = "rotate(180deg)";
        up.disabled = i === 0;
        up.onclick = () => { [d.refs[i - 1], d.refs[i]] = [d.refs[i], d.refs[i - 1]]; drawChain(); };
        const down = el("button", "icon ex-up");
        down.type = "button";
        down.title = t("Try it later");
        down.append(svg(CHEV, 11, 1.6));
        down.disabled = i === d.refs.length - 1;
        down.onclick = () => { [d.refs[i + 1], d.refs[i]] = [d.refs[i], d.refs[i + 1]]; drawChain(); };
        const rm = el("button", "icon ex-rm");
        rm.type = "button";
        rm.title = t("Remove");
        rm.append(svg("M4 4l8 8M12 4l-8 8", 10, 1.8));
        rm.onclick = () => { d.refs.splice(i, 1); drawChain(); addBtn?.querySelector("span")?.replaceChildren(document.createTextNode(t(d.refs.length ? "Add another" : "Add groups"))); };
        line.append(el("span", "grow"), up, down, rm);
        box.append(line);
      });
      if (!d.refs.length) box.append(el("div", "hint", t("Nothing in the chain yet: Add groups puts the first one on.")));
      if (adds) box.append(adds);
    };
    const chainWrap = el("div");
    chainWrap.append(box);
    ed.append(el("label", "", t("Chain")), chainWrap, el("div", "hint", t("The chain tried in turn: the whole of one fails before the next is asked")));

    adds = el("div", "rt-madds");
    addBtn = el("button", "text rt-gnew");
    addBtn.type = "button";
    addBtn.append(svg(PLUS, 11, 1.8), el("span", "", t(d.refs.length ? "Add another" : "Add groups")));
    addBtn.onclick = (ev) => {
      const gs1 = (gdata?.groups || []).filter((g) => !g.hidden && !g.auto);
      const options = gs1.map((g) => ({ value: g.id, label: g.name || g.id, note: "group/" + g.id, ref: "group/" + g.id }));
      openPicker({ id: "", name: "", fields: [] }, { key: "group", label: "group", placeholder: t("Filter groups…"), value: "", options, multi: true,
        picked: (id) => d.refs.includes("group/" + id),
        onPick: (id) => {
          if (!id) return;
          const i = d.refs.indexOf("group/" + id);
          if (i >= 0) d.refs.splice(i, 1);
          else d.refs.push("group/" + id);
          addBtn.querySelector("span").textContent = t(d.refs.length ? "Add another" : "Add groups");
          drawChain();
        } }, addBtn, ev);
    };
    adds.append(addBtn);
    drawChain();

    const bar = el("div", "bar");
    const saveBtn = el("button", "text primary", t("Save"));
    saveBtn.type = "button";
    saveBtn.onclick = doSave;
    const done = el("button", "text", t("Done"));
    done.type = "button";
    done.onclick = cancel;
    bar.append(el("span", "grow"), saveBtn, done);
    ed.append(bar);

    function cancel() { exEdit = null; draw(); }
    function doSave() {
      const name = d.name.trim();
      if (!name) { nameIn.focus({ preventScroll: true }); return status(t("a model name is one word"), "warn"); }
      if (!d.refs.length) return status(t("Nothing in the chain yet: Add groups puts the first one on."), "warn");
      if (exEdit.isNew) {
        rows ||= [];
        if (rows.some((r) => r.name.toLowerCase() === name.toLowerCase())) return status(t("the name is on the list twice"), "err");
        rows.push({ name, refs: [...d.refs] });
      } else {
        const row = (rows || []).find((r) => r.name === exEdit.name);
        if (!row) return cancel();
        row.name = name;
        row.refs = [...d.refs];
      }
      exEdit = null;
      save();
    }
    return ed;
  }
  // shown again, the groups are asked for again: a group renamed on the
  // groups page said its old name here until a restart otherwise
  const view = document.getElementById("view-exposed");
  if (view) {
    new MutationObserver(() => {
      if (view.hidden) return;
      if (exEdit) { api("groups").then((g) => { gdata = g; draw(); }).catch(() => {}); return; }
      load();
    }).observe(view, { attributes: true, attributeFilter: ["hidden"] });
  }
  load();
}
