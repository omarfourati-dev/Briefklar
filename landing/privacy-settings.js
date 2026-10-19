// "Datenschutzeinstellungen" in the landing page footer: shows Google's consent message again, so visitors can
// change or withdraw their ad consent at any time (AdSense on the landing page, see omarfourati.de/de/datenschutz).
document.querySelector('[data-privacy-settings]')?.addEventListener('click', () => {
  const fc = (window.googlefc = window.googlefc || {});
  fc.callbackQueue = fc.callbackQueue || [];
  fc.callbackQueue.push(() => window.googlefc.showRevocationMessage?.());
});
