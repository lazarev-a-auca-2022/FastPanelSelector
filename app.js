// ── Catalog definitions ─────────────────────────────────────────────────────
// Cores alone don't determine a plan: the same core count can map to
// different RAM/disk/price combos depending on location and CPU family, so
// every step below is derived from the live catalog, never index arithmetic
// against a fixed table.

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

// Quick-payment link template — product is the plan's own catalog id.
const PAYMENT_URL_BASE = 'https://my.kodu.cloud/v2/register/by-payment';
const PAYMENT_BILLCYCLE = '1';

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

function plansFor(plans, loc, type) {
  return plans.filter(p => p.location === loc && matchesType(p, type));
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

// Each axis spans every value that exists anywhere for (location, type) —
// not filtered by the other two dimensions. That's what makes every slider
// draggable through its full real range instead of collapsing to a single,
// disabled stop as soon as the other two happen to pin it down; dragging
// any one snaps to the nearest real plan and drags the *other* two along
// with it (see applyAxisChange), which is what actually makes the three
// controls feel connected.
function axisValues(plans, loc, type, field) {
  return [...new Set(plansFor(plans, loc, type).map(p => p[field]))].sort((a, b) => a - b);
}

function findPlan(plans, loc, type, cores, ram, disk) {
  return plans.find(p =>
    p.location === loc && matchesType(p, type) &&
    p.cores === cores && p.ram === ram && p.disk === disk
  );
}

// ── Selection resolution ─────────────────────────────────────────────────────
// Picking a plan is a nearest-neighbor search over the real catalog, not
// independent per-axis clamping — a "desired" point (however it arose: a
// stale URL, a garbage param, or the other two axes after a location/type
// switch invalidates the current plan) always resolves to a real, concrete
// plan, so state is never a Frankenstein combination that doesn't exist.

function pickValid(desired, options) {
  if (!options.length) return undefined;
  return options.includes(desired) ? desired : options[0];
}

function planDistance(plan, desired) {
  let d = 0;
  for (const field of ['cores', 'ram', 'disk']) {
    const want = desired[field];
    d += typeof want === 'number' && !Number.isNaN(want)
      ? Math.abs(plan[field] - want) / (want || 1)
      : 1;
  }
  return d;
}

function nearestPlan(plans, desired) {
  if (!plans.length) return undefined;
  let best = plans[0];
  let bestD = planDistance(best, desired);
  for (let i = 1; i < plans.length; i++) {
    const d = planDistance(plans[i], desired);
    if (d < bestD) { best = plans[i]; bestD = d; }
  }
  return best;
}

function resolveSelection(desired) {
  const location = pickValid(desired.location, getLocations(enabledPlans));
  const type = pickValid(desired.type, getTypesForLocation(enabledPlans, location));
  const candidates = plansFor(enabledPlans, location, type)
    .slice()
    .sort((a, b) => a.cores - b.cores || a.ram - b.ram || a.disk - b.disk);
  const plan = nearestPlan(candidates, desired);
  return {
    location,
    type,
    cores: plan ? plan.cores : undefined,
    ram: plan ? plan.ram : undefined,
    disk: plan ? plan.disk : undefined,
  };
}

// Dragging one slider to `value` finds the real plan with that exact value
// on `field`, choosing among ties the one closest to the *current* state on
// the other two fields — so RAM/disk aren't just followers of cores, and
// cores isn't just a follower of RAM either; whichever slider the user
// grabs drives the other two toward it.
function applyAxisChange(field, value) {
  const candidates = plansFor(enabledPlans, state.location, state.type).filter(p => p[field] === value);
  const plan = nearestPlan(candidates, state);
  if (!plan) return; // value came from this same catalog, so this shouldn't happen
  state = { location: state.location, type: state.type, cores: plan.cores, ram: plan.ram, disk: plan.disk };
  render();
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
const emptyStateEl = document.getElementById('empty-state');

const coreLabelEl = document.getElementById('core-label');
const ramBars     = document.querySelectorAll('.ram-bar');
const serverCore  = document.getElementById('server-core');
const badgeLine1  = document.getElementById('badge-line1');
const badgeLine2  = document.getElementById('badge-line2');

// ── Slider helpers ───────────────────────────────────────────────────────────
// Each slider's value is an index into that step's axis array, not a raw
// magnitude — this keeps stops evenly spaced regardless of gaps between
// real values (e.g. 1 / 2 / 4 / 8 / 12 / 16).

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
  configureSlider(sliderCores, ticksCores, axisValues(enabledPlans, state.location, state.type, 'cores'), state.cores);
  configureSlider(sliderRam, ticksRam, axisValues(enabledPlans, state.location, state.type, 'ram'), state.ram);
  configureSlider(sliderDisk, ticksDisk, axisValues(enabledPlans, state.location, state.type, 'disk'), state.disk);
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

// ── URL restore ──────────────────────────────────────────────────────────────
// Lets a link like ?loc=DE&type=shared-amd&cores=8&ram=16&disk=160 preselect
// a configuration; garbage or since-disabled values just resolve to the
// nearest real plan instead of breaking.

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

function buildPaymentURL(planId) {
  const params = new URLSearchParams({ product: planId, billcycle: PAYMENT_BILLCYCLE });
  return `${PAYMENT_URL_BASE}?${params.toString()}`;
}

// ── Event listeners ──────────────────────────────────────────────────────────

locationTabsEl.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    state = resolveSelection({ ...state, location: tab.dataset.location });
    render();
  });
});

sliderCores.addEventListener('input', () => {
  const axis = axisValues(enabledPlans, state.location, state.type, 'cores');
  applyAxisChange('cores', axis[+sliderCores.value]);
});

sliderRam.addEventListener('input', () => {
  const axis = axisValues(enabledPlans, state.location, state.type, 'ram');
  applyAxisChange('ram', axis[+sliderRam.value]);
});

sliderDisk.addEventListener('input', () => {
  const axis = axisValues(enabledPlans, state.location, state.type, 'disk');
  applyAxisChange('disk', axis[+sliderDisk.value]);
});

orderBtn.addEventListener('click', () => {
  const plan = findPlan(enabledPlans, state.location, state.type, state.cores, state.ram, state.disk);
  if (!plan) return;
  window.open(buildPaymentURL(plan.id), '_blank', 'noopener');
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
