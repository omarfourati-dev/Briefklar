/**
 * The landing page. In production the app runs on its own origin (app.<site>), away from the AdSense scripts of the
 * landing page, so "home" is the site without "app."; without a separate app origin it is simply "/".
 */
export function siteHome(loc: Pick<Location, 'protocol' | 'host'> = location): string {
  return loc.host.startsWith('app.') ? `${loc.protocol}//${loc.host.slice('app.'.length)}/` : '/';
}
