// "Datenschutzeinstellungen" in the landing page footer: forgets the stored ad-consent decision and reloads, so Google's
// consent message appears again like on a first visit – visitors can change or withdraw their consent at any time
// (AdSense on the landing page, see omarfourati.de/de/datenschutz). googlefc.showRevocationMessage() was not reliable.
const CONSENT_COOKIES = ['FCCDCF', 'FCNEC'];

document.querySelector('[data-privacy-settings]')?.addEventListener('click', () => {
  const parts = location.hostname.split('.');
  const domains = [''];
  for (let k = 0; k < parts.length - 1; k++) domains.push(`; domain=.${parts.slice(k).join('.')}`);
  for (const name of CONSENT_COOKIES) for (const d of domains) document.cookie = `${name}=; Max-Age=0; path=/${d}`;
  location.reload();
});
