// ── Catalog definitions ─────────────────────────────────────────────────────
// Cores alone don't determine a plan: the same core count can map to
// different RAM/disk/price combos depending on location and CPU family, so
// every step below is derived by filtering the live catalog, never by index
// arithmetic against a fixed table.

const TYPE_DEFS = [
  { key: 'shared-amd',    arch: 'x86', cpuType: 'shared',    label: 'Shared vCPU',    badge: ['AMD', 'EPYC'], coreLabel: 'AMD EPYC™ (vCPU cores)' },
  { key: 'shared-arm',    arch: 'arm', cpuType: 'shared',    label: 'Shared ARM',     badge: ['ARM', '64'],   coreLabel: 'ARM64 (vCPU cores)' },
  { key: 'dedicated-amd', arch: 'x86', cpuType: 'dedicated', label: 'Dedicated vCPU', badge: ['AMD', 'EPYC'], coreLabel: 'AMD EPYC™ (dedicated cores)' },
];

const LOCATIONS = [
  { key: 'DE', label: 'Germany',   flag: '🇩🇪' },
  { key: 'FI', label: 'Finland',   flag: '🇫🇮' },
  { key: 'US', label: 'USA',       flag: '🇺🇸' },
  { key: 'SG', label: 'Singapore', flag: '🇸🇬' },
];

function typeDef(key) {
  return TYPE_DEFS.find(t => t.key === key);
}

// ── Data loading ─────────────────────────────────────────────────────────────
// Single seam: every other function only ever consumes the resolved array,
// so this is the one place that knows where the catalog actually comes from.
function loadPlans() {
  return fetch('/api/plans').then(r => {
    if (!r.ok) throw new Error(`GET /api/plans: ${r.status}`);
    return r.json();
  });
}

// ── App state ────────────────────────────────────────────────────────────────

let enabledPlans = [];
let maxCores = 1;
let maxRam = 1;

let state = {
  location: null,
  type: null,
  cores: null,
  ram: null,
  disk: null,
};

// ── Catalog filter helpers (operate only on the enabled subset) ────────────

function matchesType(plan, typeKey) {
  const t = typeDef(typeKey);
  return !!t && plan.arch === t.arch && plan.cpuType === t.cpuType;
}

function getLocations(plans) {
  const present = new Set(plans.map(p => p.location));
  return LOCATIONS.map(l => l.key).filter(k => present.has(k));
}

function getCitiesForLocation(plans, loc) {
  return [...new Set(plans.filter(p => p.location === loc).map(p => p.city))];
}

function getTypesForLocation(plans, loc) {
  return TYPE_DEFS
    .filter(t => plans.some(p => p.location === loc && matchesType(p, t.key)))
    .map(t => t.key);
}

function getCoresOptions(plans, loc, type) {
  return [...new Set(
    plans.filter(p => p.location === loc && matchesType(p, type)).map(p => p.cores)
  )].sort((a, b) => a - b);
}

function getRamOptions(plans, loc, type, cores) {
  return [...new Set(
    plans.filter(p => p.location === loc && matchesType(p, type) && p.cores === cores).map(p => p.ram)
  )].sort((a, b) => a - b);
}

function getDiskOptions(plans, loc, type, cores, ram) {
  return [...new Set(
    plans.filter(p => p.location === loc && matchesType(p, type) && p.cores === cores && p.ram === ram).map(p => p.disk)
  )].sort((a, b) => a - b);
}

function findPlan(plans, loc, type, cores, ram, disk) {
  return plans.find(p =>
    p.location === loc && matchesType(p, type) &&
    p.cores === cores && p.ram === ram && p.disk === disk
  );
}

// ── Selection resolution / clamping ─────────────────────────────────────────
// Every change funnels through here: it re-derives each dimension's valid
// options from everything chosen upstream and snaps any now-invalid value
// (whether from a stale URL, a garbage param, or an upstream change) to the
// nearest real option in the live catalog.

function pickValid(desired, options) {
  if (!options.length) return undefined;
  return options.includes(desired) ? desired : options[0];
}

function pickNearest(desired, options) {
  if (!options.length) return undefined;
  if (typeof desired === 'number' && !Number.isNaN(desired)) {
    if (options.includes(desired)) return desired;
    return options.reduce((best, v) => Math.abs(v - desired) < Math.abs(best - desired) ? v : best, options[0]);
  }
  return options[0];
}

function resolveSelection(desired) {
  const location = pickValid(desired.location, getLocations(enabledPlans));
  const type     = pickValid(desired.type, getTypesForLocation(enabledPlans, location));
  const cores    = pickNearest(desired.cores, getCoresOptions(enabledPlans, location, type));
  const ram      = pickNearest(desired.ram, getRamOptions(enabledPlans, location, type, cores));
  const disk     = pickNearest(desired.disk, getDiskOptions(enabledPlans, location, type, cores, ram));
  return { location, type, cores, ram, disk };
}

// ── DOM refs ─────────────────────────────────────────────────────────────────

const locationTabsEl = document.getElementById('location-tabs');
const locationCityEl = document.getElementById('location-city');
const typeTabsEl     = document.getElementById('type-tabs');

const sliderCores = document.getElementById('slider-cores');
const sliderRam   = document.getElementById('slider-ram');
const sliderDisk  = document.getElementById('slider-disk');

const ticksCores = document.getElementById('ticks-cores');
const ticksRam   = document.getElementById('ticks-ram');
const ticksDisk  = document.getElementById('ticks-disk');

const coresCardLabelEl = document.getElementById('cores-card-label');

const priceDisplay = document.getElementById('price-display');
const orderBtn     = document.getElementById('order-btn');
const toast        = document.getElementById('toast');
const toastUrl     = document.getElementById('toast-url');
const emptyStateEl = document.getElementById('empty-state');

const coreLabelEl = document.getElementById('core-label');
const ramBars     = document.querySelectorAll('.ram-bar');
const serverCore  = document.getElementById('server-core');
const badgeLine1  = document.getElementById('badge-line1');
const badgeLine2  = document.getElementById('badge-line2');

// ── Slider helpers ───────────────────────────────────────────────────────────
// Each slider's value is an index into that step's *current* option array
// (recomputed on every render), not a raw magnitude — this keeps stops evenly
// spaced regardless of gaps between real values (e.g. 1 / 2 / 4 / 8 / 12 / 16).

function updateFill(slider) {
  const max = Number(slider.max) || 1;
  const pct = (slider.value / max) * 100;
  const stop = `calc(${pct}% + ${11 - pct * 0.22}px)`;
  slider.style.background = `linear-gradient(to right, #d4a017 ${stop}, #333 ${stop})`;
}

function configureSlider(sliderEl, ticksEl, options, value) {
  sliderEl.min = 0;
  sliderEl.max = Math.max(options.length - 1, 0);
  sliderEl.step = 1;
  sliderEl.disabled = options.length <= 1;
  const idx = Math.max(options.indexOf(value), 0);
  sliderEl.value = idx;
  updateFill(sliderEl);

  ticksEl.replaceChildren(...options.map(v => {
    const span = document.createElement('span');
    span.textContent = v;
    return span;
  }));
}

// ── Render ───────────────────────────────────────────────────────────────────

function renderLocationTabs() {
  locationTabsEl.querySelectorAll('.tab').forEach(tab => {
    tab.classList.toggle('active', tab.dataset.location === state.location);
  });
  locationCityEl.textContent = getCitiesForLocation(enabledPlans, state.location).join(' · ');
}

function renderTypeTabs() {
  const types = getTypesForLocation(enabledPlans, state.location);
  typeTabsEl.replaceChildren(...types.map(key => {
    const def = typeDef(key);
    const btn = document.createElement('button');
    btn.className = 'tab' + (key === state.type ? ' active' : '');
    btn.dataset.type = key;
    btn.textContent = def.label;
    btn.addEventListener('click', () => {
      state = resolveSelection({ ...state, type: key });
      render();
    });
    return btn;
  }));
}

function renderSliders() {
  const coresOptions = getCoresOptions(enabledPlans, state.location, state.type);
  const ramOptions   = getRamOptions(enabledPlans, state.location, state.type, state.cores);
  const diskOptions  = getDiskOptions(enabledPlans, state.location, state.type, state.cores, state.ram);

  configureSlider(sliderCores, ticksCores, coresOptions, state.cores);
  configureSlider(sliderRam, ticksRam, ramOptions, state.ram);
  configureSlider(sliderDisk, ticksDisk, diskOptions, state.disk);
}

function renderPriceAndVisual() {
  const plan = findPlan(enabledPlans, state.location, state.type, state.cores, state.ram, state.disk);
  const t = typeDef(state.type);

  emptyStateEl.hidden = !!plan;
  orderBtn.disabled = !plan;
  priceDisplay.textContent = plan ? plan.price.toFixed(2) : '—';

  if (t) {
    coresCardLabelEl.textContent = t.coreLabel;
    badgeLine1.textContent = t.badge[0];
    badgeLine2.textContent = t.badge[1];
  }

  coreLabelEl.textContent = state.cores != null ? state.cores : '–';

  const coreIntensity = maxCores ? Math.min((state.cores || 0) / maxCores, 1) : 0;
  const hue = Math.round(40 - coreIntensity * 10);
  serverCore.style.background = `linear-gradient(135deg, hsl(${hue},80%,${42 + coreIntensity * 8}%), hsl(${hue},80%,28%))`;

  const ramRatio = maxRam ? Math.min((state.ram || 0) / maxRam, 1) : 0;
  const ramLit = Math.round(ramRatio * ramBars.length);
  ramBars.forEach((bar, i) => bar.classList.toggle('active', i < ramLit));
}

function render() {
  renderLocationTabs();
  renderTypeTabs();
  renderSliders();
  renderPriceAndVisual();
}

// ── URL sharing ──────────────────────────────────────────────────────────────

function buildURL() {
  const params = new URLSearchParams({
    loc:   state.location,
    type:  state.type,
    cores: state.cores,
    ram:   state.ram,
    disk:  state.disk,
  });
  return `${location.origin}${location.pathname}?${params.toString()}`;
}

function showToast(url) {
  toastUrl.textContent = url;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 4000);
}

function selectionFromURL() {
  const params = new URLSearchParams(location.search);
  return resolveSelection({
    location: params.get('loc'),
    type:     params.get('type'),
    cores:    Number(params.get('cores')),
    ram:      Number(params.get('ram')),
    disk:     Number(params.get('disk')),
  });
}

// ── Event listeners ──────────────────────────────────────────────────────────

locationTabsEl.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    state = resolveSelection({ ...state, location: tab.dataset.location });
    render();
  });
});

sliderCores.addEventListener('input', () => {
  const options = getCoresOptions(enabledPlans, state.location, state.type);
  state = resolveSelection({ ...state, cores: options[+sliderCores.value] });
  render();
});

sliderRam.addEventListener('input', () => {
  const options = getRamOptions(enabledPlans, state.location, state.type, state.cores);
  state = resolveSelection({ ...state, ram: options[+sliderRam.value] });
  render();
});

sliderDisk.addEventListener('input', () => {
  const options = getDiskOptions(enabledPlans, state.location, state.type, state.cores, state.ram);
  state = resolveSelection({ ...state, disk: options[+sliderDisk.value] });
  render();
});

orderBtn.addEventListener('click', () => {
  const url = buildURL();
  navigator.clipboard.writeText(url).catch(() => {});
  showToast(url);
});

// ── Init ─────────────────────────────────────────────────────────────────────

loadPlans()
  .then(all => {
    enabledPlans = all.filter(p => p.enabled);
    maxCores = Math.max(...enabledPlans.map(p => p.cores));
    maxRam   = Math.max(...enabledPlans.map(p => p.ram));
    state = selectionFromURL();
    render();
  })
  .catch(() => {
    emptyStateEl.hidden = false;
    emptyStateEl.textContent = 'Unable to load pricing data. Please refresh the page.';
    orderBtn.disabled = true;
  });
