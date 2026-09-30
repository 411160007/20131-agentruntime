#!/usr/bin/env node
// schema-v2-check.mjs - structural machine-check of docs/schema-v2.md:
// slot census, template census, decision vocabulary mirrors against the
// independent Node validator and the external contract doc, and the
// substantive four-element predicate for every complete slot.
//
// rc=0 ALL GREEN, rc=1 violations printed. --selftest proves the two
// core predicates fire on mutated fixtures (positive control) and stay
// silent on the real file (negative control).
'use strict';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const here = import.meta.dirname;
const root = path.join(here, '..');
const v = fs.readFileSync(path.join(root, 'docs', 'schema-v2.md'), 'utf8');
const val = fs.readFileSync(path.join(here, 'validate-jsonl.mjs'), 'utf8');
const api = fs.readFileSync(path.join(root, 'docs', 'api-v0.md'), 'utf8');

const EXPECTED_SLOTS = {
  decision: { status: 'complete', slice: null },
  intent: { status: 'complete', slice: null },
  authority: { status: 'complete', slice: null },
  impact: { status: 'pending', slice: 'W2.3' },
  recovery: { status: 'pending', slice: 'W2.3' },
  evidence: { status: 'pending', slice: 'W2.4' },
  profile: { status: 'pending', slice: 'W2.4' },
};
const ELEMENTS = ['Version', 'Compatibility', 'Migration', 'Validation'];

// Predicates (pure) - reused by the main checks and the selftest so the
// positive controls exercise the exact shipped logic.
function schemav2Blocks(text) {
  const out = [];
  const re = /```schemav2\n([\s\S]*?)```/g;
  let m;
  while ((m = re.exec(text)) !== null) {
    const kv = {};
    for (const line of m[1].split('\n')) {
      const t = line.trim();
      if (!t) continue;
      const i = t.indexOf(':');
      if (i < 0) throw new Error('unparsable schemav2 line: ' + t);
      const k = t.slice(0, i).trim();
      if (k in kv) throw new Error('duplicate key in block: ' + k);
      kv[k] = t.slice(i + 1).trim();
    }
    out.push(kv);
  }
  return out;
}

function slotCensusProblems(text) {
  const bad = [];
  let blocks;
  try {
    blocks = schemav2Blocks(text);
  } catch (e) {
    return ['block parse: ' + e.message];
  }
  const slots = {};
  for (const kv of blocks) {
    if (!('schema' in kv)) continue;
    if (kv.schema in slots) bad.push('duplicate slot ' + kv.schema);
    slots[kv.schema] = kv;
  }
  for (const [name, exp] of Object.entries(EXPECTED_SLOTS)) {
    const kv = slots[name];
    if (!kv) {
      bad.push('slot missing: ' + name);
      continue;
    }
    if (kv.status !== exp.status) bad.push('slot ' + name + ' status ' + kv.status + ' want ' + exp.status);
    if (exp.slice && kv.planned_slice !== exp.slice) bad.push('slot ' + name + ' pointer ' + (kv.planned_slice || 'NONE') + ' want ' + exp.slice);
    if (!exp.slice && 'planned_slice' in kv) bad.push('complete slot ' + name + ' still carries a pointer');
  }
  const extra = Object.keys(slots).filter((n) => !(n in EXPECTED_SLOTS));
  if (extra.length) bad.push('unknown slots: ' + extra.join(','));
  return bad;
}

function templateCensusProblems(text) {
  const bad = [];
  let blocks;
  try {
    blocks = schemav2Blocks(text);
  } catch {
    return ['block parse'];
  }
  const tmpl = blocks.filter((kv) => 'template_element' in kv);
  const names = tmpl.map((kv) => kv.template_element);
  if (!ELEMENTS.every((e) => names.includes(e)) || tmpl.length !== 4) bad.push('template element census');
  if (!tmpl.every((kv) => 'requires' in kv && kv.requires.length > 20)) bad.push('template requires lines');
  return bad;
}

function findKey(text, k) {
  let blocks;
  try {
    blocks = schemav2Blocks(text);
  } catch {
    return null;
  }
  const hits = blocks.filter((kv) => k in kv);
  return hits.length === 1 ? hits[0][k] : null;
}

function elementBody(text, name) {
  const head = '#### ' + name + '\n';
  const start = text.indexOf(head);
  if (start < 0) return null;
  const rest = text.slice(start + head.length);
  const m = rest.match(/^#{2,4} /m);
  const end = m ? start + head.length + m.index : text.length;
  return text.slice(start, end);
}

function substantive(body) {
  if (!body) return 'missing section';
  const lines = body.split('\n').filter((l) => l.trim());
  if (lines.length < 3) return 'fewer than 3 lines';
  if (!lines.some((l) => l.trim().startsWith('- '))) return 'no list item';
  const spans = (body.match(/`[^`\n]+`/g) || []).length;
  if (spans < 2) return 'fewer than 2 code spans';
  if (body.replace(/\s/g, '').length < 120) return 'under 120 non-space chars';
  return null;
}

function elementProblems(text) {
  const bad = [];
  const heads = [];
  const re = /^## \d+\. ([A-Za-z]+) schema - complete contract$/gm;
  let m;
  while ((m = re.exec(text)) !== null) heads.push({ name: m[1].toLowerCase(), start: m.index });
  for (let i = 0; i < heads.length; i++) {
    const end = i + 1 < heads.length ? findNextSection(text, heads[i].start + 1) : findNextSection(text, heads[i].start + 1);
    const body = text.slice(heads[i].start, end);
    for (const el of ELEMENTS) {
      const p = substantive(elementBody(body, el));
      if (p) bad.push(heads[i].name + '/' + el + ': ' + p);
    }
  }
  for (const [name, exp] of Object.entries(EXPECTED_SLOTS)) {
    if (exp.status === 'complete' && !heads.some((h) => h.name === name)) {
      bad.push(name + ': complete slot without a contract section');
    }
  }
  return bad;
}

function findNextSection(text, from) {
  const m = /(?=^## )/m;
  // scan forward for the next level-2 header after `from`
  const rest = text.slice(from);
  const idx = rest.search(/^## /m);
  return idx < 0 ? text.length : from + idx;
}

// Main check pipeline.
const problems = [];
function check(ok, label) {
  if (ok) console.log('PASS ' + label);
  else {
    console.log('FAIL ' + label);
    problems.push(label);
  }
}

check(slotCensusProblems(v).length === 0, 'slot census: 7 slots, decision/intent/authority complete, four pending with pointers');
if (slotCensusProblems(v).length) console.log('  ' + slotCensusProblems(v).join('\n  '));
check(templateCensusProblems(v).length === 0, 'template census: four elements with concrete requires lines');

const vocab = findKey(v, 'decision_vocabulary');
const phase0 = findKey(v, 'decision_phase0_runtime_set');
const carrier = findKey(v, 'decision_carrier_field');
const mapping = findKey(v, 'decision_effect_mapping');
check(vocab !== null && phase0 !== null && carrier !== null && mapping !== null,
  'decision contract keys present exactly once');

const valMatches = (val.match(/const DECISIONS = \[([^\]]*)\]/) || [, ''])[1].match(/'([^']+)'/g) || [];
const valList = valMatches.map((s) => s.replace(/'/g, ''));
const apiLine = (api.match(/^decisions: (.+)$/m) || [, ''])[1].trim();
check(!!vocab && valList.length === 3 && !!apiLine &&
  valList.join(', ') === vocab && apiLine === vocab,
  'decision vocabulary identical across schema-v2 / validator / api doc');
check(phase0 === 'allow, would_block',
  'runtime closed set is exactly the observation-phase pair');
check(!!phase0 && !!vocab &&
  phase0.split(', ').every((x) => vocab.split(', ').includes(x)) &&
  !phase0.split(', ').includes('ask'),
  'reserved token ask stays in vocabulary, out of the runtime set');
check(carrier === 'decision' && /DECISIONS\.includes\(e\.decision\)/.test(val),
  'carrier field name matches the validator check site');

const pairs = (mapping || '').split(';').map((p) => p.trim()).filter(Boolean);
const expectMap = ['allow', 'ask', 'would_block'].map((x) => x + '->' + x);
check(pairs.length === 3 && pairs.every((p, i) => p === expectMap[i]),
  'effect-to-decision mapping is verbatim identity over the three effects');

const ep = elementProblems(v);
check(ep.length === 0, 'every complete slot carries four substantive element sections (>=3 lines, list item, >=2 code spans, >=120 chars)');
if (ep.length) console.log('  ' + ep.join('\n  '));

// Intent + authority contract checks (slice W2.2). Pure predicates so
// the selftest exercises the shipped logic.
const goSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'intentauthority.go'), 'utf8');
const snake = (s) => s.toLowerCase().replace(/ +/g, '_');

function intentProblems(text, goText) {
  const bad = [];
  const vocab = findKey(text, 'intent_field_vocabulary');
  const count = findKey(text, 'intent_field_count');
  const absent = findKey(text, 'intent_absent_semantics');
  if (!vocab || !count || !absent) return ['intent contract keys missing or duplicated'];
  const toks = vocab.split(', ');
  if (toks.length !== 10 || new Set(toks).size !== 10) bad.push('intent vocabulary not ten unique tokens');
  if (count !== '10' || toks.length !== Number(count)) bad.push('intent count line drifts from vocabulary length');
  if (absent !== 'known-gap-never-fabricated') bad.push('absent-field semantics drifted');
  const anchorHits = text.match(/```intent-spec-anchor\n([\s\S]*?)```/g) || [];
  if (anchorHits.length !== 1) return bad.concat(['anchor block must appear exactly once']);
  const lines = anchorHits[0].replace('```intent-spec-anchor\n', '').replace('```', '').trim().split('\n').map((s) => s.trim()).filter(Boolean);
  if (lines.length !== 10) bad.push('anchor line census ' + lines.length + ', want 10');
  else lines.forEach((s, i) => { if (snake(s) !== toks[i]) bad.push('anchor back-check broke at line ' + (i + 1) + ': ' + s); });
  const varblk = (goText.match(/var intentFieldWireNames = \[\]string\{([\s\S]*?)\}/) || [, ''])[1];
  const goList = (varblk.match(/"[^"]+"/g) || []).map((s) => s.slice(1, -1));
  if (goList.join(', ') !== vocab) bad.push('intent vocabulary docs <-> Go drift');
  return bad;
}

function authorityProblems(text, goText) {
  const bad = [];
  const vocab = findKey(text, 'authority_origin_vocabulary');
  const carrier = findKey(text, 'authority_untrusted_carrier');
  const rule = findKey(text, 'authority_propagation_rule');
  const plane = findKey(text, 'authority_enforcement_plane');
  if (!vocab || !carrier || !rule || !plane) return ['authority contract keys missing or duplicated'];
  const goOrigins = [...goText.matchAll(/GrantOrigin = "([^"]+)"/g)].map((m) => m[1]);
  if (vocab.split(', ').length !== 5 || goOrigins.length !== 5 || goOrigins.join(', ') !== vocab) bad.push('origin vocabulary docs <-> Go drift');
  if (rule !== 'untrusted-sticky-never-auto-escalate' || !goText.includes('AuthorityPropagationRule = "untrusted-sticky-never-auto-escalate"')) bad.push('propagation rule drifted');
  if (plane !== 'none-in-observation-phase' || !goText.includes('AuthorityEnforcementPlane = "none-in-observation-phase"')) bad.push('enforcement plane drifted off none');
  if (carrier !== 'untrusted' || !goText.includes('json:"untrusted,omitempty"')) bad.push('untrusted carrier shape drifted');
  return bad;
}

const PLANE_DIRS = ['policy', 'rules', 'bus', 'auditlog'];
function planeLeakProblems() {
  const bad = [];
  const needle = /GrantOrigin|AuthorityChain|IntentRecord|intentFieldWireNames/;
  for (const d of PLANE_DIRS) {
    const dir = path.join(root, 'internal', d);
    let entries;
    try { entries = fs.readdirSync(dir); } catch { continue; }
    for (const f of entries.filter((x) => x.endsWith('.go') && !x.endsWith('_test.go'))) {
      if (needle.test(fs.readFileSync(path.join(dir, f), 'utf8'))) bad.push(d + '/' + f + ' references the new record symbols');
    }
  }
  return bad;
}

const ip = intentProblems(v, goSrc);
check(ip.length === 0, 'intent: ten-field vocabulary, programmatic count, verbatim anchor back-check, Go mirror');
if (ip.length) console.log('  ' + ip.join('\n  '));
const ap2 = authorityProblems(v, goSrc);
check(ap2.length === 0, 'authority: five-origin vocabulary, sticky propagation rule and none-enforcement plane mirrored to Go');
if (ap2.length) console.log('  ' + ap2.join('\n  '));
const leak = planeLeakProblems();
check(leak.length === 0, 'decision-plane directories: zero references to intent/authority record symbols');
if (leak.length) console.log('  ' + leak.join('\n  '));

console.log(problems.length === 0 ? 'SCHEMA-V2: ALL GREEN' : 'SCHEMA-V2: ' + problems.length + ' PROBLEM(S)');
if (problems.length) process.exit(1);

if (process.argv.includes('--selftest')) {
  // Negative control: shipped file passes everything (proven by rc above).
  // Positive controls: the shipped predicates must catch each mutation.
  const cases = [
    ['pending slot loses its pointer', (t) => t.replace('planned_slice: W2.4', 'shadow_slice: W2.4'), (t) => slotCensusProblems(t)],
    ['complete slot gains a stray pointer', (t) => t.replace('schema: decision\nstatus: complete', 'schema: decision\nstatus: complete\nplanned_slice: W9.9'), (t) => slotCensusProblems(t)],
    ['vocabulary loses a token', (t) => t.replace('decision_vocabulary: allow, ask, would_block', 'decision_vocabulary: allow, ask'), (t) => (findKey(t, 'decision_vocabulary') !== 'allow, ask, would_block' ? ['drift'] : [])],
    ['element body gutted to a slogan', (t) => t.replace(/#### Migration\n[\s\S]*?#### Validation/, '#### Migration\nnone owed\n#### Validation'), (t) => elementProblems(t)],
    ['intent anchor line renamed', (t) => t.replace('\nExpected Outcome\n', '\nExpected Result\n'), (t) => intentProblems(t, goSrc)],
    ['intent count line drifts', (t) => t.replace('intent_field_count: 10', 'intent_field_count: 9'), (t) => intentProblems(t, goSrc)],
    ['origin token drifts', (t) => t.replace('agent_provided, ui_generated', 'agent_assumed, ui_generated'), (t) => authorityProblems(t, goSrc)],
    ['enforcement plane pre-borrowed', (t) => t.replace('authority_enforcement_plane: none-in-observation-phase', 'authority_enforcement_plane: enforce-now'), (t) => authorityProblems(t, goSrc)],
  ];
  let fired = 0;
  for (const [name, mutate, pred] of cases) {
    const out = pred(mutate(v));
    if (out.length) {
      console.log('SELFTEST FIRE ' + name + ' -> ' + out[0]);
      fired++;
    } else {
      console.log('SELFTEST MISS ' + name);
    }
  }
  // sanity: predicates stay silent on the shipped file
  for (const pred of [slotCensusProblems, templateCensusProblems, elementProblems,
    (t) => intentProblems(t, goSrc), (t) => authorityProblems(t, goSrc)]) {
    if (pred(v).length) {
      console.log('SELFTEST FAIL: predicate fired on clean file');
      process.exit(1);
    }
  }
  if (fired !== cases.length) {
    console.log('SELFTEST FAIL: ' + fired + '/' + cases.length + ' mutations caught');
    process.exit(1);
  }
  console.log('SELFTEST OK: all ' + cases.length + ' mutations caught, clean file silent');
}
