import { Explanation, Finding, Texts } from '../core/models';

const PLACEHOLDER = /\[[A-Z_]+?_\d+\]/g;

export function restore(text: string, findings: Finding[]): string {
  return findings.reduce((t, f) => t.split(f.placeholder).join(f.value), text);
}

export function restoreExplanation(e: Explanation, findings: Finding[]): Explanation {
  const r = (s: string) => restore(s, findings);
  const texts = (t: Texts): Texts => ({ de: r(t.de), en: r(t.en), fr: r(t.fr), ar: r(t.ar) });
  return {
    ...e,
    authority: r(e.authority),
    letterType: r(e.letterType),
    deadlineText: r(e.deadlineText),
    summary: texts(e.summary),
    actions: e.actions.map(texts),
    replyDraft: r(e.replyDraft),
    missingInfo: e.missingInfo.map(r),
  };
}

/** Words the user clicked in the preview become [MANUELL_n]; numbering continues after existing ones. */
export function redactManually(text: string, words: string[]): { text: string; findings: Finding[] } {
  let next = Math.max(0, ...[...text.matchAll(/\[MANUELL_(\d+)\]/g)].map((m) => Number(m[1])));
  const findings: Finding[] = [];
  const unique = [...new Set(words.map((w) => w.trim()).filter((w) => w.length >= 2))].sort((a, b) => b.length - a.length);
  let out = text;
  for (const word of unique) {
    // split the text into placeholders and plain parts, replace only in plain parts
    const parts = out.split(PLACEHOLDER);
    const holders = out.match(PLACEHOLDER) ?? [];
    if (!parts.some((p) => p.includes(word))) continue;
    const placeholder = `[MANUELL_${++next}]`;
    findings.push({ placeholder, kind: 'MANUELL', value: word });
    out = parts.map((p, i) => p.split(word).join(placeholder) + (holders[i] ?? '')).join('');
  }
  return { text: out, findings };
}
