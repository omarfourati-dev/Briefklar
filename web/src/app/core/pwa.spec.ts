import { describe, expect, it, vi } from 'vitest';
import { Pwa } from './pwa';

/** Fake ServiceWorkerContainer/Registration: enough events to walk through install → waiting → update. */
class FakeTarget extends EventTarget {}
function fakeWorker(state: string) {
  const w = Object.assign(new FakeTarget(), { state, postMessage: vi.fn() });
  return w as unknown as ServiceWorker & { state: string; postMessage: ReturnType<typeof vi.fn> };
}
function setup(opts: { controller: boolean; waiting?: boolean }) {
  const reg = Object.assign(new FakeTarget(), {
    waiting: opts.waiting ? fakeWorker('installed') : null,
    installing: null as ServiceWorker | null,
  });
  const container = Object.assign(new FakeTarget(), {
    controller: opts.controller ? fakeWorker('activated') : null,
    register: vi.fn(async () => reg),
  });
  const reload = vi.fn();
  const pwa = new Pwa(container as unknown as ServiceWorkerContainer, reload);
  return { reg, container, reload, pwa };
}

describe('Pwa', () => {
  it('registers the service worker for the app scope', async () => {
    const { container, pwa } = setup({ controller: false });
    await pwa.register();
    expect(container.register).toHaveBeenCalledWith('sw.js', { scope: './' });
    expect(pwa.updateReady()).toBe(false);
  });

  it('offers an update when a new worker is already waiting', async () => {
    const { pwa } = setup({ controller: true, waiting: true });
    await pwa.register();
    expect(pwa.updateReady()).toBe(true);
  });

  it('offers an update once a newly found worker is installed, but not on the very first install', async () => {
    for (const controller of [true, false]) {
      const { reg, pwa } = setup({ controller });
      await pwa.register();
      const worker = fakeWorker('installing');
      reg.installing = worker;
      reg.dispatchEvent(new Event('updatefound'));
      worker.state = 'installed';
      worker.dispatchEvent(new Event('statechange'));
      expect(pwa.updateReady()).toBe(controller);
    }
  });

  it('activates the waiting worker and reloads once it has taken over', async () => {
    const { reg, container, reload, pwa } = setup({ controller: true, waiting: true });
    await pwa.register();
    pwa.applyUpdate();
    expect(reg.waiting!.postMessage).toHaveBeenCalledWith('skip-waiting');
    expect(reload).not.toHaveBeenCalled();
    container.dispatchEvent(new Event('controllerchange'));
    container.dispatchEvent(new Event('controllerchange'));
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it('does not reload when the first install takes control of the page', async () => {
    const { container, reload, pwa } = setup({ controller: false });
    await pwa.register();
    container.dispatchEvent(new Event('controllerchange'));
    expect(reload).not.toHaveBeenCalled();
  });

  it('does nothing without service worker support', async () => {
    const pwa = new Pwa(undefined, vi.fn());
    await pwa.register();
    expect(pwa.updateReady()).toBe(false);
  });
});
