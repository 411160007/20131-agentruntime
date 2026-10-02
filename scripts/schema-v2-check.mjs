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
  impact: { status: 'complete', slice: null },
  recovery: { status: 'complete', slice: null },
  evidence: { status: 'complete', slice: null },
  profile: { status: 'complete', slice: null },
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

function consumeProblems(text, goText) {
  const bad = [];
  const pins = [
    ['untrusted_consume_rule', 'UntrustedConsumeRule', 'untrusted-source-modifications-are-recorded-marked-never-auto-escalate'],
    ['authority_preservation_rule', 'AuthorityPreservationRule', 'original-chain-links-preserved-identical'],
    ['origin_forge_rule', 'OriginForgeRule', 'untrusted-source-never-claims-user_direct'],
    ['sticky_clearance_rule', 'StickyClearanceRule', 'consumption-never-clears-an-existing-untrusted-mark'],
    ['consume_enforcement_plane', 'ConsumeEnforcementPlane', 'none-in-observation-phase'],
  ];
  for (const [key, ident, val] of pins) {
    const got = findKey(text, key);
    if (got === null) return ['consume contract keys missing or duplicated'];
    if (got !== val) bad.push('consume rule line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) bad.push('consume rule Go mirror drifted: ' + ident);
  }
  if (text.indexOf('## 13. UNTRUSTED consumption contract') < 0) bad.push('section 13 header missing');
  return bad;
}

function inflowProblems(text, goText) {
  const bad = [];
  const pins = [
    ['inflow_channel_vocabulary', 'AllIntentInflowSources', 'cli_file,hook_task,absent_not_reported'],
    ['hook_task_mapping_rule', 'HookTaskMappingRule', 'task-field-maps-to-goal-and-nothing-else'],
    ['inflow_absent_default', 'InflowAbsentDefault', 'not-reported-is-known-gap-never-fabricated'],
    ['inflow_forge_rule', 'InflowForgeRule', 'inflow-channel-never-escalates-authority'],
    ['inflow_enforcement_plane', 'InflowEnforcementPlane', 'none-in-observation-phase'],
  ];
  for (const [key, ident, val] of pins) {
    const got = findKey(text, key);
    if (got === null) return ['inflow contract keys missing or duplicated'];
    if (got !== val) bad.push('inflow line drifted: ' + key);
    if (ident === 'AllIntentInflowSources') {
      for (const tok of val.split(',')) {
        if (!goText.includes('"' + tok + '"')) bad.push('inflow channel Go mirror missing: ' + tok);
      }
    } else if (!goText.includes(ident + ' = "' + val + '"')) {
      bad.push('inflow rule Go mirror drifted: ' + ident);
    }
  }
  if (text.indexOf('## 14. Intent inflow contract') < 0) bad.push('section 14 header missing');
  return bad;
}

function dataActionProblems(text, goText) {
  const bad = [];
  const pins = [
    ['data_action_vocabulary', 'AllDataActions', 'discover,read,write,modify,delete,execute,export,share,persist'],
    ['data_action_nonequivalence_rule', 'DataActionNonequivalenceRule', 'read-not-export-read-not-share-write-not-execute'],
    ['data_action_export_rejudgement_rule', 'DataActionExportRejudgementRule', 'export-requires-independent-rejudgement'],
    ['data_action_absent_default', 'DataActionAbsentDefault', 'absent-means-unclassified-legacy-never-inferred'],
    ['data_action_enforcement_plane', 'DataActionEnforcementPlane', 'none-in-observation-phase'],
  ];
  for (const [key, ident, val] of pins) {
    const got = findKey(text, key);
    if (got === null) return ['data action contract keys missing or duplicated'];
    if (got !== val) bad.push('data action line drifted: ' + key);
    if (ident === 'AllDataActions') {
      for (const tok of val.split(',')) {
        if (!goText.includes('"' + tok + '"')) bad.push('data action vocabulary Go mirror missing: ' + tok);
      }
    } else if (!goText.includes(ident + ' = "' + val + '"')) {
      bad.push('data action rule Go mirror drifted: ' + ident);
    }
  }
  if (text.indexOf('## 15. Data action vocabulary contract') < 0) bad.push('section 15 header missing');
  return bad;
}

function trustDomainProblems(text, goText) {
  const bad = [];
  const rulePins = [
    ['trust_domain_crossing_rule', 'TrustDomainCrossingRule', 'cross-domain-data-movement-requires-data-boundary'],
    ['trust_domain_absent_default', 'TrustDomainAbsentDefault', 'absent-means-unclassified-legacy-never-inferred'],
    ['trust_domain_enforcement_plane', 'TrustDomainEnforcementPlane', 'none-in-observation-phase'],
  ];
  const levels = findKey(text, 'trust_level_vocabulary');
  const domains = findKey(text, 'run_domain_vocabulary');
  const projection = findKey(text, 'trust_domain_legacy_projection');
  if (levels === null || domains === null || projection === null) {
    return ['trust domain contract keys missing or duplicated'];
  }
  for (const tok of levels.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('trust level Go mirror missing: ' + tok);
  }
  for (const tok of domains.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('run domain Go mirror missing: ' + tok);
  }
  // Migration annotation pairs, pinned against the Go map literal.
  // A projection line rewritten into the lift direction (low=s0...)
  // is the planted red shape: pair keys must stay trust levels and
  // pair values must stay legacy classes.
  const goPairKey = {
    s0_normal: 'LevelNormal', s1_private: 'LevelPrivate',
    s2_sensitive: 'LevelSensitive', s3_credential: 'LevelCredential',
    s4_security_boundary: 'LevelSecurityBoundary',
  };
  const goPairVal = { low: 'ResLow', medium: 'ResMedium', high: 'ResHigh' };
  const seenLevels = new Set();
  for (const pair of projection.split(',')) {
    const kv = pair.split('=');
    if (kv.length !== 2 || !(kv[0] in goPairKey) || !(kv[1] in goPairVal)) {
      return ['trust domain projection line drifted off the five-pair table'];
    }
    seenLevels.add(kv[0]);
    const pairRe = new RegExp(goPairKey[kv[0]] + '\\s*:\\s*' + goPairVal[kv[1]] + ',');
    if (!pairRe.test(goText)) {
      bad.push('trust domain projection Go mirror drifted: ' + pair);
    }
  }
  if (seenLevels.size !== 5) bad.push('trust domain projection lost a level row');
  for (const [key, ident, val] of rulePins) {
    const got = findKey(text, key);
    if (got === null) return ['trust domain contract keys missing or duplicated'];
    if (got !== val) bad.push('trust domain line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) {
      bad.push('trust domain rule Go mirror drifted: ' + ident);
    }
  }
  if (text.indexOf('## 16. Trust domain mapping contract') < 0) bad.push('section 16 header missing');
  return bad;
}

function exportWrapProblems(text, goText) {
  const bad = [];
  const rulePins = [
    ['export_packaging_sensitivity_rule', 'ExportPackagingSensitivityRule', 'packaging-never-lowers-sensitivity'],
    ['export_rejudgement_independence_rule', 'ExportRejudgementIndependenceRule', 'read-allow-never-carries-to-export'],
    ['export_rejudgement_absent_default', 'ExportRejudgementAbsentDefault', 'absent-means-no-packaging-observed-never-inferred'],
    ['export_rejudgement_enforcement_plane', 'ExportRejudgementEnforcementPlane', 'none-in-observation-phase'],
  ];
  const words = findKey(text, 'export_packaging_word_vocabulary');
  if (words === null) return ['export wrap contract keys missing or duplicated'];
  if (words !== 'compress,encode,encrypt,archive,copy,upload,send,share') {
    bad.push('export wrap vocabulary line drifted from the closed eight');
  }
  for (const tok of words.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('packaging word Go mirror missing: ' + tok);
  }
  for (const [key, ident, val] of rulePins) {
    const got = findKey(text, key);
    if (got === null) return ['export wrap contract keys missing or duplicated'];
    if (got !== val) bad.push('export wrap line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) {
      bad.push('export wrap rule Go mirror drifted: ' + ident);
    }
  }
  if (text.indexOf('## 17. EXPORT re-judgement and packaging annotation contract') < 0) bad.push('section 17 header missing');
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

// Master table (W2 closing slice): seven wave schemas x four elements,
// 28 cells, each cell exactly one closed three-state value and a
// machine-resolvable pointer. Pure predicates so the selftest exercises
// the shipped logic.
const MASTER_STATES = ['preexisting', 'this_wave', 'planned_build'];
function contractSectionMap(text) {
  const map = {};
  const re = /^## \d+\. ([A-Za-z]+) schema - complete contract$/gm;
  let m;
  while ((m = re.exec(text)) !== null) {
    map[m[1].toLowerCase()] = text.slice(m.index, findNextSection(text, m.index + 1));
  }
  return map;
}
function masterTableProblems(text) {
  const bad = [];
  const blocks = schemav2Blocks(text).filter((kv) => 'master_cell' in kv);
  if (blocks.length !== 28) bad.push('master cell census ' + blocks.length + ', want 28');
  const seen = new Set();
  const sections = contractSectionMap(text);
  for (const kv of blocks) {
    const cell = String(kv.master_cell || '');
    if (!/^[a-z]+\/[a-z]+$/.test(cell)) { bad.push('malformed master_cell: ' + cell); continue; }
    const [schemaName, element] = cell.split('/');
    if (!(schemaName in EXPECTED_SLOTS)) { bad.push('master_cell on off-wave schema: ' + cell); continue; }
    const el = element[0].toUpperCase() + element.slice(1);
    if (!ELEMENTS.includes(el)) { bad.push('master_cell with unknown element: ' + cell); continue; }
    if (seen.has(cell)) { bad.push('duplicate master_cell: ' + cell); continue; }
    seen.add(cell);
    const st = kv.state;
    if (!MASTER_STATES.includes(st)) { bad.push('wild master_cell state at ' + cell + ': ' + st); continue; }
    const ev = String(kv.evidence || '');
    if (!ev.trim()) { bad.push('master_cell without evidence: ' + cell); continue; }
    if (st === 'planned_build') {
      if (!/\bW\d+\.\d+\b/.test(ev)) bad.push('planned_build cell missing W commitment point: ' + cell);
      continue;
    }
    if (st === 'this_wave' && !/\bW2\.[1-4]\b/.test(ev)) bad.push('this_wave cell missing creating W2 slice: ' + cell);
    if (st === 'preexisting' && !ev.includes('api-v0')) bad.push('preexisting cell must point at the frozen api-v0 contract: ' + cell);
    const sec = sections[schemaName];
    if (!sec) bad.push('cell target section missing: ' + cell);
    else {
      const p = substantive(elementBody(sec, el));
      if (p) bad.push('cell target section not substantive: ' + cell + ' - ' + p);
    }
  }
  for (const s of Object.keys(EXPECTED_SLOTS)) {
    for (const el of ELEMENTS) {
      if (!seen.has(s + '/' + el.toLowerCase())) bad.push('missing master_cell: ' + s + '/' + el.toLowerCase());
    }
  }
  return bad;
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

check(slotCensusProblems(v).length === 0, 'slot census: 7 slots, all seven schemas complete with zero pointers');
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
const irSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'impactrecovery.go'), 'utf8');
const epSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'evidenceprofile.go'), 'utf8');
const aaSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'actionalignment.go'), 'utf8');
const ucSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'untrustedconsume.go'), 'utf8');
const iiSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'intentinflow.go'), 'utf8');
const daSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'dataaction.go'), 'utf8');
const tdSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'trustdomain.go'), 'utf8');
const ewSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'exportwrap.go'), 'utf8');
const snake = (s) => s.toLowerCase().replace(/ +/g, '_');
// Recovery anchor rule is one step wider: spaces AND hyphens collapse
// to single underscores (the spec spells NON-REVERSIBLE with a hyphen).
const snakeH = (s) => s.toLowerCase().replace(/[ -]+/g, '_');
// Evidence anchor rule is wider once more: spaces AND slashes collapse
// (the spec spells "Actor / Agent Identity" with a slash).
const snakeS = (s) => s.toLowerCase().replace(/[ \/]+/g, '_');

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

function impactProblems(text, goText) {
  const bad = [];
  const vocab = findKey(text, 'impact_field_vocabulary');
  const count = findKey(text, 'impact_field_count');
  const absent = findKey(text, 'impact_absent_semantics');
  const scopeVocab = findKey(text, 'blast_radius_scope_vocabulary');
  const scopeCount = findKey(text, 'blast_radius_scope_count');
  const plane = findKey(text, 'impact_enforcement_plane');
  if (!vocab || !count || !absent || !scopeVocab || !scopeCount || !plane) return ['impact contract keys missing or duplicated'];
  const toks = vocab.split(', ');
  if (toks.length !== 7 || new Set(toks).size !== 7) bad.push('impact vocabulary not seven unique tokens');
  if (count !== '7' || toks.length !== Number(count)) bad.push('impact count line drifts from vocabulary length');
  if (absent !== 'not-estimated-known-gap') bad.push('impact absent-field semantics drifted');
  if (plane !== 'none-in-observation-phase' || !goText.includes('ImpactEnforcementPlane = "none-in-observation-phase"')) bad.push('impact enforcement plane drifted off none');
  const anchorHits = text.match(/```impact-spec-anchor\n([\s\S]*?)```/g) || [];
  if (anchorHits.length !== 1) return bad.concat(['impact anchor block must appear exactly once']);
  const lines = anchorHits[0].replace('```impact-spec-anchor\n', '').replace('```', '').trim().split('\n').map((s) => s.trim()).filter(Boolean);
  if (lines.length !== 7) bad.push('impact anchor line census ' + lines.length + ', want 7');
  else lines.forEach((s, i) => { if (snake(s) !== toks[i]) bad.push('impact anchor back-check broke at line ' + (i + 1) + ': ' + s); });
  const varblk = (goText.match(/var impactFieldWireNames = \[\]string\{([\s\S]*?)\}/) || [, ''])[1];
  const goList = (varblk.match(/"[^"]+"/g) || []).map((s) => s.slice(1, -1));
  if (goList.join(', ') !== vocab) bad.push('impact vocabulary docs <-> Go drift');
  const stoks = scopeVocab.split(', ');
  if (stoks.length !== 8 || new Set(stoks).size !== 8) bad.push('blast scope vocabulary not eight unique tokens');
  if (scopeCount !== '8' || stoks.length !== Number(scopeCount)) bad.push('blast scope count line drifts');
  const sb = (goText.match(/var blastScopeWireNames = \[\]string\{([\s\S]*?)\}/) || [, ''])[1];
  const sgo = (sb.match(/"[^"]+"/g) || []).map((s) => s.slice(1, -1));
  if (sgo.join(', ') !== scopeVocab) bad.push('blast scope vocabulary docs <-> Go drift');
  const rowRe = /^\| `([a-z_]+_scope)` \|[^|\n]+\| line (\d+) \|$/gm;
  const rows = [];
  let rm;
  while ((rm = rowRe.exec(text)) !== null) rows.push([rm[1], rm[2]]);
  if (rows.length !== 8) bad.push('scope table row census ' + rows.length + ', want 8');
  else rows.forEach(([tok, n], i) => {
    if (tok !== stoks[i]) bad.push('scope row-order pin broke at row ' + (i + 1) + ': ' + tok);
    if (Number(n) !== i + 1) bad.push('scope bullet pointer off at row ' + (i + 1));
  });
  return bad;
}

function recoveryProblems(text, goText) {
  const bad = [];
  const vocab = findKey(text, 'recovery_class_vocabulary');
  const count = findKey(text, 'recovery_class_count');
  const unclassified = findKey(text, 'recovery_unclassified_semantics');
  const execPlane = findKey(text, 'recovery_execution_plane');
  const truthful = findKey(text, 'recovery_truthfulness_rule');
  if (!vocab || !count || !unclassified || !execPlane || !truthful) return ['recovery contract keys missing or duplicated'];
  const toks = vocab.split(', ');
  if (toks.length !== 4 || new Set(toks).size !== 4) bad.push('recovery vocabulary not four unique classes');
  if (count !== '4' || toks.length !== Number(count)) bad.push('recovery count line drifts from vocabulary length');
  if (unclassified !== 'absent-record-means-unknown-never-imply-reversible') bad.push('unclassified semantics drifted');
  if (execPlane !== 'none-in-observation-phase' || !goText.includes('RecoveryExecutionPlane = "none-in-observation-phase"')) bad.push('recovery execution plane drifted off none');
  if (truthful !== 'never-claim-fully-reversible' || !goText.includes('RecoveryTruthfulnessRule = "never-claim-fully-reversible"')) bad.push('truthfulness rule drifted');
  const anchorHits = text.match(/```recovery-spec-anchor\n([\s\S]*?)```/g) || [];
  if (anchorHits.length !== 1) return bad.concat(['recovery anchor block must appear exactly once']);
  const lines = anchorHits[0].replace('```recovery-spec-anchor\n', '').replace('```', '').trim().split('\n').map((s) => s.trim()).filter(Boolean);
  if (lines.length !== 4) bad.push('recovery anchor line census ' + lines.length + ', want 4');
  else lines.forEach((s, i) => { if (snakeH(s) !== toks[i]) bad.push('recovery anchor back-check broke at line ' + (i + 1) + ': ' + s); });
  const goClasses = [...goText.matchAll(/RecoveryClass = "([^"]+)"/g)].map((m) => m[1]);
  if (goClasses.join(', ') !== vocab) bad.push('recovery class vocabulary docs <-> Go drift');
  return bad;
}

function evidenceProblems(text, goText) {
  const bad = [];
  const vocab = findKey(text, 'evidence_field_vocabulary');
  const count = findKey(text, 'evidence_field_count');
  const required = findKey(text, 'evidence_required_semantics');
  const plane = findKey(text, 'evidence_enforcement_plane');
  const retain = findKey(text, 'evidence_context_retention_rule');
  const sigPhase = findKey(text, 'evidence_signature_envelope_phase');
  if (!vocab || !count || !required || !plane || !retain || !sigPhase) return ['evidence contract keys missing or duplicated'];
  const toks = vocab.split(', ');
  if (toks.length !== 7 || new Set(toks).size !== 7) bad.push('evidence vocabulary not seven unique tokens');
  if (count !== '7' || toks.length !== Number(count)) bad.push('evidence count line drifts from vocabulary length');
  if (required !== 'all-seven-required-for-key-evidence') bad.push('evidence requiredness semantics drifted');
  if (plane !== 'none-in-observation-phase' || !goText.includes('EvidenceEnforcementPlane = "none-in-observation-phase"')) bad.push('evidence enforcement plane drifted off none');
  if (retain !== 'must-not-drop-key-context-for-export' || !goText.includes('EvidenceContextRetentionRule = "must-not-drop-key-context-for-export"')) bad.push('context-retention rule drifted');
  if (sigPhase !== 'deferred-later-phase' || !goText.includes('EvidenceSignatureEnvelope = "deferred-later-phase"')) bad.push('signature-envelope phasing note drifted');
  const anchorHits = text.match(/```evidence-spec-anchor\n([\s\S]*?)```/g) || [];
  if (anchorHits.length !== 1) return bad.concat(['evidence anchor block must appear exactly once']);
  const lines = anchorHits[0].replace('```evidence-spec-anchor\n', '').replace('```', '').trim().split('\n').map((s) => s.trim()).filter(Boolean);
  if (lines.length !== 7) bad.push('evidence anchor line census ' + lines.length + ', want 7');
  else lines.forEach((s, i) => { if (snakeS(s) !== toks[i]) bad.push('evidence anchor back-check broke at line ' + (i + 1) + ': ' + s); });
  const varblk = (goText.match(/var evidenceFieldWireNames = \[\]string\{([\s\S]*?)\}/) || [, ''])[1];
  const goList = (varblk.match(/"[^"]+"/g) || []).map((s) => s.slice(1, -1));
  if (goList.join(', ') !== vocab) bad.push('evidence vocabulary docs <-> Go drift');
  return bad;
}

function profileProblems(text, goText) {
  const bad = [];
  const scope = findKey(text, 'profile_scope_vocabulary');
  const scopeCount = findKey(text, 'profile_scope_count');
  const pvocab = findKey(text, 'profile_personal_field_vocabulary');
  const pcount = findKey(text, 'profile_personal_field_count');
  const avocab = findKey(text, 'profile_agent_field_vocabulary');
  const acount = findKey(text, 'profile_agent_field_count');
  const absent = findKey(text, 'profile_absent_semantics');
  const numberSem = findKey(text, 'profile_number_semantics');
  const plane = findKey(text, 'profile_enforcement_plane');
  const hard = findKey(text, 'profile_hard_boundary_rule');
  if (!scope || !scopeCount || !pvocab || !pcount || !avocab || !acount || !absent || !numberSem || !plane || !hard) return ['profile contract keys missing or duplicated'];
  const stoks = scope.split(', ');
  if (stoks.length !== 2 || stoks.join(', ') !== 'personal, agent') bad.push('profile scope vocabulary drifted');
  if (scopeCount !== '2') bad.push('profile scope count drifts');
  const sgo = [...goText.matchAll(/ProfileScope\w+\s+ProfileScope = "([^"]+)"/g)].map((m) => m[1]);
  if (sgo.join(', ') !== scope) bad.push('profile scope docs <-> Go drift');
  const ptoks = pvocab.split(', '); const atoks = avocab.split(', ');
  if (ptoks.length !== 12 || new Set(ptoks).size !== 12) bad.push('personal vocabulary not twelve unique tokens');
  if (atoks.length !== 11 || new Set(atoks).size !== 11) bad.push('agent vocabulary not eleven unique tokens');
  if (pcount !== '12' || ptoks.length !== Number(pcount)) bad.push('personal count line drifts');
  if (acount !== '11' || atoks.length !== Number(acount)) bad.push('agent count line drifts');
  if (absent !== 'not-yet-observed-known-gap' || !goText.includes('ProfileAbsentSemantics = "not-yet-observed-known-gap"')) bad.push('profile absent semantics drifted');
  if (numberSem !== 'recorded-text-no-numeric-score') bad.push('profile number semantics drifted');
  if (plane !== 'none-in-observation-phase' || !goText.includes('ProfileEnforcementPlane = "none-in-observation-phase"')) bad.push('profile enforcement plane drifted off none');
  if (hard !== 'never-replaces-hard-security-boundaries' || !goText.includes('ProfileHardBoundaryRule = "never-replaces-hard-security-boundaries"')) bad.push('hard-boundary rule drifted');
  const anchor = (name, toks, conv) => {
    const hits = text.match(new RegExp('```' + name + '\n([\\s\\S]*?)```', 'g')) || [];
    if (hits.length !== 1) { bad.push(name + ' must appear exactly once'); return; }
    const lines = hits[0].replace('```' + name + '\n', '').replace('```', '').trim().split('\n').map((s) => s.trim()).filter(Boolean);
    if (lines.length !== toks.length) bad.push(name + ' line census ' + lines.length + ', want ' + toks.length);
    else lines.forEach((s, i) => { if (conv(s) !== toks[i]) bad.push(name + ' back-check broke at line ' + (i + 1) + ': ' + s); });
  };
  anchor('profile-personal-spec-anchor', ptoks, snake);
  anchor('profile-agent-spec-anchor', atoks, snake);
  const gv = (name) => {
    const blk = (goText.match(new RegExp('var ' + name + ' = \"?\\[\\]string\\{([^\\]\\}]*?)\\}')) || [, ''])[1];
    return (blk.match(/"[^"]+"/g) || []).map((s) => s.slice(1, -1));
  };
  if (gv('profilePersonalFieldWireNames').join(', ') !== pvocab) bad.push('personal vocabulary docs <-> Go drift');
  if (gv('profileAgentFieldWireNames').join(', ') !== avocab) bad.push('agent vocabulary docs <-> Go drift');
  // shared-token discipline: common_tools is one token in both scopes
  if (!ptoks.includes('common_tools') || !atoks.includes('common_tools')) bad.push('shared tools token forked');
  return bad;
}

function alignmentProblems(text, goText) {
  const bad = [];
  const vocab = findKey(text, 'alignment_class_vocabulary');
  const count = findKey(text, 'alignment_class_count');
  const absent = findKey(text, 'alignment_absent_semantics');
  const malicious = findKey(text, 'alignment_malicious_rule');
  const escalation = findKey(text, 'alignment_escalation_precondition');
  const plane = findKey(text, 'alignment_enforcement_plane');
  if (!vocab || !count || !absent || !malicious || !escalation || !plane) return ['alignment contract keys missing or duplicated'];
  const toks = vocab.split(', ');
  if (toks.length !== 4 || new Set(toks).size !== 4) bad.push('alignment vocabulary not four unique classes');
  if (count !== '4' || toks.length !== Number(count)) bad.push('alignment count line drifts from vocabulary length');
  if (absent !== 'no-record-means-never-compared-never-implied-direct' || !goText.includes('AlignmentAbsentSemantics = "no-record-means-never-compared-never-implied-direct"')) bad.push('alignment absent semantics drifted');
  if (malicious !== 'uncertain-is-not-malicious-unrelated-is-not-malicious' || !goText.includes('AlignmentMaliciousRule = "uncertain-is-not-malicious-unrelated-is-not-malicious"')) bad.push('malicious-equation rule drifted');
  if (escalation !== 'recorded-not-enforced' || !goText.includes('AlignmentEscalationPrecondition = "recorded-not-enforced"')) bad.push('escalation precondition drifted off recorded');
  if (plane !== 'none-in-observation-phase' || !goText.includes('AlignmentEnforcementPlane = "none-in-observation-phase"')) bad.push('alignment enforcement plane drifted off none');
  const anchorHits = text.match(/```alignment-spec-anchor\n([\s\S]*?)```/g) || [];
  if (anchorHits.length !== 1) return bad.concat(['alignment anchor block must appear exactly once']);
  const lines = anchorHits[0].replace('```alignment-spec-anchor\n', '').replace('```', '').trim().split('\n').map((s) => s.trim()).filter(Boolean);
  if (lines.length !== 4) bad.push('alignment anchor line census ' + lines.length + ', want 4');
  else lines.forEach((s, i) => { if (s.toLowerCase() !== toks[i]) bad.push('alignment anchor back-check broke at line ' + (i + 1) + ': ' + s); });
  const goClasses = [...goText.matchAll(/ActionClass = "([^"]+)"/g)].map((m) => m[1]);
  if (goClasses.join(', ') !== vocab) bad.push('alignment class vocabulary docs <-> Go drift');
  const fb = (goText.match(/var alignmentRecordFieldWireNames = \[\]string\{([\s\S]*?)\}/) || [, ''])[1];
  const flist = (fb.match(/"[^"]+"/g) || []).map((s) => s.slice(1, -1));
  if (flist.join(', ') !== 'class, action_ref, intent_ref, plan_ref, basis') bad.push('alignment record field list drifted from Go');
  const secAt = text.indexOf('## 12. Intent alignment record contract');
  if (secAt < 0) return bad.concat(['alignment section header missing']);
  const rows = [...text.slice(secAt).matchAll(/^\| `([a-z_]+)` \| (yes|no) \|[^|\n]*\|$/gm)];
  if (rows.length !== 5) bad.push('alignment field table row census ' + rows.length + ', want 5');
  else rows.forEach((rm, i) => { if (rm[1] !== flist[i]) bad.push('alignment field row-order pin broke at row ' + (i + 1) + ': ' + rm[1]); });
  return bad;
}

const PLANE_DIRS = ['policy', 'rules', 'bus', 'auditlog'];
function planeLeakProblems() {
  const bad = [];
  const needle = /GrantOrigin|AuthorityChain|IntentRecord|intentFieldWireNames|RecoveryClass|RecoveryRecord|ImpactRecord|BlastScope|BlastRadiusEstimate|blastScopeWireNames|impactFieldWireNames|EvidenceRecord|EvidenceField|evidenceFieldWireNames|PersonalProfileRecord|AgentProfileRecord|profilePersonalFieldWireNames|profileAgentFieldWireNames|profileScopeWireNames|ProfileScope|ActionClass|AlignmentRecord|alignmentClassWireNames|alignmentRecordFieldWireNames|AlignmentAbsentSemantics|AlignmentMaliciousRule|IntentModification|ApplyIntentModification|UntrustedConsumeRule|AuthorityPreservationRule|OriginForgeRule|StickyClearanceRule|ConsumeEnforcementPlane|IntentInflow|InflowSource|AllIntentInflowSources|InflowFromFile|InflowFromHookPayload|AbsentIntentRecord|HookTaskMappingRule|InflowAbsentDefault|InflowForgeRule|InflowEnforcementPlane|DataAction|AllDataActions|DataActionNonequivalenceRule|DataActionExportRejudgementRule|DataActionAbsentDefault|DataActionEnforcementPlane|TrustLevel|RunDomain|AllTrustLevels|AllRunDomains|LegacyClassOf|LiftCandidates|trustToLegacyClass|TrustDomainLegacyProjectionRule|TrustDomainCrossingRule|TrustDomainAbsentDefault|TrustDomainEnforcementPlane|PackagingWord|AllPackagingWords|ExportRejudgementRecord|BuildExportRejudgement|ExportPackagingSensitivityRule|ExportRejudgementIndependenceRule|ExportRejudgementAbsentDefault|ExportRejudgementEnforcementPlane/;
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

// Recovery-execution red-shape gate (slice W2.3): no rollback-family
// execution vocabulary may appear in decision-plane files or anywhere
// in the command tree while the execution plane is contracted as none.
// Baseline is zero hits; any hit is a deliberate contract event that
// must retire the plane line in the same change.
const EXEC_VERB_RE = /rollback|revert|undo|compensat/i;
function execVerbFiles(dir) {
  const out = [];
  let entries;
  try { entries = fs.readdirSync(dir, { withFileTypes: true }); } catch { return out; }
  for (const e of entries) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) out.push(...execVerbFiles(p));
    else if (e.name.endsWith('.go') && !e.name.endsWith('_test.go')) out.push(p);
  }
  return out;
}
function execVerbProblems() {
  const bad = [];
  const dirs = PLANE_DIRS.map((d) => path.join(root, 'internal', d)).concat([path.join(root, 'cmd')]);
  for (const d of dirs) {
    for (const f of execVerbFiles(d)) {
      const text = fs.readFileSync(f, 'utf8');
      if (EXEC_VERB_RE.test(text)) bad.push(path.relative(root, f) + ' carries recovery-execution vocabulary');
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
check(leak.length === 0, 'decision-plane directories: zero references to wave record symbols');
if (leak.length) console.log('  ' + leak.join('\n  '));

const imp = impactProblems(v, irSrc);
check(imp.length === 0, 'impact: seven-field vocabulary, anchor back-check, eight-scope row-order pin, Go mirrors, none-plane');
if (imp.length) console.log('  ' + imp.join('\n  '));
const rec = recoveryProblems(v, irSrc);
check(rec.length === 0, 'recovery: four-class closed vocabulary, hyphen-aware anchor, truthfulness and execution-plane strings mirrored to Go');
if (rec.length) console.log('  ' + rec.join('\n  '));
const exv = execVerbProblems();
check(exv.length === 0, 'execution-vocabulary red-shape grep: no rollback/revert/undo/compensate in decision planes or command tree');
if (exv.length) console.log('  ' + exv.join('\n  '));
const evd = evidenceProblems(v, epSrc);
check(evd.length === 0, 'evidence: seven required fields, slash-aware anchor back-check, Go mirrors, none-plane, retention and signature-phasing strings');
if (evd.length) console.log('  ' + evd.join('\n  '));
const prf = profileProblems(v, epSrc);
check(prf.length === 0, 'profile: scope/personal/agent vocabularies 2+12+11, dual anchors, shared tools token, hard-boundary and none-plane mirrored to Go');
if (prf.length) console.log('  ' + prf.join('\n  '));

const aln = alignmentProblems(v, aaSrc);
check(aln.length === 0, 'alignment: four-class closed vocabulary, lower-case anchor back-check, malicious-equation and escalation strings, none-plane, field table row-order pin, Go mirrors');
if (aln.length) console.log('  ' + aln.join('\n  '));

const con = consumeProblems(v, ucSrc);
check(con.length === 0, 'untrusted consumption: five rule lines mirrored docs<->Go, forge and preservation and stickiness and none-plane pinned, section 13 present');
if (con.length) console.log('  ' + con.join('\n  '));

const inf = inflowProblems(v, iiSrc);
check(inf.length === 0, 'inflow: three-channel closed vocabulary mirrored docs<->Go, task mapping and known-gap default and forge rejection and none-plane pinned, section 14 present');
const da = dataActionProblems(v, daSrc);
check(da.length === 0, 'data action: nine-word closed vocabulary mirrored docs<->Go<->api contract, nonequivalence and rejudgement and absent-default and none-plane pinned, section 15 present');
if (inf.length) console.log('  ' + inf.join('\n  '));

const td = trustDomainProblems(v, tdSrc);
check(td.length === 0, 'trust domain: five levels and seven domains mirrored docs<->Go, projection table pinned pair by pair, lift-window and crossing and absent-default and none-plane rules pinned, section 16 present');
if (td.length) console.log('  ' + td.join('\n  '));
const ew = exportWrapProblems(v, ewSrc);
check(ew.length === 0, 'export wrap: eight-word packaging family mirrored docs<->Go, sensitivity and independence and absent-default and none-plane rules pinned, section 17 present');
if (ew.length) console.log('  ' + ew.join('\n  '));

const mtp = masterTableProblems(v);
check(mtp.length === 0, 'master table: 28 cells (seven schemas x four elements), closed three-state census, evidence rules, pointer sections substantive');
if (mtp.length) console.log('  ' + mtp.join('\n  '));

console.log(problems.length === 0 ? 'SCHEMA-V2: ALL GREEN' : 'SCHEMA-V2: ' + problems.length + ' PROBLEM(S)');
if (problems.length) process.exit(1);

if (process.argv.includes('--selftest')) {
  // Negative control: shipped file passes everything (proven by rc above).
  // Positive controls: the shipped predicates must catch each mutation.
  const cases = [
    ['pending slot loses its pointer', (t) => t.replace('schema: evidence\nstatus: complete', 'schema: evidence\nstatus: pending'), (t) => slotCensusProblems(t)],
    ['complete slot gains a stray pointer', (t) => t.replace('schema: decision\nstatus: complete', 'schema: decision\nstatus: complete\nplanned_slice: W9.9'), (t) => slotCensusProblems(t)],
    ['vocabulary loses a token', (t) => t.replace('decision_vocabulary: allow, ask, would_block', 'decision_vocabulary: allow, ask'), (t) => (findKey(t, 'decision_vocabulary') !== 'allow, ask, would_block' ? ['drift'] : [])],
    ['element body gutted to a slogan', (t) => t.replace(/#### Migration\n[\s\S]*?#### Validation/, '#### Migration\nnone owed\n#### Validation'), (t) => elementProblems(t)],
    ['intent anchor line renamed', (t) => t.replace('\nExpected Outcome\n', '\nExpected Result\n'), (t) => intentProblems(t, goSrc)],
    ['intent count line drifts', (t) => t.replace('intent_field_count: 10', 'intent_field_count: 9'), (t) => intentProblems(t, goSrc)],
    ['origin token drifts', (t) => t.replace('agent_provided, ui_generated', 'agent_assumed, ui_generated'), (t) => authorityProblems(t, goSrc)],
    ['enforcement plane pre-borrowed', (t) => t.replace('authority_enforcement_plane: none-in-observation-phase', 'authority_enforcement_plane: enforce-now'), (t) => authorityProblems(t, goSrc)],
    ['recovery anchor line respelled', (t) => t.replace('\nEXTERNAL COMPENSATION\n', '\nEXTERNAL REWARD\n'), (t) => recoveryProblems(t, irSrc)],
    ['recovery execution plane pre-borrowed', (t) => t.replace('recovery_execution_plane: none-in-observation-phase', 'recovery_execution_plane: rollback-armed'), (t) => recoveryProblems(t, irSrc)],
    ['blast scope token drifts from Go', (t) => t.replace('credential_scope, device_scope', 'secret_scope, device_scope'), (t) => impactProblems(t, irSrc)],
    ['impact count line drifts', (t) => t.replace('impact_field_count: 7', 'impact_field_count: 6'), (t) => impactProblems(t, irSrc)],
    ['evidence anchor line respelled', (t) => t.replace('\nIntegrity Hash\n', '\nIntegrity Checksum\n'), (t) => evidenceProblems(t, epSrc)],
    ['evidence requiredness pre-borrowed', (t) => t.replace('evidence_required_semantics: all-seven-required-for-key-evidence', 'evidence_required_semantics: optional-observations'), (t) => evidenceProblems(t, epSrc)],
    ['profile personal count drifts', (t) => t.replace('profile_personal_field_count: 12', 'profile_personal_field_count: 11'), (t) => profileProblems(t, epSrc)],
    ['profile hard boundary pre-borrowed', (t) => t.replace('profile_hard_boundary_rule: never-replaces-hard-security-boundaries', 'profile_hard_boundary_rule: may-override-boundaries-on-high-trust'), (t) => profileProblems(t, epSrc)],
    ['profile agent anchor line dropped', (t) => t.replace('\nTrust Decay\n', '\n'), (t) => profileProblems(t, epSrc)],
    ['master cell dropped', (t) => t.replace('master_cell: decision/version\nstate: this_wave\nevidence: created by W2.1\n', ''), (t) => masterTableProblems(t)],
    ['master cell duplicated', (t) => t.replace('master_cell: decision/compatibility', 'master_cell: decision/version'), (t) => masterTableProblems(t)],
    ['master cell state wild', (t) => t.replace('state: this_wave\nevidence: created by W2.1', 'state: probably_done\nevidence: created by W2.1'), (t) => masterTableProblems(t)],
    ['this_wave cell loses creator token', (t) => t.replace('evidence: created by W2.1', 'evidence: created during the wave'), (t) => masterTableProblems(t)],
    ['planned cell loses W commitment', (t) => t.replace('master_cell: evidence/version\nstate: this_wave\nevidence: created by W2.4', 'master_cell: evidence/version\nstate: planned_build\nevidence: someday'), (t) => masterTableProblems(t)],
    ['master cell pointer hollowed', (t) => t.replace(/#### Version\n[\s\S]*?#### Compatibility/, '#### Version\nslogan only\n#### Compatibility'), (t) => masterTableProblems(t)],
    ['alignment anchor line respelled', (t) => t.replace('\nUNCERTAIN\n', '\nDOUBTFUL\n'), (t) => alignmentProblems(t, aaSrc)],
    ['alignment malicious equation pre-borrowed', (t) => t.replace('alignment_malicious_rule: uncertain-is-not-malicious-unrelated-is-not-malicious', 'alignment_malicious_rule: uncertain-means-malicious'), (t) => alignmentProblems(t, aaSrc)],
    ['alignment escalation precondition wired', (t) => t.replace('alignment_escalation_precondition: recorded-not-enforced', 'alignment_escalation_precondition: enforce-now'), (t) => alignmentProblems(t, aaSrc)],
    ['consume forge rule pre-borrowed', (t) => t.replace('origin_forge_rule: untrusted-source-never-claims-user_direct', 'origin_forge_rule: untrusted-may-claim-user-direct-on-high-trust'), (t) => consumeProblems(t, ucSrc)],
    ['consume enforcement plane pre-borrowed', (t) => t.replace('consume_enforcement_plane: none-in-observation-phase', 'consume_enforcement_plane: enforce-now'), (t) => consumeProblems(t, ucSrc)],
    ['consume stickiness rule flipped', (t) => t.replace('sticky_clearance_rule: consumption-never-clears-an-existing-untrusted-mark', 'sticky_clearance_rule: consumption-clears-marks-on-trusted-write'), (t) => consumeProblems(t, ucSrc)],
    ['inflow channel vocabulary drifts', (t) => t.replace('inflow_channel_vocabulary: cli_file,hook_task,absent_not_reported', 'inflow_channel_vocabulary: cli_file,hook_task'), (t) => inflowProblems(t, iiSrc)],
    ['inflow absent default pre-borrowed into fabrication', (t) => t.replace('inflow_absent_default: not-reported-is-known-gap-never-fabricated', 'inflow_absent_default: fill-in-a-plausible-default-when-missing'), (t) => inflowProblems(t, iiSrc)],
    ['inflow enforcement plane pre-borrowed', (t) => t.replace('inflow_enforcement_plane: none-in-observation-phase', 'inflow_enforcement_plane: enforce-now'), (t) => inflowProblems(t, iiSrc)],
    ['data action vocabulary drifts', (t) => t.replace('data_action_vocabulary: discover,read,write,modify,delete,execute,export,share,persist', 'data_action_vocabulary: read,write,export'), (t) => dataActionProblems(t, daSrc)],
    ['data action absent default pre-borrowed into a default class', (t) => t.replace('data_action_absent_default: absent-means-unclassified-legacy-never-inferred', 'data_action_absent_default: default-to-read-when-absent'), (t) => dataActionProblems(t, daSrc)],
    ['data action enforcement plane pre-borrowed', (t) => t.replace('data_action_enforcement_plane: none-in-observation-phase', 'data_action_enforcement_plane: enforce-now'), (t) => dataActionProblems(t, daSrc)],
    ['trust domain projection rewritten into a lift rule', (t) => t.replace('trust_domain_legacy_projection: s0_normal=low,s1_private=medium,s2_sensitive=medium,s3_credential=high,s4_security_boundary=high', 'trust_domain_legacy_projection: low=s0_normal,medium=s1_private,high=s3_credential'), (t) => trustDomainProblems(t, tdSrc)],
    ['trust domain absent default pre-borrowed into an upgrade', (t) => t.replace('trust_domain_absent_default: absent-means-unclassified-legacy-never-inferred', 'trust_domain_absent_default: default-to-s0-normal-when-absent'), (t) => trustDomainProblems(t, tdSrc)],
    ['trust domain enforcement plane pre-borrowed', (t) => t.replace('trust_domain_enforcement_plane: none-in-observation-phase', 'trust_domain_enforcement_plane: enforce-now'), (t) => trustDomainProblems(t, tdSrc)],
    ['export wrap vocabulary loses a word', (t) => t.replace('export_packaging_word_vocabulary: compress,encode,encrypt,archive,copy,upload,send,share', 'export_packaging_word_vocabulary: compress,encode,archive,copy,upload,send,share'), (t) => exportWrapProblems(t, ewSrc)],
    ['export independence rule rewritten into carry-over', (t) => t.replace('export_rejudgement_independence_rule: read-allow-never-carries-to-export', 'export_rejudgement_independence_rule: read-allow-carries-to-export'), (t) => exportWrapProblems(t, ewSrc)],
    ['export rejudgement enforcement plane pre-borrowed', (t) => t.replace('export_rejudgement_enforcement_plane: none-in-observation-phase', 'export_rejudgement_enforcement_plane: enforce-now'), (t) => exportWrapProblems(t, ewSrc)],
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
    (t) => intentProblems(t, goSrc), (t) => authorityProblems(t, goSrc),
    (t) => impactProblems(t, irSrc), (t) => recoveryProblems(t, irSrc), () => execVerbProblems(),
    (t) => evidenceProblems(t, epSrc), (t) => profileProblems(t, epSrc),
    (t) => alignmentProblems(t, aaSrc),
    (t) => masterTableProblems(t)]) {
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
