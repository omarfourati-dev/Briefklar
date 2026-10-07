import { redactManually, restore, restoreExplanation } from './redaction';
import { Explanation } from '../core/models';

describe('redaction helpers', () => {
  it('restores placeholders', () => {
    expect(restore('Hallo [NAME_1], [NAME_1]!', [{ placeholder: '[NAME_1]', kind: 'NAME', value: 'Karim' }]))
      .toBe('Hallo Karim, Karim!');
  });

  it('redacts manually marked words with continuing numbers', () => {
    const r = redactManually('Termin bei [MANUELL_1] mit Herrn Okafor, Okafor bestätigt.', ['Okafor', 'Okafor', ' ']);
    expect(r.text).toBe('Termin bei [MANUELL_1] mit Herrn [MANUELL_2], [MANUELL_2] bestätigt.');
    expect(r.findings).toEqual([{ placeholder: '[MANUELL_2]', kind: 'MANUELL', value: 'Okafor' }]);
  });

  it('does not redact parts of placeholders', () => {
    const r = redactManually('Sehr geehrte [NAME_1]', ['NAME']);
    expect(r.text).toBe('Sehr geehrte [NAME_1]');
  });

  it('restores every text field of an explanation', () => {
    const e: Explanation = {
      authority: 'Amt', letterType: 'Brief', deadline: null, deadlineText: '', urgency: 'green',
      summary: { de: 'an [NAME_1]', en: 'to [NAME_1]', fr: '[NAME_1]', ar: '[NAME_1]' },
      actions: [{ de: '[IBAN_1]', en: '', fr: '', ar: '' }], replyDraft: 'Gruß [NAME_1]', missingInfo: ['[NAME_1]?'],
    };
    const r = restoreExplanation(e, [
      { placeholder: '[NAME_1]', kind: 'NAME', value: 'Karim' },
      { placeholder: '[IBAN_1]', kind: 'IBAN', value: 'DE89' },
    ]);
    expect(r.summary.ar).toBe('Karim');
    expect(r.actions[0].de).toBe('DE89');
    expect(r.replyDraft).toBe('Gruß Karim');
    expect(r.missingInfo[0]).toBe('Karim?');
  });

  it('restores in a single pass', () => {
    const f = [
      { placeholder: '[NAME_1]', kind: 'NAME', value: 'Anna' },
      { placeholder: '[NAME_10]', kind: 'NAME', value: 'Zoe' },
    ];
    expect(restore('[NAME_1] und [NAME_10]', f)).toBe('Anna und Zoe');
    const g = [
      { placeholder: '[NAME_1]', kind: 'NAME', value: '[NAME_2]' },
      { placeholder: '[NAME_2]', kind: 'NAME', value: 'Bob' },
    ];
    expect(restore('[NAME_1]', g)).toBe('[NAME_2]');
  });

  it('never touches placeholders through partial words', () => {
    for (const w of ['NAME_1', '_1', 'NAME']) {
      expect(redactManually('Sehr geehrte [NAME_1]', [w]).text).toBe('Sehr geehrte [NAME_1]');
    }
  });

  it('redacts longest words first', () => {
    const r = redactManually('Herr Okafor und Okafor', ['Okafor', 'Herr Okafor']);
    expect(r.text).toBe('[MANUELL_1] und [MANUELL_2]');
    expect(r.findings.map((x) => x.value)).toEqual(['Herr Okafor', 'Okafor']);
  });
});

describe('redactManually word boundaries', () => {
  it('matches whole words only, including one-character words', () => {
    const r = redactManually('Okafor und Okafors, 5 von 15 Äpfeln', ['Okafor', '5']);
    expect(r.text).toBe('[MANUELL_1] und Okafors, [MANUELL_2] von 15 Äpfeln');
    expect(r.findings.map((f) => f.value).sort()).toEqual(['5', 'Okafor']);
  });
});
