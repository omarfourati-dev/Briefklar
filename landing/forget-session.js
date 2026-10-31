// The app lives on its own origin (app.briefklar.omarfourati.de), away from the AdSense scripts of this page. A tab that
// was logged in before the move may still hold the session here: drop it before the ad script runs (loaded
// synchronously in <head>, before adsbygoogle.js).
try {
  sessionStorage.removeItem('briefklar.session');
  localStorage.removeItem('briefklar.session');
} catch {
  // storage blocked (private mode, settings): nothing stored then either
}
