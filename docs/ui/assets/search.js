// ==========================================
// FULL-TEXT SEARCH PALETTE (Cmd/Ctrl-K)
// Self-contained: injects its own CSS + trigger button, so wiring it in only
// needs this one <script> tag. Fetches assets/search-index.json (built by the
// Go side) once on first open and ranks client-side. Navigates by setting the
// docs hash, which app.js already routes to the right doc + section.
// ==========================================
(function () {
  var INDEX = null;      // loaded lazily
  var loading = false;
  var results = [];
  var active = 0;

  // --- styling (kept here so search.js is a drop-in; Phase 3 CSS can override) ---
  var css = `
  .search-trigger{display:inline-flex;align-items:center;gap:.5rem;background:#0a0a0a;border:1px solid #262626;color:#a1a1a1;border-radius:8px;padding:.4rem .7rem;font:inherit;font-size:.85rem;cursor:pointer}
  .search-trigger:hover{border-color:#3b82f6;color:#ededed}
  .search-trigger kbd{border:1px solid #262626;border-radius:4px;padding:0 .35rem;font-size:.75rem;color:#737373}
  .search-overlay{position:fixed;inset:0;background:rgba(0,0,0,.6);backdrop-filter:blur(2px);display:none;z-index:1000;align-items:flex-start;justify-content:center}
  .search-overlay.open{display:flex}
  .search-box{margin-top:10vh;width:min(640px,92vw);background:#0a0a0a;border:1px solid #262626;border-radius:12px;overflow:hidden;box-shadow:0 20px 60px rgba(0,0,0,.5)}
  .search-box input{width:100%;box-sizing:border-box;background:transparent;border:0;border-bottom:1px solid #262626;color:#ededed;font:inherit;font-size:1rem;padding:1rem 1.1rem;outline:none}
  .search-results{max-height:60vh;overflow-y:auto;margin:0;padding:.4rem;list-style:none}
  .search-results li{padding:.6rem .8rem;border-radius:8px;cursor:pointer}
  .search-results li.active,.search-results li:hover{background:#171717}
  .search-crumb{font-size:.75rem;color:#3b82f6;margin-bottom:.2rem}
  .search-crumb .sep{color:#525252;margin:0 .35rem}
  .search-snippet{font-size:.85rem;color:#a1a1a1;overflow:hidden;text-overflow:ellipsis;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical}
  .search-snippet mark{background:rgba(59,130,246,.25);color:#ededed;border-radius:2px}
  .search-empty{padding:1.2rem;color:#737373;font-size:.9rem;text-align:center}
  `;
  var style = document.createElement('style');
  style.textContent = css;
  document.head.appendChild(style);

  // --- DOM ---
  var overlay = document.createElement('div');
  overlay.className = 'search-overlay';
  overlay.innerHTML =
    '<div class="search-box">' +
    '<input type="text" placeholder="Search the docs..." aria-label="Search docs" autocomplete="off" spellcheck="false">' +
    '<ul class="search-results"></ul>' +
    '</div>';
  var input = overlay.querySelector('input');
  var list = overlay.querySelector('.search-results');

  function isMac() { return navigator.platform.toUpperCase().indexOf('MAC') >= 0; }

  var trigger = document.createElement('button');
  trigger.className = 'search-trigger';
  trigger.type = 'button';
  trigger.innerHTML = 'Search <kbd>' + (isMac() ? '⌘K' : 'Ctrl K') + '</kbd>';

  document.addEventListener('DOMContentLoaded', function () {
    document.body.appendChild(overlay);
    var host = document.querySelector('.nav-tabs') || document.querySelector('.header-left');
    if (host) host.parentNode.insertBefore(trigger, host.nextSibling);
    trigger.addEventListener('click', open);
  });

  // --- index loading ---
  async function ensureIndex() {
    if (INDEX || loading) return;
    loading = true;
    try {
      var url = (typeof buildUrl === 'function') ? buildUrl('assets/search-index.json') : 'assets/search-index.json';
      var res = await fetch(url);
      INDEX = res.ok ? await res.json() : [];
    } catch (e) {
      INDEX = [];
      console.error('search index load failed', e);
    }
    loading = false;
  }

  // --- ranking ---
  function escapeRe(s) { return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'); }

  function score(rec, terms) {
    var s = 0;
    var heading = (rec.heading || '').toLowerCase();
    var title = (rec.docTitle || '').toLowerCase();
    var text = (rec.text || '').toLowerCase();
    for (var i = 0; i < terms.length; i++) {
      var t = terms[i];
      if (!t) continue;
      var inSomething = false;
      if (heading.indexOf(t) >= 0) { s += 10; inSomething = true; }
      if (title.indexOf(t) >= 0) { s += 4; inSomething = true; }
      var idx = text.indexOf(t), hits = 0;
      while (idx >= 0 && hits < 5) { s += 2; hits++; inSomething = true; idx = text.indexOf(t, idx + t.length); }
      if (!inSomething) return 0; // every term must appear somewhere (AND)
    }
    return s;
  }

  function snippet(rec, terms) {
    var text = rec.text || '';
    var low = text.toLowerCase();
    var at = -1;
    for (var i = 0; i < terms.length; i++) { var p = low.indexOf(terms[i]); if (p >= 0 && (at < 0 || p < at)) at = p; }
    if (at < 0) at = 0;
    var start = Math.max(0, at - 50);
    var frag = text.slice(start, start + 160);
    if (start > 0) frag = '…' + frag;
    frag = frag.replace(/[&<>]/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]; });
    if (terms.length) {
      var re = new RegExp('(' + terms.map(escapeRe).join('|') + ')', 'ig');
      frag = frag.replace(re, '<mark>$1</mark>');
    }
    return frag;
  }

  function runSearch(q) {
    var terms = q.toLowerCase().split(/\s+/).filter(Boolean);
    if (!terms.length || !INDEX) { results = []; render(); return; }
    var scored = [];
    for (var i = 0; i < INDEX.length; i++) {
      var sc = score(INDEX[i], terms);
      if (sc > 0) scored.push({ rec: INDEX[i], score: sc, terms: terms });
    }
    scored.sort(function (a, b) { return b.score - a.score; });
    results = scored.slice(0, 30);
    active = 0;
    render();
  }

  function render() {
    if (!input.value.trim()) { list.innerHTML = ''; return; }
    if (!results.length) { list.innerHTML = '<div class="search-empty">No results</div>'; return; }
    list.innerHTML = results.map(function (r, i) {
      var rec = r.rec;
      var crumb = escapeHtml(rec.docTitle || rec.file);
      if (rec.heading) crumb += '<span class="sep">›</span>' + escapeHtml(rec.heading);
      return '<li data-i="' + i + '" class="' + (i === active ? 'active' : '') + '">' +
        '<div class="search-crumb">' + crumb + '</div>' +
        '<div class="search-snippet">' + snippet(rec, r.terms) + '</div></li>';
    }).join('');
    Array.prototype.forEach.call(list.querySelectorAll('li[data-i]'), function (li) {
      li.addEventListener('click', function () { go(parseInt(li.dataset.i, 10)); });
    });
  }

  function escapeHtml(s) { return (s || '').replace(/[&<>]/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]; }); }

  function go(i) {
    var r = results[i];
    if (!r) return;
    var rec = r.rec;
    close();
    window.location.hash = 'docs:' + rec.file + (rec.headingId ? ':' + rec.headingId : '');
  }

  // --- open/close + keys ---
  async function open() {
    overlay.classList.add('open');
    input.value = '';
    list.innerHTML = '';
    input.focus();
    await ensureIndex();
  }
  function close() { overlay.classList.remove('open'); }

  input.addEventListener('input', function () { runSearch(input.value); });
  overlay.addEventListener('click', function (e) { if (e.target === overlay) close(); });

  input.addEventListener('keydown', function (e) {
    if (e.key === 'ArrowDown') { e.preventDefault(); active = Math.min(active + 1, results.length - 1); render(); scrollActive(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); active = Math.max(active - 1, 0); render(); scrollActive(); }
    else if (e.key === 'Enter') { e.preventDefault(); go(active); }
    else if (e.key === 'Escape') { close(); }
  });

  function scrollActive() {
    var el = list.querySelector('li.active');
    if (el) el.scrollIntoView({ block: 'nearest' });
  }

  document.addEventListener('keydown', function (e) {
    var typing = /^(INPUT|TEXTAREA|SELECT)$/.test((e.target.tagName || '')) || e.target.isContentEditable;
    if ((e.key === 'k' || e.key === 'K') && (e.metaKey || e.ctrlKey)) { e.preventDefault(); overlay.classList.contains('open') ? close() : open(); }
    else if (e.key === '/' && !typing && !overlay.classList.contains('open')) { e.preventDefault(); open(); }
  });
})();
