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
});
