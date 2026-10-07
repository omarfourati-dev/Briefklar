import { Explanation } from '../core/models';

export interface Example { id: string; title: string; letter: string; explanation: Explanation; }

export const EXAMPLES: Example[] = [
  {
    id: 'auslaenderbehoerde',
    title: 'Ausländerbehörde: Verlängerung des Aufenthaltstitels',
    letter: `Landkreis Musterland – Ausländerbehörde
Herrn Max Beispiel, Musterweg 1, 12345 Musterstadt

Ihr Aufenthaltstitel nach § 18b AufenthG läuft am 30.11.2026 ab.
Für die Verlängerung reichen Sie bitte bis zum 15.11.2026 folgende Unterlagen ein:
aktueller Arbeitsvertrag, Gehaltsnachweise der letzten drei Monate, Nachweis der Krankenversicherung,
ein biometrisches Passfoto. Bitte vereinbaren Sie danach online einen Termin zur Vorsprache.
Die Gebühr beträgt 93,00 Euro.`,
    explanation: {
      authority: 'Ausländerbehörde, Landkreis Musterland',
      letterType: 'Aufforderung: Unterlagen für die Verlängerung',
      deadline: '2026-11-15',
      deadlineText: 'Unterlagen bis 15.11.2026, Titel läuft am 30.11.2026 ab',
      urgency: 'yellow',
      summary: {
        de: 'Ihre Aufenthaltserlaubnis läuft am 30.11.2026 ab. Damit sie verlängert wird, schicken Sie bis zum 15.11.2026 vier Unterlagen. Danach buchen Sie online einen Termin. Die Verlängerung kostet 93 €.',
        en: 'Your residence permit expires on 30 Nov 2026. To renew it, send four documents by 15 Nov 2026, then book an appointment online. The renewal costs €93.',
        fr: 'Votre titre de séjour expire le 30/11/2026. Pour le renouveler, envoyez quatre documents avant le 15/11/2026, puis prenez rendez-vous en ligne. Le renouvellement coûte 93 €.',
        ar: 'تنتهي إقامتك في 30‏/11‏/2026. لتجديدها أرسل أربعة مستندات قبل 15‏/11‏/2026، ثم احجز موعدًا عبر الإنترنت. تكلفة التجديد 93 يورو.',
      },
      actions: [
        { de: 'Arbeitsvertrag kopieren', en: 'Copy your employment contract', fr: 'Copier votre contrat de travail', ar: 'انسخ عقد العمل' },
        { de: 'Gehaltsnachweise der letzten 3 Monate beilegen', en: 'Add payslips of the last 3 months', fr: 'Joindre les fiches de paie des 3 derniers mois', ar: 'أرفق كشوف الرواتب لآخر 3 أشهر' },
        { de: 'Nachweis der Krankenversicherung anfordern', en: 'Request proof of health insurance', fr: 'Demander une attestation d’assurance maladie', ar: 'اطلب إثبات التأمين الصحي' },
        { de: 'Biometrisches Passfoto machen lassen', en: 'Get a biometric passport photo', fr: 'Faire une photo d’identité biométrique', ar: 'التقط صورة بيومترية للجواز' },
        { de: 'Alles bis 15.11.2026 einreichen und Termin buchen', en: 'Submit everything by 15 Nov 2026 and book an appointment', fr: 'Tout envoyer avant le 15/11/2026 et prendre rendez-vous', ar: 'قدّم كل شيء قبل 15‏/11‏/2026 واحجز موعدًا' },
      ],
      replyDraft: 'Sehr geehrte Damen und Herren,\n\nanbei sende ich Ihnen die angeforderten Unterlagen für die Verlängerung meines Aufenthaltstitels: Arbeitsvertrag, Gehaltsnachweise der letzten drei Monate, Nachweis der Krankenversicherung und ein biometrisches Passfoto.\n\nBitte bestätigen Sie mir den Eingang.\n\nMit freundlichen Grüßen\nMax Beispiel',
      missingInfo: ['Wie die Gebühr bezahlt wird, steht nicht im Brief.'],
    },
  },
  {
    id: 'finanzamt',
    title: 'Finanzamt: Belege für die Steuererklärung',
    letter: `Finanzamt Musterstadt
Einkommensteuer 2025 – Steuernummer 123/456/78901
Sehr geehrter Herr Beispiel,
zur Bearbeitung Ihrer Einkommensteuererklärung 2025 benötige ich noch Nachweise zu den geltend gemachten
Werbungskosten (Arbeitsmittel 1.240,00 Euro, Fortbildung 890,00 Euro). Bitte reichen Sie die Belege bis
zum 30.10.2026 ein. Ohne Nachweise kann ich die Kosten nicht berücksichtigen.`,
    explanation: {
      authority: 'Finanzamt Musterstadt',
      letterType: 'Rückfrage zur Einkommensteuererklärung 2025',
      deadline: '2026-10-30',
      deadlineText: 'Belege bis 30.10.2026',
      urgency: 'red',
      summary: {
        de: 'Das Finanzamt will Belege für zwei Ausgaben aus Ihrer Steuererklärung 2025 sehen: Arbeitsmittel (1.240 €) und Fortbildung (890 €). Schicken Sie die Rechnungen bis zum 30.10.2026. Sonst werden die Kosten nicht anerkannt und Sie bekommen weniger Geld zurück.',
        en: 'The tax office wants receipts for two expenses in your 2025 tax return: work equipment (€1,240) and training (€890). Send the invoices by 30 Oct 2026, otherwise they will not be counted and your refund will be lower.',
        fr: 'Le Finanzamt demande des justificatifs pour deux dépenses de votre déclaration 2025 : matériel de travail (1 240 €) et formation (890 €). Envoyez les factures avant le 30/10/2026, sinon elles ne seront pas prises en compte.',
        ar: 'يطلب مكتب الضرائب إثباتات لمصروفين في إقرارك الضريبي لعام 2025: أدوات العمل (1240 يورو) والتدريب (890 يورو). أرسل الفواتير قبل 30‏/10‏/2026، وإلا لن تُحتسب وسيقل المبلغ المسترد.',
      },
      actions: [
        { de: 'Rechnungen für Arbeitsmittel heraussuchen (1.240 €)', en: 'Find the invoices for work equipment (€1,240)', fr: 'Retrouver les factures du matériel (1 240 €)', ar: 'ابحث عن فواتير أدوات العمل (1240 يورو)' },
        { de: 'Rechnung und Teilnahmebescheinigung der Fortbildung (890 €)', en: 'Training invoice and certificate (€890)', fr: 'Facture et attestation de la formation (890 €)', ar: 'فاتورة وشهادة حضور التدريب (890 يورو)' },
        { de: 'Belege bis 30.10.2026 über ELSTER oder per Post senden', en: 'Send them by 30 Oct 2026 via ELSTER or post', fr: 'Les envoyer avant le 30/10/2026 via ELSTER ou par courrier', ar: 'أرسلها قبل 30‏/10‏/2026 عبر ELSTER أو بالبريد' },
      ],
      replyDraft: 'Sehr geehrte Damen und Herren,\n\nzu Ihrem Schreiben zur Einkommensteuererklärung 2025 (Steuernummer 123/456/78901) übersende ich Ihnen anbei die Belege für die Arbeitsmittel und die Fortbildung.\n\nMit freundlichen Grüßen\nMax Beispiel',
      missingInfo: [],
    },
  },
  {
    id: 'rundfunkbeitrag',
    title: 'Rundfunkbeitrag: Zahlungserinnerung',
    letter: `ARD ZDF Deutschlandradio Beitragsservice
Beitragsnummer 123 456 789
Zahlungserinnerung
Für den Zeitraum 01.07.2026 bis 30.09.2026 ist der Rundfunkbeitrag von 55,08 Euro noch offen.
Bitte überweisen Sie den Betrag bis zum 21.10.2026. Andernfalls erhalten Sie einen Festsetzungsbescheid,
für den ein Säumniszuschlag von 8,00 Euro anfällt.`,
    explanation: {
      authority: 'ARD ZDF Deutschlandradio Beitragsservice',
      letterType: 'Zahlungserinnerung',
      deadline: '2026-10-21',
      deadlineText: 'Zahlung bis 21.10.2026',
      urgency: 'red',
      summary: {
        de: 'Sie haben den Rundfunkbeitrag für Juli bis September 2026 noch nicht bezahlt: 55,08 €. Überweisen Sie das Geld bis zum 21.10.2026. Sonst kommen 8 € Strafgebühr dazu.',
        en: 'You have not paid the broadcasting fee for July–September 2026: €55.08. Transfer it by 21 Oct 2026, otherwise an €8 late fee is added.',
        fr: 'Vous n’avez pas payé la redevance audiovisuelle de juillet à septembre 2026 : 55,08 €. Virez le montant avant le 21/10/2026, sinon 8 € de pénalité s’ajoutent.',
        ar: 'لم تدفع رسوم البث لأشهر يوليو إلى سبتمبر 2026: ‏55٫08 يورو. حوّل المبلغ قبل 21‏/10‏/2026، وإلا ستُضاف غرامة تأخير قدرها 8 يورو.',
      },
      actions: [
        { de: '55,08 € mit Beitragsnummer als Verwendungszweck überweisen', en: 'Transfer €55.08 with your contribution number as reference', fr: 'Virer 55,08 € avec le numéro de cotisation en référence', ar: 'حوّل 55٫08 يورو واذكر رقم الاشتراك في سبب الدفع' },
        { de: 'Lastschrift einrichten, damit es nicht wieder passiert', en: 'Set up direct debit so it does not happen again', fr: 'Mettre en place un prélèvement automatique', ar: 'فعّل الخصم المباشر حتى لا يتكرر ذلك' },
      ],
      replyDraft: 'Sehr geehrte Damen und Herren,\n\nden offenen Betrag von 55,08 Euro (Beitragsnummer 123 456 789) habe ich heute überwiesen. Bitte erteilen Sie mir künftig ein SEPA-Lastschriftmandat-Formular.\n\nMit freundlichen Grüßen\nMax Beispiel',
      missingInfo: [],
    },
  },
];
