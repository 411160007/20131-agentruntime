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

function actionNecessityProblems(text, goText) {
  const bad = [];
  const rulePins = [
    ['action_necessity_danger_only_rule', 'ActionNecessityDangerOnlyRule', 'danger-judgement-alone-never-suffices'],
    ['action_necessity_evidence_rule', 'ActionNecessityEvidenceRule', 'every-recorded-answer-requires-nonempty-evidence'],
    ['action_necessity_absent_default', 'ActionNecessityAbsentDefault', 'absent-means-unassessed-never-inferred'],
    ['action_necessity_enforcement_plane', 'ActionNecessityEnforcementPlane', 'none-in-observation-phase'],
  ];
  const qs = findKey(text, 'action_necessity_question_vocabulary');
  if (qs === null) return ['action necessity contract keys missing or duplicated'];
  if (qs !== 'reasonable_step,necessary_step,substitutable_step') {
    bad.push('action necessity question line drifted from the closed three');
  }
  const as = findKey(text, 'action_necessity_answer_vocabulary');
  if (as === null) return ['action necessity contract keys missing or duplicated'];
  if (as !== 'supports,refutes,no_material') {
    bad.push('action necessity answer line drifted from the closed three');
  }
  for (const tok of (qs + ',' + as).split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('action necessity token Go mirror missing: ' + tok);
  }
  for (const [key, ident, val] of rulePins) {
    const got = findKey(text, key);
    if (got === null) return ['action necessity contract keys missing or duplicated'];
    if (got !== val) bad.push('action necessity line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) {
      bad.push('action necessity rule Go mirror drifted: ' + ident);
    }
  }
  if (text.indexOf('## 18. Action necessity record contract') < 0) bad.push('section 18 header missing');
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
const anSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'actionnecessity.go'), 'utf8');
const cdSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'chaindimensions.go'), 'utf8');
const dlSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'delegationobservation.go'), 'utf8');
const agSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'agencyguard.go'), 'utf8');
const cgSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'costguard.go'), 'utf8');
const dtSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'decisiontrace.go'), 'utf8');
const eeSrc = fs.readFileSync(path.join(root, 'internal', 'schema', 'evidenceexport.go'), 'utf8');
const evCliSrc = fs.readFileSync(path.join(root, 'cmd', 'agent-collector', 'evidence.go'), 'utf8');
const repCliSrc = fs.readFileSync(path.join(root, 'cmd', 'agent-collector', 'report.go'), 'utf8');
const sgSrc = fs.readFileSync(path.join(root, 'internal', 'auditlog', 'governance.go'), 'utf8');
const quotaSrc = fs.readFileSync(path.join(root, 'internal', 'auditlog', 'quota.go'), 'utf8');
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

// Behavior chain dimension contract checks (slice W5.2): the eight
// vocabulary tokens must derive mechanically from the section 242
// anchor bullets, the 8/8 coverage table must match the Go
// registration row by row (token, phase, anchor), and the computable
// subset line must equal the computable rows of that same table.
function chainDimensionProblems(text, goText) {
  const bad = [];
  const rules = [
    ['chain_dimension_coverage_rule', 'ChainDimensionCoverageRule', 'eight-of-eight-rows-registered-never-dangling'],
    ['chain_dimension_unclassified_rule', 'ChainDimensionUnclassifiedRule', 'missing-step-data-yields-unclassified-never-not-observed'],
    ['chain_dimension_reversibility_source_rule', 'ChainDimensionReversibilitySourceRule', 'reversibility-reduction-reads-recovery-classes-only'],
    ['chain_dimension_enforcement_plane', 'ChainDimensionEnforcementPlane', 'none-in-observation-phase'],
  ];
  const vocab = findKey(text, 'chain_dimension_vocabulary');
  if (vocab === null) return ['chain dimension vocabulary key missing or duplicated'];
  const toks = vocab.split(',');
  if (toks.length !== 8 || new Set(toks).size !== 8) bad.push('chain dimension vocabulary not eight unique tokens');
  for (const t of toks) {
    if (!goText.includes('"' + t + '"')) bad.push('chain dimension token Go mirror missing: ' + t);
  }
  const states = findKey(text, 'chain_dimension_observation_states');
  if (states === null) return bad.concat(['chain observation states key missing or duplicated']);
  if (states !== 'observed,not_observed,unclassified') bad.push('chain observation states line drifted');
  for (const tok of states.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('chain observation state Go mirror missing: ' + tok);
  }
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['chain contract keys missing or duplicated']);
    if (got !== val) bad.push('chain dimension line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) bad.push('chain dimension rule Go mirror drifted: ' + ident);
  }
  const secAt = text.indexOf('## 19. Behavior chain dimension record contract');
  if (secAt < 0) return bad.concat(['section 19 header missing']);
  const nextSec = text.indexOf('\n## ', secAt + 5);
  const sec = text.slice(secAt, nextSec < 0 ? text.length : nextSec);
  // Anchor reverse-check: the eight section 242 bullet lines derive
  // to the vocabulary, in order (lower case, spaces to underscores).
  const bullets = [...sec.matchAll(/^\* ([A-Z][A-Za-z ]+)$/gm)].map((m) => m[1]);
  if (bullets.length !== 8) bad.push('section 242 anchor bullets ' + bullets.length + ', want 8');
  else if (bullets.map((b) => b.toLowerCase().replace(/ +/g, '_')).join(',') !== vocab) {
    bad.push('section 242 anchor bullets derive to a different vocabulary than the key line');
  }
  // Go registration parse: constants and the coverage function rows.
  const dimOf = {};
  for (const m of goText.matchAll(/(Dimension\w+)\s+ChainDimension = "([a-z_]+)"/g)) dimOf[m[1]] = m[2];
  const phaseOf = {};
  for (const m of goText.matchAll(/(ChainPhase\w+)\s+ChainDimensionPhase = "([a-z_0-9]+)"/g)) phaseOf[m[1]] = m[2];
  const covBlock = (goText.match(/func chainDimensionCoverage\(\) \[\]ChainDimensionCoverageRow \{[\s\S]*?\n\}/) || [''])[0];
  const goRows = [...covBlock.matchAll(/\{(Dimension\w+), (ChainPhase\w+), "([^"]*)"\}/g)]
    .map((m) => [dimOf[m[1]], phaseOf[m[2]], m[3]]);
  if (goRows.length !== 8 || goRows.some((r) => !r[0] || !r[1])) bad.push('Go coverage registration ' + goRows.length + '/8 rows (or unresolved constant)');
  const goComputable = goRows.filter((r) => r[1] === 'computable_v0').map((r) => r[0]).join(',');
  const comp = findKey(text, 'chain_dimension_computable_v0');
  if (comp === null) return bad.concat(['chain computable subset key missing or duplicated']);
  if (comp !== goComputable) bad.push('computable subset line drifted from the Go registration');
  // Docs table pin: eight rows, row order and every cell against Go.
  const rows = [...sec.matchAll(/^\| `([a-z_]+)` \| (computable_v0|anchored|pending) \| (-|W5\.3|W6|W11) \|$/gm)];
  if (rows.length !== 8) bad.push('coverage table row census ' + rows.length + ', want 8');
  else rows.forEach((rm, i) => {
    const g = goRows[i];
    if (!g) { bad.push('no Go coverage row at ' + (i + 1)); return; }
    if (rm[1] !== g[0] || rm[2] !== g[1] || rm[3] !== (g[2] === '' ? '-' : g[2])) {
      bad.push('coverage row ' + (i + 1) + ' drifted from the Go registration: ' + rm[1]);
    }
  });
  if (goRows.length === 8 && rows.length === 8) {
    if (goRows.map((r) => r[0]).join(',') !== vocab) bad.push('Go coverage order drifted from the vocabulary line');
  }
  return bad;
}

// Delegation observation contract checks (slice W5.3): the five
// subject tokens and three revalidation states must mirror the Go
// vocabularies, both spec anchor bullet blocks (section 283 eight
// lines, section 236 five lines) must derive mechanically to the
// record field order and the subject vocabulary, the four rule
// constants must match Go verbatim, and the twelve-row field table
// must pin row order against the Go wire list.
function delegationProblems(text, goText) {
  const bad = [];
  const rules = [
    ['delegation_no_auto_inheritance_rule', 'DelegationNoAutoInheritanceRule', 'parent-authority-never-auto-transfers-to-child'],
    ['delegation_reevaluation_registry', 'DelegationReevaluationRegistry', 'identity,authority,capability,scope,data,risk,ttl'],
    ['delegation_revalidation_obligation_rule', 'DelegationRevalidationObligationRule', 'every-child-must-revalidate-own-permissions'],
    ['delegation_enforcement_plane', 'DelegationEnforcementPlane', 'none-in-observation-phase'],
  ];
  const subj = findKey(text, 'delegation_subject_vocabulary');
  if (subj === null) return ['delegation subject vocabulary key missing or duplicated'];
  const toks = subj.split(',');
  if (toks.length !== 5 || new Set(toks).size !== 5) bad.push('delegation subject vocabulary not five unique tokens');
  for (const t of toks) {
    if (!goText.includes('"' + t + '"')) bad.push('delegation subject token Go mirror missing: ' + t);
  }
  const states = findKey(text, 'delegation_revalidation_states');
  if (states === null) return bad.concat(['delegation revalidation states key missing or duplicated']);
  if (states !== 'revalidated,not_revalidated,unrecorded') bad.push('delegation revalidation states line drifted');
  for (const tok of states.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('delegation revalidation state Go mirror missing: ' + tok);
  }
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['delegation contract keys missing or duplicated']);
    if (got !== val) bad.push('delegation line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) bad.push('delegation rule Go mirror drifted: ' + ident);
  }
  const secAt = text.indexOf('## 20. Delegation observation record contract');
  if (secAt < 0) return bad.concat(['section 20 header missing']);
  const nextSec = text.indexOf('\n## ', secAt + 5);
  const sec = text.slice(secAt, nextSec < 0 ? text.length : nextSec);
  // Section 283 anchor reverse-check: the eight must-record bullet
  // lines derive to the first eight record field tokens, in order
  // (lower case, spaces to single underscores).
  const fieldList = (goText.match(/var DelegationRecordFieldWireNames = \[\]string\{([\s\S]*?)\n\}/) || ['', ''])[1];
  const goFields = (fieldList.match(/"[a-z_]+"/g) || []).map((s) => s.slice(1, -1));
  if (goFields.length !== 12) bad.push('Go record field wire list census ' + goFields.length + ', want 12');
  const spec8 = [...sec.matchAll(/^\* ([A-Z][A-Za-z ]+)$/gm)].map((m) => m[1]).slice(0, 8);
  if (spec8.length !== 8) bad.push('section 283 anchor bullets ' + spec8.length + ', want 8');
  else spec8.forEach((b, i) => {
    const d = b.toLowerCase().replace(/ +/g, '_');
    if (goFields[i] !== d) bad.push('section 283 bullet ' + (i + 1) + ' derives to ' + d + ', Go field is ' + goFields[i]);
  });
  // Section 236 subject reverse-check: the five listing bullets
  // derive to the subject vocabulary in order.
  const at236 = sec.indexOf('### 20.2');
  const sec236 = sec.slice(at236);
  const subjBullets = [...sec236.matchAll(/^\* Skill$/gm)].length ? [...sec236.matchAll(/^\* ([A-Z][A-Za-z ]*)$/gm)].map((m) => m[1]).slice(8) : [];
  if (subjBullets.length !== 5) bad.push('section 236 subject bullets ' + subjBullets.length + ', want 5');
  else if (subjBullets.map((b) => b.toLowerCase().replace(/ +/g, '_')).join(',') !== subj) {
    bad.push('section 236 subject bullets derive to a different vocabulary than the key line');
  }
  // Field table pin: twelve rows, row order against the Go list.
  const rows = [...sec.matchAll(/^\| `([a-z_]+)` \| (recorder|constructor-pinned) \|[^|\n]*\|$/gm)];
  if (rows.length !== 12) bad.push('delegation field table row census ' + rows.length + ', want 12');
  else rows.forEach((rm, i) => {
    if (rm[1] !== goFields[i]) bad.push('delegation field row-order pin broke at row ' + (i + 1) + ': ' + rm[1]);
    const wantOwner = i < 10 ? 'recorder' : 'constructor-pinned';
    if (rm[2] !== wantOwner) bad.push('delegation field ' + rm[1] + ' ownership drifted (want ' + wantOwner + ')');
  });
  return bad;
}

// Agency guard observation contract checks (slice W6.1): the ten
// section 261 bound-field words, three counter words, two scope
// words, and four limit-state words are closed vocabularies pinned
// docs<->Go; the conservation, enum-gate, and plane rules mirror
// the rule constants; the ten anchor bullets derive to the field
// vocabulary mechanically; and the 10/10 coverage table is pinned
// row by row against the Go registration.
function agencyGuardProblems(text, goText) {
  const bad = [];
  const rules = [
    ['agency_count_conservation_rule', 'AgencyCountConservationRule', 'sequences-one-to-N-events-never-lost-or-duplicated'],
    ['agency_action_enum_gate', 'AgencyActionEnumGate', 'limit-hold-cancel-recover-never-recorded-as-response'],
    ['agency_enforcement_plane', 'AgencyEnforcementPlane', 'none-in-observation-phase'],
  ];
  const fields = findKey(text, 'agency_guard_field_vocabulary');
  if (fields === null) return ['agency guard field vocabulary key missing or duplicated'];
  const toks = fields.split(',');
  if (toks.length !== 10 || new Set(toks).size !== 10) bad.push('agency guard field vocabulary not ten unique tokens');
  for (const t of toks) {
    if (!goText.includes('"' + t + '"')) bad.push('agency guard field token Go mirror missing: ' + t);
  }
  const counters = findKey(text, 'agency_counter_vocabulary');
  if (counters === null) return bad.concat(['agency counter vocabulary key missing or duplicated']);
  if (counters !== 'event_rate,step_count,parallelism') bad.push('agency counter vocabulary line drifted');
  for (const tok of counters.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('agency counter token Go mirror missing: ' + tok);
  }
  const scopes = findKey(text, 'agency_counter_scope_vocabulary');
  if (scopes === null) return bad.concat(['agency scope vocabulary key missing or duplicated']);
  if (scopes !== 'agent,task') bad.push('agency scope line drifted');
  for (const tok of scopes.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('agency scope token Go mirror missing: ' + tok);
  }
  const states = findKey(text, 'agency_limit_state_vocabulary');
  if (states === null) return bad.concat(['agency limit state vocabulary key missing or duplicated']);
  if (states !== 'below_ceiling,at_ceiling,above_ceiling') bad.push('agency limit state line drifted');
  for (const tok of states.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('agency limit state token Go mirror missing: ' + tok);
  }
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['agency guard contract keys missing or duplicated']);
    if (got !== val) bad.push('agency guard line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) bad.push('agency guard rule Go mirror drifted: ' + ident);
  }
  // Constructor-pinned response and registry lines: pinned in Go,
  // restated in the section body; the enum gate admits no action
  // token as a record value anywhere else.
  if (!goText.includes('AgencyGuardResponseRule = "record-only-no-action"')) bad.push('agency response constant Go mirror drifted');
  if (!goText.includes('AgencyLadderRegistry = "limit,hold,cancel-or-recover"')) bad.push('agency ladder registry Go mirror drifted');
  const secAt = text.indexOf('## 21. Agency guard observation record contract');
  if (secAt < 0) return bad.concat(['section 21 header missing']);
  const nextSec = text.indexOf('\n## ', secAt + 5);
  const sec = text.slice(secAt, nextSec < 0 ? text.length : nextSec);
  if (!sec.includes('record-only-no-action')) bad.push('section 21 body lost the record-only response restatement');
  // Anchor reverse-check: the ten section 261 bullet lines derive
  // to the field vocabulary, in order (lower case, spaces to
  // single underscores).
  const bullets = [...sec.matchAll(/^\* ([A-Z][A-Za-z ]+)$/gm)].map((m) => m[1]);
  if (bullets.length !== 10) bad.push('section 261 anchor bullets ' + bullets.length + ', want 10');
  else if (bullets.map((b) => b.toLowerCase().replace(/ +/g, '_')).join(',') !== fields) {
    bad.push('section 261 anchor bullets derive to a different vocabulary than the key line');
  }
  // Go registration parse: constants and the coverage function rows.
  const fieldOf = {};
  for (const m of goText.matchAll(/(Guard\w+)\s+AgencyGuardField = "([a-z_]+)"/g)) fieldOf[m[1]] = m[2];
  const phaseOf = {};
  for (const m of goText.matchAll(/(GuardPhase\w+)\s+AgencyGuardPhase = "([a-z_0-9]+)"/g)) phaseOf[m[1]] = m[2];
  const covBlock = (goText.match(/func agencyGuardCoverage\(\) \[\]AgencyGuardCoverageRow \{[\s\S]*?\n\}/) || [''])[0];
  const goRows = [...covBlock.matchAll(/\{(Guard\w+), (GuardPhase\w+), "([^"]*)"\}/g)]
    .map((m) => [fieldOf[m[1]], phaseOf[m[2]], m[3]]);
  if (goRows.length !== 10 || goRows.some((r) => !r[0] || !r[1])) bad.push('Go coverage registration ' + goRows.length + '/10 rows (or unresolved constant)');
  // Docs table pin: ten rows, row order and every cell against Go.
  const rows = [...sec.matchAll(/^\| `([a-z_]+)` \| (counted_v0|anchored|pending) \| (-|W6\.2|W8\.2) \|$/gm)];
  if (rows.length !== 10) bad.push('agency coverage table row census ' + rows.length + ', want 10');
  else rows.forEach((rm, i) => {
    const g = goRows[i];
    if (!g) { bad.push('no Go coverage row at ' + (i + 1)); return; }
    if (rm[1] !== g[0] || rm[2] !== g[1] || rm[3] !== (g[2] === '' ? '-' : g[2])) {
      bad.push('agency coverage row ' + (i + 1) + ' drifted from the Go registration: ' + rm[1]);
    }
  });
  if (goRows.length === 10 && rows.length === 10) {
    if (goRows.map((r) => r[0]).join(',') !== fields) bad.push('Go coverage order drifted from the field vocabulary line');
  }
  return bad;
}

// Cost guard observation contract checks (slice W6.2): the five
// proxy-field words, two shape words, three value-state words, and
// three collector-standing words are closed vocabularies pinned
// docs<->Go; the field-shape mapping line must equal the mapping
// derived mechanically from the Go coverage registration; the
// absence, truth-stance, and plane rules mirror the rule
// constants; and the 5/3 coverage table is pinned row by row
// against that registration.
function costGuardProblems(text, goText) {
  const bad = [];
  const rules = [
    ['cost_absence_rule', 'CostAbsenceRule', 'no-source-is-known-gap-never-fabricated-zero'],
    ['cost_truth_stance', 'CostTruthStance', 'cost-truth-lives-on-external-billing-plane-structurally-absent'],
    ['cost_enforcement_plane', 'CostGuardEnforcementPlane', 'none-in-observation-phase'],
  ];
  const fields = findKey(text, 'cost_proxy_field_vocabulary');
  if (fields === null) return ['cost proxy field vocabulary key missing or duplicated'];
  const toks = fields.split(',');
  if (toks.length !== 5 || new Set(toks).size !== 5) bad.push('cost proxy field vocabulary not five unique tokens');
  for (const t of toks) {
    if (!goText.includes('"' + t + '"')) bad.push('cost proxy field token Go mirror missing: ' + t);
  }
  const shapes = findKey(text, 'cost_proxy_shape_vocabulary');
  if (shapes === null) return bad.concat(['cost proxy shape vocabulary key missing or duplicated']);
  if (shapes !== 'count,duration') bad.push('cost proxy shape vocabulary line drifted');
  for (const tok of shapes.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('cost shape token Go mirror missing: ' + tok);
  }
  const states = findKey(text, 'cost_value_state_vocabulary');
  if (states === null) return bad.concat(['cost value state vocabulary key missing or duplicated']);
  if (states !== 'stated_observed,known_gap,gate_evidenced_zero') bad.push('cost value state line drifted');
  for (const tok of states.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('cost value state token Go mirror missing: ' + tok);
  }
  const standings = findKey(text, 'cost_collector_standing_vocabulary');
  if (standings === null) return bad.concat(['cost collector standing vocabulary key missing or duplicated']);
  if (standings !== 'stated_by_recorder,no_collector_known_gap,gate_evidenced_zero_line') bad.push('cost collector standing line drifted');
  for (const tok of standings.split(',')) {
    if (!goText.includes('"' + tok + '"')) bad.push('cost collector standing token Go mirror missing: ' + tok);
  }
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['cost guard contract keys missing or duplicated']);
    if (got !== val) bad.push('cost guard line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) bad.push('cost guard rule Go mirror drifted: ' + ident);
  }
  // Constructor-pinned response and evidence lines: pinned in Go,
  // restated in the section body.
  if (!goText.includes('CostGuardResponseRule = "record-only-no-action"')) bad.push('cost response constant Go mirror drifted');
  if (!goText.includes('CostZeroLLMStatement = "zero-llm-calls-in-decision-path-proven-by-gate"')) bad.push('cost zero-llm evidence constant Go mirror drifted');
  const secAt = text.indexOf('## 22. Cost guard observation record contract');
  if (secAt < 0) return bad.concat(['section 22 header missing']);
  const nextSec = text.indexOf('\n## ', secAt + 5);
  const sec = text.slice(secAt, nextSec < 0 ? text.length : nextSec);
  if (!sec.includes('record-only-no-action')) bad.push('section 22 body lost the record-only response restatement');
  if (!sec.includes('gate_evidenced_zero')) bad.push('section 22 body lost the gate-evidenced zero restatement');
  // Go registration parse: constants and the coverage rows, then
  // the mapping line must equal the mapping derived from them.
  const fieldOf = {};
  for (const m of goText.matchAll(/(CostProxy\w+)\s+CostProxyField = "([a-z_]+)"/g)) fieldOf[m[1]] = m[2];
  const shapeOf = {};
  for (const m of goText.matchAll(/(CostShape\w+)\s+CostProxyShape = "([a-z_]+)"/g)) shapeOf[m[1]] = m[2];
  const standOf = {};
  for (const m of goText.matchAll(/(CostCollector\w+)\s+CostCollectorStanding = "([a-z_]+)"/g)) standOf[m[1]] = m[2];
  const covBlock = (goText.match(/func costProxyCoverage\(\) \[\]CostProxyCoverageRow \{[\s\S]*?\n\}/) || [''])[0];
  const goRows = [...covBlock.matchAll(/\{(CostProxy\w+), (CostShape\w+), (CostCollector\w+)\}/g)]
    .map((m) => [fieldOf[m[1]], shapeOf[m[2]], standOf[m[3]]]);
  if (goRows.length !== 5 || goRows.some((r) => !r[0] || !r[1] || !r[2])) bad.push('Go coverage registration ' + goRows.length + '/5 rows (or unresolved constant)');
  const derived = goRows.map((r) => r[0] + '=' + r[1]).join(',');
  const mapping = findKey(text, 'cost_field_shape_mapping');
  if (mapping === null) return bad.concat(['cost field-shape mapping key missing or duplicated']);
  if (goRows.length === 5 && mapping !== derived) bad.push('cost field-shape mapping drifted from the Go coverage registration');
  // Docs table pin: five rows, row order and every cell against Go.
  const rows = [...sec.matchAll(/^\| `(runtime_duration_millis|billable_request_count|cpu_time_millis|memory_mib_millis|llm_invocation_count)` \| (count|duration) \| (stated_by_recorder|no_collector_known_gap|gate_evidenced_zero_line) \|$/gm)];
  if (rows.length !== 5) bad.push('cost coverage table row census ' + rows.length + ', want 5');
  else rows.forEach((rm, i) => {
    const g = goRows[i];
    if (!g) { bad.push('no Go coverage row at ' + (i + 1)); return; }
    if (rm[1] !== g[0] || rm[2] !== g[1] || rm[3] !== g[2]) {
      bad.push('cost coverage row ' + (i + 1) + ' drifted from the Go registration: ' + rm[1]);
    }
  });
  if (goRows.length === 5 && rows.length === 5) {
    if (goRows.map((r) => r[0]).join(',') !== fields) bad.push('Go coverage order drifted from the field vocabulary line');
  }
  return bad;
}

const PLANE_DIRS = ['policy', 'rules', 'bus', 'auditlog'];
function planeLeakProblems() {
  const bad = [];
  const needle = /GrantOrigin|AuthorityChain|IntentRecord|intentFieldWireNames|RecoveryClass|RecoveryRecord|ImpactRecord|BlastScope|BlastRadiusEstimate|blastScopeWireNames|impactFieldWireNames|EvidenceRecord|EvidenceField|evidenceFieldWireNames|PersonalProfileRecord|AgentProfileRecord|profilePersonalFieldWireNames|profileAgentFieldWireNames|profileScopeWireNames|ProfileScope|ActionClass|AlignmentRecord|alignmentClassWireNames|alignmentRecordFieldWireNames|AlignmentAbsentSemantics|AlignmentMaliciousRule|IntentModification|ApplyIntentModification|UntrustedConsumeRule|AuthorityPreservationRule|OriginForgeRule|StickyClearanceRule|ConsumeEnforcementPlane|IntentInflow|InflowSource|AllIntentInflowSources|InflowFromFile|InflowFromHookPayload|AbsentIntentRecord|HookTaskMappingRule|InflowAbsentDefault|InflowForgeRule|InflowEnforcementPlane|DataAction|AllDataActions|DataActionNonequivalenceRule|DataActionExportRejudgementRule|DataActionAbsentDefault|DataActionEnforcementPlane|TrustLevel|RunDomain|AllTrustLevels|AllRunDomains|LegacyClassOf|LiftCandidates|trustToLegacyClass|TrustDomainLegacyProjectionRule|TrustDomainCrossingRule|TrustDomainAbsentDefault|TrustDomainEnforcementPlane|PackagingWord|AllPackagingWords|ExportRejudgementRecord|BuildExportRejudgement|ExportPackagingSensitivityRule|ExportRejudgementIndependenceRule|ExportRejudgementAbsentDefault|ExportRejudgementEnforcementPlane|NecessityQuestion|NecessityAnswer|AllNecessityQuestions|AllNecessityAnswers|ActionNecessityRecord|BuildActionNecessity|ActionNecessityDangerOnlyRule|ActionNecessityEvidenceRule|ActionNecessityAbsentDefault|ActionNecessityEnforcementPlane|ChainDimension|ChainStep|ChainObservationState|ChainStateObserved|ChainStateNotObserved|ChainStateUnclassified|BuildChainDimensions|ComputableChainDimensions|AllChainDimensionCoverage|ChainPhaseComputable|ChainPhaseAnchored|ChainPhasePending|ChainDimensionCoverageRule|ChainDimensionUnclassifiedRule|ChainDimensionReversibilitySourceRule|ChainDimensionEnforcementPlane|DelegationSubject|DelegationRevalidationState|DelegationObservationInput|DelegationObservation|BuildDelegationObservation|AllDelegationSubjects|AllDelegationRevalidationStates|DelegationRecordFieldWireNames|DelegationNoAutoInheritanceRule|DelegationReevaluationRegistry|DelegationRevalidationObligationRule|DelegationEnforcementPlane/;
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
const an = actionNecessityProblems(v, anSrc);
check(an.length === 0, 'action necessity: three-question and three-answer vocabularies mirrored docs<->Go, danger-only and evidence and absent-default and none-plane rules pinned, section 18 present');
if (an.length) console.log('  ' + an.join('\n  '));
const cd = chainDimensionProblems(v, cdSrc);
check(cd.length === 0, 'chain dimensions: eight-word vocabulary derived from section 242 anchor bullets, 8/8 coverage table pinned row by row against the Go registration, computable subset and unclassified and reversibility-source and none-plane rules mirrored, section 19 present');
if (cd.length) console.log('  ' + cd.join('\n  '));
// decision trace field-table reverse-lookup (slice W7.1)
function decisionTraceProblems(text, goText) {
  const bad = [];
  const fv = findKey(text, 'decision_trace_field_vocabulary');
  if (fv === null) return ['decision trace field vocabulary key missing or duplicated'];
  const toks = fv.split(',');
  if (toks.length !== 15 || new Set(toks).size !== 15) bad.push('decision trace field vocabulary not fifteen unique tokens');
  const goFields = [...goText.matchAll(/Field\w+ +TraceField = "([a-z_]+)"/g)].map((m) => m[1]);
  if (goFields.length !== 15) bad.push('Go trace field constants census ' + goFields.length + ', want 15');
  toks.forEach((t, i) => {
    if (goFields[i] !== t) bad.push('trace field order pin broke at ' + (i + 1) + ': docs ' + t + ' vs Go ' + goFields[i]);
    if (!goText.includes('"' + t + '"')) bad.push('trace field token Go mirror missing: ' + t);
  });
  const sv = findKey(text, 'decision_trace_stance_vocabulary');
  if (sv === null) return bad.concat(['decision trace stance vocabulary key missing or duplicated']);
  const stances = sv.split(',');
  if (stances.length !== 4 || new Set(stances).size !== 4) bad.push('decision trace stance vocabulary not four unique tokens');
  const goStances = [...goText.matchAll(/Stance\w+ +TraceStance = "([a-z_]+)"/g)].map((m) => m[1]);
  if (goStances.length !== 4) bad.push('Go trace stance constants census ' + goStances.length + ', want 4');
  stances.forEach((s, i) => {
    if (goStances[i] !== s) bad.push('stance order pin broke at ' + (i + 1));
    if (!goText.includes('"' + s + '"')) bad.push('trace stance token Go mirror missing: ' + s);
  });
  const rules = [
    ['decision_trace_absence_rule', 'TraceAbsenceRule', 'field-without-value-is-absent-not-zero'],
    ['decision_trace_correlation_rule', 'TraceCorrelationRule', 'trace-pins-origin-event-id-not-equal-decision-id'],
    ['decision_trace_enforcement_mode', 'TraceEnforcementModeValue', 'record_only_phase0'],
    ['decision_trace_enforcement_plane', 'TraceEnforcementPlane', 'none-in-observation-phase'],
    ['decision_trace_default_applied_rule', 'TraceDefaultAppliedRule', 'empty-rule-means-default-applied'],
  ];
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['decision trace contract key missing or duplicated: ' + key]);
    if (got !== val) bad.push('decision trace line drifted: ' + key);
    if (!goText.includes(ident + ' = "' + val + '"')) bad.push('decision trace rule Go mirror drifted: ' + ident);
  }
  const sec23 = text.indexOf('## 23. Decision trace field table');
  if (sec23 === -1) return bad.concat(['docs section 23 missing']);
  const secText = text.slice(sec23);
  const rowRe = /^\| `([a-z_]+)` \| ([a-z_]+) \|.*\|$/gm;
  const rows = [];
  let mm;
  while ((mm = rowRe.exec(secText)) !== null) {
    if (toks.includes(mm[1])) rows.push([mm[1], mm[2]]);
  }
  if (rows.length !== 15) bad.push('decision trace table row census ' + rows.length + ', want 15');
  rows.forEach((r, i) => {
    if (r[0] !== toks[i]) bad.push('decision trace table row-order pin broke at row ' + (i + 1) + ': ' + r[0]);
    if (!stances.includes(r[1])) bad.push('decision trace table stance wild at row ' + (i + 1));
  });
  // stance counts: 6 line + 3 attrs + 2 additive + 4 gap
  const census = {};
  rows.forEach((r) => { census[r[1]] = (census[r[1]] || 0) + 1; });
  if (census.carried_on_audit_line !== 6 || census.carried_via_attrs !== 3 || census.additive_field !== 2 || census.known_gap_absent !== 4) {
    bad.push('decision trace stance census drifted: ' + JSON.stringify(census));
  }
  if (text.indexOf('## 23. Decision trace field table') === -1) bad.push('docs section 23 missing');
  return bad;
}

// storage governance v2 dual limits (slice W8.1)
function storageGovProblems(text, goText) {
  const bad = [];
  const goHas = (ident, val) => new RegExp(ident + '\\s*=\\s*"' + val + '"').test(goText);
  // retention mapping derived mechanically from the Go constants
  const rets = [...goText.matchAll(/StorageRetention(\w+)\s*=\s*"(\d+d)"/g)].map((m) => [m[1].toLowerCase(), m[2]]);
  if (rets.length !== 5) return bad.concat(['storage retention Go constant census ' + rets.length + ', want 5']);
  const derivedMap = rets.map((r) => r[0] + '=' + r[1]).join(',');
  const mapLine = findKey(text, 'storage_severity_retention_mapping');
  if (mapLine === null) return bad.concat(['storage retention mapping key missing or duplicated']);
  if (mapLine !== derivedMap) bad.push('storage retention mapping drifted from the Go constants: docs ' + mapLine + ' vs Go ' + derivedMap);
  // band vocabulary derived from the coverage registration
  const bandConst = {};
  for (const m of goText.matchAll(/(Band\w+)\s+StorageBand = "([a-z_]+)"/g)) bandConst[m[1]] = m[2];
  const covBlock = (goText.match(/func storageBandCoverage\(\) \[\]bandRow \{[\s\S]*?\n\}/) || [''])[0];
  const goBands = [...covBlock.matchAll(/\{(Band\w+), (\d+), StoragePressureResponse\}/g)]
    .map((m) => [bandConst[m[1]], Number(m[2])]);
  if (goBands.length !== 5 || goBands.some((r) => !r[0] || !r[1])) bad.push('Go band coverage registration ' + goBands.length + '/5 rows (or unresolved constant)');
  const bv = findKey(text, 'storage_pressure_band_vocabulary');
  if (bv === null) return bad.concat(['storage pressure band vocabulary key missing or duplicated']);
  if (goBands.length === 5 && bv !== goBands.map((r) => r[0]).join(',')) bad.push('storage band vocabulary drifted from the Go coverage registration');
  // closed action enum mirrored in Go
  const av = findKey(text, 'storage_govern_action_vocabulary');
  if (av === null) return bad.concat(['storage govern action vocabulary key missing or duplicated']);
  for (const tok of av.split(',')) if (!goText.includes('"' + tok + '"')) bad.push('govern action token Go mirror missing: ' + tok);
  // eight rule lines, docs<->Go one for one
  const rules = [
    ['storage_dual_limit_rule', 'StorageDualLimitRule', 'time-and-capacity-both-enforced-neither-infinite-growth'],
    ['storage_critical_hold_rule', 'StorageCriticalHoldRule', 'critical-classified-segments-never-pruned-by-capacity-pressure'],
    ['storage_pressure_response_rule', 'StoragePressureResponse', 'record-only-no-action'],
    ['storage_overquota_honesty_rule', 'StorageOverQuotaHonesty', 'when-only-held-segments-remain-report-over-quota-never-drop-critical'],
    ['storage_live_head_rule', 'StorageLiveHeadRule', 'governance-never-prunes-the-live-segment-empty-head-stays-valid'],
    ['storage_write_block_rule', 'StorageWriteBlockRule', 'pressure-never-blocks-writes-in-observation-phase'],
    ['storage_unreadable_rule', 'StorageUnreadableRule', 'unreadable-segment-is-held-never-guessed-pruned'],
    ['storage_quota_stance', 'StorageQuotaStance', 'tier-defaults-are-initial-engineering-defaults-not-permanent-commercial-promise'],
  ];
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['storage contract key missing or duplicated: ' + key]);
    if (got !== val) bad.push('storage line drifted: ' + key);
    if (!goHas(ident, val)) bad.push('storage rule Go mirror drifted: ' + ident);
  }
  const secAt = text.indexOf('## 26. Storage governance V2');
  if (secAt < 0) return bad.concat(['section 26 header missing']);
  const nextSec = text.indexOf('\n## ', secAt + 5);
  const sec = text.slice(secAt, nextSec < 0 ? text.length : nextSec);
  if (!sec.includes('record-only-no-action')) bad.push('section 26 body lost the record-only response restatement');
  if (!sec.includes('0953ac1')) bad.push('section 26 body lost the empty-head exemption lineage');
  const retRows = [...sec.matchAll(/^\| `(info|low|medium|high|critical)` \| (\d+d) \|$/gm)];
  if (retRows.length !== 5) bad.push('retention table row census ' + retRows.length + ', want 5');
  else retRows.forEach((rm, i) => {
    if (rm[1] !== rets[i][0] || rm[2] !== rets[i][1]) bad.push('retention row ' + (i + 1) + ' drifted from the Go constants: ' + rm[1]);
  });
  const bandRows = [...sec.matchAll(/^\| `(normal|compress_aggregate|strong_aggregation|evict_old_normal|retain_critical_only)` \| (\S+) \| record_only \|$/gm)];
  if (bandRows.length !== 5) bad.push('band table row census ' + bandRows.length + ', want 5');
  else bandRows.forEach((rm, i) => {
    if (!goBands[i] || rm[1] !== goBands[i][0]) bad.push('band row ' + (i + 1) + ' drifted from the Go coverage order');
  });
  return bad;
}


// per-scope quota observation reverse-lookup (slice W8.2)
function quotaProblems(text, goText) {
  const bad = [];
  const goHas = (ident, val) => new RegExp(ident + '\\s*=\\s*"' + val + '"').test(goText);
  const kinds = [...goText.matchAll(/QuotaKind(\w+)\s*=\s*"([a-z_]+)"/g)].map((m) => m[2]);
  if (kinds.length !== 2) return bad.concat(['quota kind Go census ' + kinds.length + ', want 2']);
  const kv = findKey(text, 'quota_kind_vocabulary');
  if (kv === null) return bad.concat(['quota kind vocabulary key missing or duplicated']);
  if (kv !== kinds.join(',')) bad.push('quota kind vocabulary drifted from the Go constants: docs ' + kv + ' vs Go ' + kinds.join(','));
  const states = [...goText.matchAll(/Quota\w+\s+QuotaState = "([a-z_]+)"/g)].map((m) => m[1]);
  if (states.length !== 3) return bad.concat(['quota state Go census ' + states.length + ', want 3']);
  const sm = findKey(text, 'quota_state_mirror');
  if (sm === null) return bad.concat(['quota state mirror key missing or duplicated']);
  if (sm !== states.join(',')) bad.push('quota state mirror drifted from the Go constants');
  const av = findKey(text, 'agency_limit_state_vocabulary');
  if (av !== null && av !== sm) bad.push('quota state mirror drifted from the section 21 closed three-state vocabulary');
  if (!goHas('QuotaScopeAgent', 'agent') || !goHas('QuotaScopeTask', 'task')) bad.push('quota scope token Go mirror missing');
  if (!goHas('QuotaTaskAttrKey', 'task_id')) bad.push('quota task attribution key drifted');
  const rules = [
    ['quota_scope_pair_rule', 'QuotaDualDimensionRule', 'per-agent-and-per-task-quoted-separately-never-averaged'],
    ['quota_shared_source_rule', 'QuotaSharedSourceRule', 'quota-recounts-stored-audit-lines-never-a-second-collector'],
    ['quota_critical_exempt_rule', 'QuotaCriticalExemptRule', 'critical-events-never-quota-dropped-report-watermark'],
    ['quota_unattributed_rule', 'QuotaUnattributedRule', 'line-without-stated-id-unattributed-never-defaulted'],
    ['quota_absent_ceiling_rule', 'QuotaAbsentCeilingRule', 'undeclared-ceiling-not-computed-stated-known-gap'],
  ];
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['quota contract key missing or duplicated: ' + key]);
    if (got !== val) bad.push('quota line drifted: ' + key);
    if (!goHas(ident, val)) bad.push('quota rule Go mirror drifted: ' + ident);
  }
  const rsp = findKey(text, 'quota_response_rule');
  if (rsp !== 'record-only-no-action') bad.push('quota response line drifted: ' + rsp);
  if (!/QuotaResponseRule\s*=\s*StoragePressureResponse/.test(goText)) bad.push('quota response must alias the shipped StoragePressureResponse token, never a second dialect');
  const wm = findKey(text, 'quota_watermark_token');
  if (wm !== 'over_quota_critical_only') bad.push('quota watermark token drifted: ' + wm);
  if (!/QuotaWatermarkToken\s*=\s*string\(GovernOverQuotaCritical\)/.test(goText)) bad.push('quota watermark must alias the shipped GovernOverQuotaCritical token, never a second dialect');
  const secAt = text.indexOf('## 27. Per-agent / per-task storage quota observation');
  if (secAt < 0) return bad.concat(['section 27 header missing']);
  const nextSec = text.indexOf('\n## ', secAt + 5);
  const sec = text.slice(secAt, nextSec < 0 ? text.length : nextSec);
  if (!sec.includes('record-only-no-action')) bad.push('section 27 body lost the record-only response restatement');
  if (!sec.includes('second collector')) bad.push('section 27 body lost the shared-source restatement');
  if (!sec.includes('task_id')) bad.push('section 27 body lost the stated task-attr attribution restatement');
  return bad;
}

// evidence export v0 reverse-lookup (slice W7.2)
function cliSurfaceProblems(text, evGo, repGo) {
  const bad = [];
  const bv = findKey(text, 'evidence_cli_skip_bucket_vocabulary');
  if (bv === null) return ['evidence CLI skip-bucket vocabulary key missing or duplicated'];
  const buckets = bv.split(',');
  if (buckets.length !== 3 || new Set(buckets).size !== 3) bad.push('evidence CLI skip buckets not three unique tokens');
  const decl = evGo.match(/var evidenceSkipBuckets = \[\]string\{([^\}]*)\}/);
  const goBuckets = decl ? [...decl[1].matchAll(/"([a-z_]+)"/g)].map((m) => m[1]) : [];
  if (goBuckets.length !== 3) bad.push('Go evidence CLI bucket census ' + goBuckets.length + ', want 3');
  buckets.forEach((b, i) => { if (goBuckets[i] !== b) bad.push('evidence CLI bucket order pin broke at ' + (i + 1) + ': docs ' + b + ' vs Go ' + goBuckets[i]); });
  const ruleLines = {
    evidence_cli_origin_rule: 'origin-is-preceding-same-agent-event-never-self-correlation',
    evidence_cli_version_rule: 'policy-version-must-be-operator-stated-never-defaulted',
    evidence_cli_write_rule: 'evidence-writes-only-into-existing-explicit-dir-never-overwrite',
    report_cli_zero_write_rule: 'report-subcommand-performs-filesystem-writes-zero',
    cli_surface_enforcement_plane: 'none-in-observation-phase',
  };
  for (const [k, want] of Object.entries(ruleLines)) {
    const got = findKey(text, k);
    if (got === null) { bad.push(k + ' missing or duplicated'); continue; }
    if (got !== want) bad.push(k + ' drifted: ' + got);
  }
  if (!/var evidenceSkipBuckets = \[\]string\{/.test(evGo)) bad.push('evidenceSkipBuckets declaration absent');
  if (!/-dir is required/.test(evGo)) bad.push('evidence -dir refusal string absent');
  if (!/-policy-version must be stated/.test(evGo)) bad.push('evidence -policy-version refusal string absent');
  if (!/func runEvidence/.test(evGo)) bad.push('runEvidence dispatch body absent');
  if (repGo.includes('os.WriteFile') || repGo.includes('os.Create') || repGo.includes('os.OpenFile')) bad.push('report subcommand contains a write API (zero-write rule broken)');
  if (!/func runReport/.test(repGo)) bad.push('runReport dispatch body absent');
  if (!text.includes('## 25. Read-only CLI surface')) bad.push('docs section 25 heading absent');
  return bad;
}

function evidenceExportProblems(text, goText) {
  const bad = [];
  const ruleRe = (ident, val) => new RegExp(ident + '\\s*=\\s*"' + val + '"').test(goText);
  const fv = findKey(text, 'evidence_export_format_vocabulary');
  if (fv === null) return ['evidence export format vocabulary key missing or duplicated'];
  const toks = fv.split(',');
  if (toks.length !== 5 || new Set(toks).size !== 5) bad.push('evidence format vocabulary not five unique tokens');
  const goFormats = [...goText.matchAll(/Format\w+ +EvidenceFormat = "([a-z_]+)"/g)].map((m) => m[1]);
  if (goFormats.length !== 5) bad.push('Go evidence format constants census ' + goFormats.length + ', want 5');
  toks.forEach((t, i) => { if (goFormats[i] !== t) bad.push('evidence format order pin broke at ' + (i + 1) + ': docs ' + t + ' vs Go ' + goFormats[i]); });
  const sv = findKey(text, 'evidence_format_stance_vocabulary');
  if (sv === null) return bad.concat(['evidence format stance vocabulary key missing or duplicated']);
  const stances = sv.split(',');
  if (stances.length !== 3 || new Set(stances).size !== 3) bad.push('evidence stance vocabulary not three unique tokens');
  const goStances = [...goText.matchAll(/Stance\w+ +EvidenceFormatStance = "([a-z_0-9]+)"/g)].map((m) => m[1]);
  if (goStances.length !== 3) bad.push('Go evidence stance constants census ' + goStances.length + ', want 3');
  stances.forEach((s, i) => { if (goStances[i] !== s) bad.push('evidence stance order pin broke at ' + (i + 1)); });
  const mv = findKey(text, 'evidence_bundle_member_vocabulary');
  if (mv === null) return bad.concat(['evidence bundle member vocabulary key missing or duplicated']);
  const members = mv.split(',');
  if (members.length !== 7 || new Set(members).size !== 7) bad.push('evidence member vocabulary not seven unique tokens');
  const goMembers = [...goText.matchAll(/Member\w+ +EvidenceBundleMember = "([a-z_.]+)"/g)].map((m) => m[1]);
  if (goMembers.length !== 7) bad.push('Go evidence member constants census ' + goMembers.length + ', want 7');
  members.forEach((mName, i) => { if (goMembers[i] !== mName) bad.push('evidence member order pin broke at ' + (i + 1) + ': docs ' + mName + ' vs Go ' + goMembers[i]); });
  const iv = findKey(text, 'evidence_integrity_field_vocabulary');
  if (iv === null) return bad.concat(['evidence integrity field vocabulary key missing or duplicated']);
  const fields = iv.split(',');
  if (fields.length !== 7 || new Set(fields).size !== 7) bad.push('evidence integrity vocabulary not seven unique tokens');
  const goFields = [...goText.matchAll(/Integrity\w+ +EvidenceIntegrityField = "([a-z_]+)"/g)].map((m) => m[1]);
  if (goFields.length !== 7) bad.push('Go evidence integrity constants census ' + goFields.length + ', want 7');
  fields.forEach((f, i) => { if (goFields[i] !== f) bad.push('evidence integrity order pin broke at ' + (i + 1)); });
  const rules = [
    ['evidence_timestamp_rule', 'EvidenceTimestampRule', 'envelope-timestamp-derived-from-evidence-never-wall-clock'],
    ['evidence_absent_cell_rule', 'EvidenceAbsentCellRule', 'csv-absent-cell-is-literal-absent-token-never-empty'],
    ['evidence_writeback_rule', 'EvidenceWriteBackRule', 'export-never-writes-back-to-source'],
    ['evidence_context_omission_rule', 'EvidenceContextRule', 'no-section251-field-dropped-from-export-for-convenience'],
    ['evidence_hash_rule', 'EvidenceHashRule', 'sha256-of-file-bytes-mismatch-is-red-never-repaired'],
    ['evidence_bundle_enforcement_plane', 'EvidenceBundleEnforcementPlane', 'none-in-observation-phase'],
  ];
  for (const [key, ident, val] of rules) {
    const got = findKey(text, key);
    if (got === null) return bad.concat(['evidence export contract key missing or duplicated: ' + key]);
    if (got !== val) bad.push('evidence export line drifted: ' + key);
    if (!ruleRe(ident, val)) bad.push('evidence export rule Go mirror drifted: ' + ident);
  }
  if (!/EvidenceAbsentCellToken = "absent"/.test(goText)) bad.push('absent-cell token Go constant drifted');
  const sec24 = text.indexOf('## 24. Evidence export v0 contract');
  if (sec24 === -1) return bad.concat(['docs section 24 missing']);
  const sec = text.slice(sec24);
  const fRows = [...sec.matchAll(/^\| `([a-z_.]+)` \| ([a-z_0-9]+) \|.*\|$/gm)].map((m) => [m[1], m[2]]);
  const fmtRows = fRows.filter((r) => toks.includes(r[0]));
  if (fmtRows.length !== 5) bad.push('evidence format table row census ' + fmtRows.length + ', want 5');
  fmtRows.forEach((r, i) => {
    if (r[0] !== toks[i]) bad.push('evidence format table row-order pin broke at row ' + (i + 1) + ': ' + r[0]);
    if (!stances.includes(r[1])) bad.push('evidence format table stance wild at row ' + (i + 1));
  });
  const memRows = fRows.filter((r) => members.includes(r[0]));
  if (memRows.length !== 7) bad.push('evidence member table row census ' + memRows.length + ', want 7');
  memRows.forEach((r, i) => {
    if (r[0] !== members[i]) bad.push('evidence member table row-order pin broke at row ' + (i + 1) + ': ' + r[0]);
    if (!stances.includes(r[1])) bad.push('evidence member table stance wild at row ' + (i + 1));
  });
  const intRows = [...sec.matchAll(/^\| `([a-z_]+)` \|[^|]*\|$/gm)].map((m) => m[1]).filter((n) => fields.includes(n));
  if (intRows.length !== 7) bad.push('evidence integrity table row census ' + intRows.length + ', want 7');
  intRows.forEach((n, i) => { if (n !== fields[i]) bad.push('evidence integrity row-order pin broke at ' + (i + 1) + ': ' + n); });
  const fc = {}; fmtRows.forEach((r) => { fc[r[1]] = (fc[r[1]] || 0) + 1; });
  if (fc.delivered_by_export_v0 !== 3 || fc.known_gap_absent !== 2 || fc.delivered_via_manifest_inline) bad.push('evidence format stance census drifted: ' + JSON.stringify(fc));
  const mc = {}; memRows.forEach((r) => { mc[r[1]] = (mc[r[1]] || 0) + 1; });
  if (mc.delivered_by_export_v0 !== 2 || mc.delivered_via_manifest_inline !== 1 || mc.known_gap_absent !== 4) bad.push('evidence member stance census drifted: ' + JSON.stringify(mc));
  return bad;
}

const dl = delegationProblems(v, dlSrc);
check(dl.length === 0, 'delegation observation: five-subject and three-state vocabularies mirrored docs<->Go, section 283 eight-bullet and section 236 five-bullet anchors derive mechanically, four rules and twelve-row field table pinned, none-plane, section 20 present');
const ag = agencyGuardProblems(v, agSrc);
check(ag.length === 0, 'agency guard counters: ten-field, three-counter, two-scope, and four-state vocabularies mirrored docs<->Go, section 261 ten-bullet anchor derives mechanically, conservation and enum-gate rules pinned, 10/10 coverage table row-pinned, none-plane, section 21 present');
const cg = costGuardProblems(v, cgSrc);
check(cg.length === 0, 'cost guard proxies: five-field, two-shape, three-state, and three-standing vocabularies mirrored docs<->Go, field-shape mapping derived from the coverage registration, absence and truth-stance rules pinned, 5/3 coverage table row-pinned, none-plane, section 22 present');
if (cg.length) console.log('  ' + cg.join('\n  '));
const dt = decisionTraceProblems(v, dtSrc);
check(dt.length === 0, 'decision trace field table: fifteen-token vocabulary mirrored docs<->Go in spec order, four-stance census 6/3/2/4 row-pinned, absence and correlation and pinned-mode and default-applied rules mirrored, docs section 23 present');
if (dt.length) console.log('  ' + dt.join('\n  '));
const cli = cliSurfaceProblems(v, evCliSrc, repCliSrc);
check(cli.length === 0, 'read-only CLI surface: evidence CLI three skip buckets mirrored docs<->Go in registration order, five rule lines plus section-25 heading pinned, report zero-write structural pin, none-plane');
if (cli.length) console.log('  ' + cli.join('\n  '));
const ee = evidenceExportProblems(v, eeSrc);
check(ee.length === 0, 'evidence export v0: five-format, three-stance, seven-member, and seven-integrity vocabularies mirrored docs<->Go in registration order, six rule lines plus absent-token pinned, format 3/2 and member 2/1/4 coverage censuses row-pinned, none-plane, docs section 24 present');
if (ee.length) console.log('  ' + ee.join('\n  '));
if (dl.length) console.log('  ' + dl.join('\n  '));

const sg = storageGovProblems(v, sgSrc);
check(sg.length === 0, 'storage governance v2: retention mapping derived from Go constants, band ladder derived from coverage registration, six actions mirrored, eight rule lines pinned docs<->Go, both tables row-pinned in order, section 26 present with record-only and empty-head restatements');
if (sg.length) console.log('  ' + sg.join('\n  '));

const qp = quotaProblems(v, quotaSrc);
check(qp.length === 0, 'per-scope quota observation: kind and state vocabularies derived from Go constants, state mirror pinned against the section 21 triad, five rule lines plus two aliased shipped tokens pinned docs<->Go, task attr key pinned, section 27 present with record-only, shared-source, and task-attribution restatements');
if (qp.length) console.log('  ' + qp.join('\n  '));

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
    ['action necessity question vocabulary loses a question', (t) => t.replace('action_necessity_question_vocabulary: reasonable_step,necessary_step,substitutable_step', 'action_necessity_question_vocabulary: reasonable_step,necessary_step'), (t) => actionNecessityProblems(t, anSrc)],
    ['action necessity evidence rule pre-borrowed into optional answers', (t) => t.replace('action_necessity_evidence_rule: every-recorded-answer-requires-nonempty-evidence', 'action_necessity_evidence_rule: evidence-optional-for-plausible-answers'), (t) => actionNecessityProblems(t, anSrc)],
    ['action necessity enforcement plane pre-borrowed', (t) => t.replace('action_necessity_enforcement_plane: none-in-observation-phase', 'action_necessity_enforcement_plane: enforce-now'), (t) => actionNecessityProblems(t, anSrc)],
    ['chain dimension vocabulary loses a dimension', (t) => t.replace('chain_dimension_vocabulary: intent_deviation,capability_escalation,data_sensitivity_escalation,trust_domain_crossing,reversibility_reduction,blast_radius_growth,destination_change,delegation_chain', 'chain_dimension_vocabulary: intent_deviation,capability_escalation,data_sensitivity_escalation,trust_domain_crossing,reversibility_reduction,blast_radius_growth,destination_change'), (t) => chainDimensionProblems(t, cdSrc)],
    ['chain coverage row upgraded into a false promise', (t) => t.replace('| `blast_radius_growth` | anchored | W11 |', '| `blast_radius_growth` | computable_v0 | - |'), (t) => chainDimensionProblems(t, cdSrc)],
    ['chain reversibility source rule rewritten to a rival scale', (t) => t.replace('chain_dimension_reversibility_source_rule: reversibility-reduction-reads-recovery-classes-only', 'chain_dimension_reversibility_source_rule: reversibility-reduction-reads-any-score'), (t) => chainDimensionProblems(t, cdSrc)],
    ['delegation subject vocabulary loses a word', (t) => t.replace('delegation_subject_vocabulary: skill,mcp,tool,child_process,plugin', 'delegation_subject_vocabulary: skill,mcp,tool,plugin'), (t) => delegationProblems(t, dlSrc)],
    ['delegation no-inheritance rule pre-borrowed into carry-over', (t) => t.replace('delegation_no_auto_inheritance_rule: parent-authority-never-auto-transfers-to-child', 'delegation_no_auto_inheritance_rule: parent-authority-carries-to-child'), (t) => delegationProblems(t, dlSrc)],
    ['delegation enforcement plane pre-borrowed', (t) => t.replace('delegation_enforcement_plane: none-in-observation-phase', 'delegation_enforcement_plane: enforce-now'), (t) => delegationProblems(t, dlSrc)],
    ['agency counter vocabulary loses a counter', (t) => t.replace('agency_counter_vocabulary: event_rate,step_count,parallelism', 'agency_counter_vocabulary: event_rate,step_count'), (t) => agencyGuardProblems(t, agSrc)],
    ['agency action enum gate pre-borrowed into enforcement', (t) => t.replace('agency_action_enum_gate: limit-hold-cancel-recover-never-recorded-as-response', 'agency_action_enum_gate: limit-hold-cancel-recover-recorded-as-response'), (t) => agencyGuardProblems(t, agSrc)],
    ['agency coverage counted row fakes an anchor', (t) => t.replace('| `max_steps` | counted_v0 | - |', '| `max_steps` | anchored | W6.2 |'), (t) => agencyGuardProblems(t, agSrc)],
    ['cost absence rule softened into a zero default', (t) => t.replace('cost_absence_rule: no-source-is-known-gap-never-fabricated-zero', 'cost_absence_rule: no-source-is-zero-by-default'), (t) => costGuardProblems(t, cgSrc)],
    ['cost truth given a fabricated number', (t) => t.replace('cost_truth_stance: cost-truth-lives-on-external-billing-plane-structurally-absent', 'cost_truth_stance: cost-truth-reported-as-zero-when-unbilled'), (t) => costGuardProblems(t, cgSrc)],
    ['cost coverage row loses its known-gap standing', (t) => t.replace('| `cpu_time_millis` | duration | no_collector_known_gap |', '| `cpu_time_millis` | duration | stated_by_recorder |'), (t) => costGuardProblems(t, cgSrc)],
    ['evidence envelope timestamp softened into wall-clock', (t) => t.replace('evidence_timestamp_rule: envelope-timestamp-derived-from-evidence-never-wall-clock', 'evidence_timestamp_rule: envelope-timestamp-is-wall-clock-now'), (t) => evidenceExportProblems(t, eeSrc)],
    ['evidence write-back rule rewritten into repair-on-export', (t) => t.replace('evidence_writeback_rule: export-never-writes-back-to-source', 'evidence_writeback_rule: export-may-rewrite-source-when-convenient'), (t) => evidenceExportProblems(t, eeSrc)],
    ['evidence member gap row fakes a delivered file', (t) => t.replace('| `events.jsonl` | known_gap_absent |', '| `events.jsonl` | delivered_by_export_v0 |'), (t) => evidenceExportProblems(t, eeSrc)],
    ['evidence CLI write rule softened into overwrite-ok', (t) => t.replace('evidence_cli_write_rule: evidence-writes-only-into-existing-explicit-dir-never-overwrite', 'evidence_cli_write_rule: evidence-may-overwrite-files-when-convenient'), (t) => cliSurfaceProblems(t, evCliSrc, repCliSrc)],
    ['report zero-write rule rewritten into temp-writes', (t) => t.replace('report_cli_zero_write_rule: report-subcommand-performs-filesystem-writes-zero', 'report_cli_zero_write_rule: report-may-write-temp-files'), (t) => cliSurfaceProblems(t, evCliSrc, repCliSrc)],
    ['trace field vocabulary loses a token', (t) => t.replace('enforcement_mode,outcome,recovery_state', 'enforcement_mode,outcome'), (t) => decisionTraceProblems(t, dtSrc)],
    ['trace stance census row faked', (t) => t.replace('| `task` | known_gap_absent |', '| `task` | carried_on_audit_line |'), (t) => decisionTraceProblems(t, dtSrc)],
    ['trace absence rule softened into a zero default', (t) => t.replace('decision_trace_absence_rule: field-without-value-is-absent-not-zero', 'decision_trace_absence_rule: absent-means-zero'), (t) => decisionTraceProblems(t, dtSrc)],
    ['storage pressure response softened into write-blocking', (t) => t.replace('storage_pressure_response_rule: record-only-no-action', 'storage_pressure_response_rule: block-writes-over-quota'), (t) => storageGovProblems(t, sgSrc)],
    ['storage critical hold flipped into evict-any', (t) => t.replace('storage_critical_hold_rule: critical-classified-segments-never-pruned-by-capacity-pressure', 'storage_critical_hold_rule: critical-segments-evictable-under-capacity-pressure'), (t) => storageGovProblems(t, sgSrc)],
    ['storage retention window shrinks critical below its class', (t) => t.replace('high=90d,critical=180d', 'high=90d,critical=7d'), (t) => storageGovProblems(t, sgSrc)],
    ['quota critical exemption flipped into silent drop', (t) => t.replace('quota_critical_exempt_rule: critical-events-never-quota-dropped-report-watermark', 'quota_critical_exempt_rule: critical-events-droppable-under-quota-pressure'), (t) => quotaProblems(t, quotaSrc)],
    ['quota shared source renamed into a second collector', (t) => t.replace('quota_shared_source_rule: quota-recounts-stored-audit-lines-never-a-second-collector', 'quota_shared_source_rule: quota-runs-a-parallel-event-collector'), (t) => quotaProblems(t, quotaSrc)],
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
    (t) => chainDimensionProblems(t, cdSrc),
    (t) => delegationProblems(t, dlSrc),
    (t) => agencyGuardProblems(t, agSrc),
    (t) => evidenceExportProblems(t, eeSrc),
    (t) => costGuardProblems(t, cgSrc),
    (t) => decisionTraceProblems(t, dtSrc),
    (t) => storageGovProblems(t, sgSrc),
    (t) => cliSurfaceProblems(t, evCliSrc, repCliSrc),
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
