import { siteHome } from './site';

describe('siteHome', () => {
  it('leads from the app origin to the landing page on the site', () => {
    expect(siteHome({ protocol: 'https:', host: 'app.briefklar.omarfourati.de' })).toBe('https://briefklar.omarfourati.de/');
    expect(siteHome({ protocol: 'http:', host: 'app.localhost:8080' })).toBe('http://localhost:8080/');
  });

  it('stays on the same origin without a separate app origin (local development)', () => {
    expect(siteHome({ protocol: 'http:', host: 'localhost:4200' })).toBe('/');
    expect(siteHome({ protocol: 'https:', host: 'briefklar.omarfourati.de' })).toBe('/');
  });
});
