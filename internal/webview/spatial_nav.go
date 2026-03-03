package webview

// spatialNavJS is injected into every page to provide arrow-key spatial
// navigation and Enter-to-activate — a couch-friendly browsing experience.
const spatialNavJS = `
(function() {
  'use strict';

  const FOCUS_COLOR = '#00A4DC';
  const INTERACTIVE = 'a[href], button, input, select, textarea, [role="button"], [role="link"], [role="menuitem"], [role="tab"], [tabindex]';

  let currentEl = null;
  let initialURL = location.href;

  // ── Focus ring ──────────────────────────────────────────────────────
  const ring = document.createElement('div');
  ring.id = '__jc_ring';
  ring.style.cssText = [
    'position:fixed', 'pointer-events:none', 'z-index:2147483647',
    'border:3px solid ' + FOCUS_COLOR, 'border-radius:4px',
    'box-shadow:0 0 0 2px rgba(0,164,220,0.4)',
    'transition:top .15s ease,left .15s ease,width .15s ease,height .15s ease',
    'display:none',
  ].join(';');
  (document.documentElement || document.body).appendChild(ring);

  function showRing(el) {
    if (!el) { ring.style.display = 'none'; return; }
    const r = el.getBoundingClientRect();
    ring.style.top    = (r.top - 3) + 'px';
    ring.style.left   = (r.left - 3) + 'px';
    ring.style.width  = (r.width + 6) + 'px';
    ring.style.height = (r.height + 6) + 'px';
    ring.style.display = 'block';
  }

  // ── Collect visible interactive elements ────────────────────────────
  function getTargets() {
    const els = Array.from(document.querySelectorAll(INTERACTIVE));
    return els.filter(function(el) {
      if (el.offsetParent === null && getComputedStyle(el).position !== 'fixed') return false;
      if (el.disabled) return false;
      const r = el.getBoundingClientRect();
      if (r.width < 4 || r.height < 4) return false;
      if (r.bottom < 0 || r.top > window.innerHeight) return false;
      if (r.right < 0 || r.left > window.innerWidth) return false;
      const s = getComputedStyle(el);
      if (s.visibility === 'hidden' || s.opacity === '0') return false;
      return true;
    });
  }

  // ── Spatial search ──────────────────────────────────────────────────
  // direction: 'up' | 'down' | 'left' | 'right'
  function findNearest(from, direction) {
    const targets = getTargets().filter(function(el) { return el !== from; });
    if (targets.length === 0) return null;

    const fr = from ? from.getBoundingClientRect() : {top:0,bottom:0,left:0,right:0,width:0,height:0};
    const fcx = fr.left + fr.width / 2;
    const fcy = fr.top + fr.height / 2;

    let best = null;
    let bestScore = Infinity;

    for (let i = 0; i < targets.length; i++) {
      const tr = targets[i].getBoundingClientRect();
      const tcx = tr.left + tr.width / 2;
      const tcy = tr.top + tr.height / 2;

      // Filter by direction
      var inDirection = false;
      switch (direction) {
        case 'up':    inDirection = tcy < fcy - 1; break;
        case 'down':  inDirection = tcy > fcy + 1; break;
        case 'left':  inDirection = tcx < fcx - 1; break;
        case 'right': inDirection = tcx > fcx + 1; break;
      }
      if (!inDirection) continue;

      // Distance
      var dx = tcx - fcx;
      var dy = tcy - fcy;
      var dist = Math.sqrt(dx * dx + dy * dy);

      // Angular alignment penalty: prefer elements on the primary axis
      var crossDist;
      if (direction === 'up' || direction === 'down') {
        crossDist = Math.abs(dx);
      } else {
        crossDist = Math.abs(dy);
      }

      var score = dist + crossDist * 2;
      if (score < bestScore) {
        bestScore = score;
        best = targets[i];
      }
    }
    return best;
  }

  // ── Focus management ────────────────────────────────────────────────
  function focusEl(el) {
    if (!el) return;
    currentEl = el;
    el.focus({ preventScroll: true });
    el.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    showRing(el);
  }

  function ensureFocus() {
    if (currentEl && currentEl.isConnected && currentEl.offsetParent !== null) return;
    var targets = getTargets();
    if (targets.length > 0) focusEl(targets[0]);
  }

  // ── Keyboard handler ────────────────────────────────────────────────
  function isTextInput(el) {
    if (!el) return false;
    var tag = el.tagName;
    if (tag === 'TEXTAREA') return true;
    if (tag === 'INPUT') {
      var t = (el.type || '').toLowerCase();
      return t === '' || t === 'text' || t === 'search' || t === 'url' ||
             t === 'email' || t === 'password' || t === 'tel' || t === 'number';
    }
    return el.isContentEditable;
  }

  document.addEventListener('keydown', function(e) {
    // Skip when a text input has focus (allow normal typing)
    if (isTextInput(document.activeElement) && e.key !== 'Escape' && e.key !== 'Backspace') return;

    switch (e.key) {
      case 'ArrowUp':
      case 'ArrowDown':
      case 'ArrowLeft':
      case 'ArrowRight': {
        e.preventDefault();
        e.stopPropagation();
        var dir = e.key.replace('Arrow', '').toLowerCase();
        ensureFocus();
        var next = findNearest(currentEl, dir);
        if (next) focusEl(next);
        break;
      }

      case 'Enter': {
        if (!currentEl) { ensureFocus(); break; }
        e.preventDefault();
        e.stopPropagation();
        currentEl.click();
        break;
      }

      case 'Backspace':
      case 'Escape': {
        e.preventDefault();
        e.stopPropagation();
        // If text input was focused, just blur it
        if (isTextInput(document.activeElement)) {
          document.activeElement.blur();
          break;
        }
        // Navigate back or close
        if (location.href !== initialURL && history.length > 1) {
          history.back();
        } else {
          if (window.__jc_close) window.__jc_close();
        }
        break;
      }
    }
  }, true);

  // ── MutationObserver — re-check focus when DOM changes ──────────────
  var debounceTimer = null;
  var observer = new MutationObserver(function() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(function() {
      showRing(currentEl);
    }, 100);
  });
  observer.observe(document.documentElement, {
    childList: true, subtree: true, attributes: true, attributeFilter: ['style', 'class', 'hidden']
  });

  // ── Window scroll / resize — update ring position ───────────────────
  window.addEventListener('scroll', function() { showRing(currentEl); }, true);
  window.addEventListener('resize', function() { showRing(currentEl); });

  // ── Initial focus ───────────────────────────────────────────────────
  setTimeout(ensureFocus, 500);
})();
`
