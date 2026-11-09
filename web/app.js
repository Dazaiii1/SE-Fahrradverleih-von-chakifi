let cfg = {}, idTypes = [], returnId = null;
const $ = s => document.querySelector(s);
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const eur = n => n.toFixed(2).replace('.', ',') + ' €';
const fmt = d => new Date(d).toLocaleString('de-AT', {dateStyle:'short', timeStyle:'short'});
const fmtDate = d => new Date(d + 'T00:00:00').toLocaleDateString('de-AT');

async function api(path, method = 'GET', body) {
  const res = await fetch('/api' + path, {method, headers: {'Content-Type':'application/json'}, body: body ? JSON.stringify(body) : undefined});
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Fehler');
  return data;
}
function msg(text, err) {
  const m = $('#msg'); m.textContent = text; m.className = 'msg' + (err ? ' err' : ''); m.hidden = false;
  setTimeout(() => m.hidden = true, 5000);
}
const run = fn => async (...a) => { try { await fn(...a); } catch (e) { msg(e.message, true); } };
const readFile = f => new Promise((ok, no) => { const r = new FileReader(); r.onload = () => ok(r.result); r.onerror = no; r.readAsDataURL(f); });
async function fileField(input, maxBytes = 2e6) {
  const f = input.files[0];
  if (!f) return {};
  if (f.size > maxBytes) throw new Error('Datei zu groß (max. 2 MB)');
  return {data: await readFile(f), name: f.name};
}

function show(view) {
  if (!document.querySelector('#view-' + view)) view = 'dashboard';
  document.querySelectorAll('main > section').forEach(s => s.hidden = s.id !== 'view-' + view);
  document.querySelectorAll('nav a').forEach(a => a.classList.toggle('active', a.dataset.view === view));
  ({dashboard: loadDashboard, items: loadItems, persons: loadPersons, rentals: loadRentals, vouchers: loadVouchers})[view]();
}
const route = () => show((location.hash || '#dashboard').slice(1));

async function loadBanner() {
  const v = (await api('/vouchers')).find(x => x.featured);
  if (!v) return;
  $('#banner-text').innerHTML = `${esc(v.emoji)} <b>${esc(v.tagline)}</b> – Code <code>${esc(v.code)}</code>`;
  $('#banner').hidden = false;
}

async function loadDashboard() {
  const s = await api('/dashboard');
  $('#tiles').innerHTML = [
    ['t1', 'Verfügbare ' + cfg.itemPlural, s.available], ['t2', 'Ausgeliehene ' + cfg.itemPlural, s.rented],
    ['t3', 'Gesamt ' + cfg.itemPlural, s.total], ['t4', 'Registrierte Personen', s.persons],
    ['t4', 'Lädt gerade', s.charging], ['t5', 'Überfällig', s.overdue], ['t6', 'Außer Betrieb', s.outOfService], ['t1', 'Umsatz', eur(s.revenue)],
  ].map(([c, l, v]) => `<div class="tile ${c}">${esc(l)}<b>${esc(v)}</b></div>`).join('');
}

async function loadVouchers() {
  const vs = await api('/vouchers');
  $('#vouchers').innerHTML = vs.map((v, i) => `<article class="voucher v${i % 3}">
    <div class="v-emoji">${esc(v.emoji)}</div><div class="v-pct">${v.percent} %</div>
    <h3>${esc(v.title)}</h3><p class="v-tag">${esc(v.tagline)}</p><p>${esc(v.description)}</p>
    <div class="v-code"><code>${esc(v.code)}</code><button class="btn gray" data-copy="${esc(v.code)}">Code kopieren</button></div></article>`).join('');
}

async function loadItems() {
  const items = await api('/items');
  const label = {available: ['ok', 'verfügbar'], rented: ['busy', 'ausgeliehen'], charging: ['charge', 'lädt'], outOfService: ['oos', 'außer Betrieb']};
  $('#items-table').innerHTML = '<tr><th>Nr.</th><th>Bezeichnung</th><th>Kategorie</th><th>Preis (Basis bis max.)</th><th>Akku</th><th>Status</th><th></th></tr>' +
    items.map(i => {
      const [cls, txt] = label[i.status];
      const dmg = i.damage ? `<br><small>Schaden: ${esc(i.damage.description || 'keine Beschreibung')}${i.damage.hasPhoto ? ` – <a href="/api/items/${encodeURIComponent(i.nr)}/damage-photo" target="_blank" rel="noopener">Foto</a>` : ''}</small>` : '';
      const review = i.outOfService ? `<button class="btn gray" data-review="${esc(i.nr)}">Prüfung abgeschlossen</button> ` : '';
      const bat = i.hasBattery ? `<meter min="0" max="100" low="21" high="60" optimum="100" value="${Math.round(i.battery)}"></meter> ${Math.round(i.battery)} %` : '–';
      return `<tr><td>${esc(i.nr)}</td><td>${esc(i.name)}</td><td>${esc(i.type)}</td>
        <td>${eur(i.rate)} – ${eur(i.maxRate)} / Std.</td><td>${bat}</td>
        <td><span class="badge ${cls}">${txt}</span>${dmg}</td>
        <td>${review}<button class="btn red" data-del-item="${esc(i.nr)}">Löschen</button></td></tr>`;
    }).join('');
}

async function loadPersons() {
  const ps = await api('/persons');
  $('#persons-table').innerHTML = '<tr><th>ID</th><th>Name</th><th>Geburtstag</th><th>Ausweis</th><th>Nummer</th><th>Scan</th><th></th></tr>' +
    ps.map(p => `<tr><td>${p.id}</td><td>${esc(p.name)}</td><td>${esc(fmtDate(p.birthday))}</td><td>${esc(p.idType)}</td><td>${esc(p.idNumber)}</td>
      <td>${p.hasScan ? `<a href="/api/persons/${p.id}/id-scan" target="_blank" rel="noopener">Ansehen</a>` : '–'}</td>
      <td><button class="btn red" data-del-person="${p.id}">Löschen</button></td></tr>`).join('');
}

async function loadRentals() {
  const [cats, persons, rentals] = await Promise.all([api('/categories'), api('/persons'), api('/rentals')]);
  const keepCat = $('#r-cat').value, keepPerson = $('#r-person').value;
  $('#r-cat').innerHTML = cats.categories.map(c => `<option value="${esc(c.name)}"${c.available ? '' : ' disabled'}>${esc(c.name)} – ${eur(c.currentRate)} / Std. · Kaution ${eur(c.deposit)} · ${c.available} frei${c.charging ? ` (${c.charging} lädt)` : ''}</option>`).join('');
  if (!$('#r-acc').children.length) $('#r-acc').innerHTML = cats.accessories.map(a => `<label class="check"><input type="checkbox" value="${esc(a.code)}"> ${esc(a.name)} (+${eur(a.deposit)} Kaution)</label>`).join('');
  $('#r-person').innerHTML = persons.map(p => `<option value="${p.id}">${esc(p.name)} (${esc(p.idType)} ${esc(p.idNumber)})</option>`).join('')
    || '<option value="">Zuerst Person anlegen</option>';
  if (keepCat) $('#r-cat').value = keepCat;
  if (keepPerson) $('#r-person').value = keepPerson;
  if (!$('#r-start').value) { const d = new Date(Date.now() - new Date().getTimezoneOffset() * 60000); $('#r-start').value = d.toISOString().slice(0, 16); }
  updateQuote();
  const now = Date.now();
  $('#rentals-table').innerHTML = '<tr><th>#</th><th>Fahrrad</th><th>Person</th><th>Beginn</th><th>Dauer</th><th>Preis / Kaution</th><th>Status</th><th></th></tr>' +
    rentals.map(r => {
      const late = !r.returnedAt && now > new Date(r.start).getTime() + r.hours * 3600000;
      const st = r.returnedAt ? '<span class="badge done">zurück</span>' : late ? '<span class="badge late">überfällig</span>' : '<span class="badge busy">aktiv</span>';
      const disc = r.voucher ? `<br><small>${esc(r.voucher)} −${r.discountPercent} %</small>` : '';
      const acc = r.accessories?.length ? ` <small>(${esc(r.accessories.join(', '))})</small>` : '';
      const dep = `<br><small>Kaution ${eur(r.deposit)}${r.returnedAt ? (r.depositReturned ? ' – erstattet' : ' – einbehalten') : ''}</small>`;
      return `<tr><td>${r.id}</td><td>${esc(r.itemNr)} ${esc(r.itemName)}${acc}</td><td>${esc(r.personName)}</td><td>${fmt(r.start)}</td>
        <td>${r.hours} Std.</td><td>${eur(r.price)}${disc}${dep}</td><td>${st}</td>
        <td>${r.returnedAt ? '' : `<button class="btn gray" data-return="${r.id}" data-label="${esc(r.itemNr)}">Zurückgeben</button>`}</td></tr>`;
    }).join('');
}

let quoteTimer;
function updateQuote() { clearTimeout(quoteTimer); quoteTimer = setTimeout(doQuote, 200); }
async function doQuote() {
  const el = $('#price-preview'); el.className = 'hint';
  const accessories = [...document.querySelectorAll('#r-acc input:checked')].map(x => x.value);
  const body = {category: $('#r-cat').value, personId: +$('#r-person').value, hours: +$('#r-hours').value, voucher: $('#r-voucher').value.trim(), accessories};
  if (!body.category || !body.personId || !body.hours || !$('#r-start').value) { el.textContent = ''; return; }
  body.start = new Date($('#r-start').value).toISOString();
  try {
    const q = await api('/quote', 'POST', body);
    const lines = [`${q.category}: ${eur(q.total)}`, `${q.units} Std. × ${eur(q.rate)}`];
    if (q.factor > 1.0001) lines.push(`Auslastungsaufschlag ×${q.factor.toFixed(2).replace('.', ',')} (max. ×1,50)`);
    if (q.voucher) lines.push(`Gutschein ${q.voucher}: −${q.discountPercent} % (−${eur(q.discountAmount)})`);
    lines.push(`Kaution: ${eur(q.deposit)} (wird bei der Rückgabe erstattet)`);
    el.innerHTML = lines.map(esc).join('<br>');
  } catch (e) { el.className = 'hint err'; el.textContent = e.message; }
}

async function init() {
  const c = await api('/config'); cfg = c.config; idTypes = c.idTypes;
  document.title = cfg.appName; $('#brand').textContent = cfg.appName; $('#foot-name').textContent = cfg.appName;
  $('#nav-items').textContent = cfg.itemPlural + '-Verwaltung';
  $('#q-items').textContent = cfg.itemPlural + ' verwalten';
  $('#items-title').textContent = cfg.itemPlural + '-Verwaltung';
  $('#item-type').innerHTML = cfg.types.map(t => `<option>${esc(t)}</option>`).join('');
  $('#id-type').innerHTML = idTypes.map(t => `<option>${esc(t)}</option>`).join('');
  $('#item-form [name=nr]').placeholder = `Nr. (z.B. ${cfg.nrPrefix}-100)`;
  loadBanner();

  document.body.addEventListener('click', run(async e => {
    const t = e.target, d = t.dataset;
    if (d.go) location.hash = d.go;
    if (d.delItem && confirm('Wirklich löschen?')) { await api('/items/' + encodeURIComponent(d.delItem), 'DELETE'); loadItems(); }
    if (d.delPerson && confirm('Wirklich löschen?')) { await api('/persons/' + d.delPerson, 'DELETE'); loadPersons(); }
    if (d.review) { await api(`/items/${encodeURIComponent(d.review)}/review-done`, 'POST'); msg('Wieder im Betrieb'); loadItems(); }
    if (d.copy) { await navigator.clipboard.writeText(d.copy); msg('Code kopiert: ' + d.copy); }
    if (d.return) {
      returnId = d.return; $('#return-title').textContent = 'Rückgabe ' + d.label;
      $('#rf-damage').checked = false; $('#rf-fields').hidden = true; $('#rf-desc').value = ''; $('#rf-photo').value = '';
      $('#return-dialog').showModal();
    }
  }));
  $('#rf-damage').addEventListener('change', e => $('#rf-fields').hidden = !e.target.checked);
  $('#rf-cancel').addEventListener('click', () => $('#return-dialog').close());
  $('#return-form').addEventListener('submit', run(async e => {
    e.preventDefault();
    const body = {};
    if ($('#rf-damage').checked) {
      const f = await fileField($('#rf-photo'));
      body.report = {description: $('#rf-desc').value, photo: f.data || '', photoName: f.name || ''};
    }
    await api(`/rentals/${returnId}/return`, 'POST', body);
    $('#return-dialog').close(); msg(body.report ? 'Zurückgegeben – Artikel außer Betrieb (Prüfung)' : 'Zurückgegeben'); loadRentals();
  }));
  $('#item-form').addEventListener('submit', run(async e => {
    e.preventDefault(); const f = Object.fromEntries(new FormData(e.target));
    await api('/items', 'POST', f); e.target.reset(); msg('Hinzugefügt'); loadItems();
  }));
  $('#person-form').addEventListener('submit', run(async e => {
    e.preventDefault(); const fd = new FormData(e.target);
    const body = {name: fd.get('name'), birthday: fd.get('birthday'), idType: fd.get('idType'), idNumber: fd.get('idNumber')};
    const f = await fileField(e.target.querySelector('[name=scan]'));
    if (f.data) { body.scan = f.data; body.scanName = f.name; }
    await api('/persons', 'POST', body); e.target.reset(); msg('Person angelegt'); loadPersons();
  }));
  $('#rental-form').addEventListener('submit', run(async e => {
    e.preventDefault(); const f = Object.fromEntries(new FormData(e.target));
    await api('/rentals', 'POST', {category: f.category, personId: +f.personId, hours: +f.hours, voucher: f.voucher.trim(), start: new Date(f.start).toISOString(), accessories: [...document.querySelectorAll('#r-acc input:checked')].map(x => x.value)});
    $('#r-voucher').value = ''; msg('Ausleihe angelegt'); loadRentals();
  }));
  ['#r-cat', '#r-person', '#r-start', '#r-acc'].forEach(s => $(s).addEventListener('change', updateQuote));
  setInterval(() => { if (!$('#view-items').hidden) loadItems(); if (!$('#view-dashboard').hidden) loadDashboard(); }, 10000);
  ['#r-hours', '#r-voucher'].forEach(s => $(s).addEventListener('input', updateQuote));
  window.addEventListener('hashchange', route);
  route();
}
init().catch(e => msg(e.message, true));
