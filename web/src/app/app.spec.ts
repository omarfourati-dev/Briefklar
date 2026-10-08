import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { vi } from 'vitest';
import { App } from './app';
import { Pwa } from './core/pwa';

describe('App', () => {
  const pwa = { updateReady: signal(false), offline: signal(false), register: vi.fn(async () => undefined), applyUpdate: vi.fn() };

  beforeEach(async () => {
    pwa.updateReady.set(false);
    pwa.offline.set(false);
    await TestBed.configureTestingModule({
      imports: [App], providers: [provideRouter([]), { provide: Pwa, useValue: pwa }],
    }).compileComponents();
  });

  it('should create the app without banners', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[role="status"]')).toBeNull();
  });

  it('shows the update banner and applies the update on click', () => {
    pwa.updateReady.set(true);
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Neue Version von Briefklar verfügbar.');
    el.querySelector('button')!.click();
    expect(pwa.applyUpdate).toHaveBeenCalled();
  });

  it('tells the user when the device is offline', () => {
    pwa.offline.set(true);
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Keine Verbindung');
  });
});
