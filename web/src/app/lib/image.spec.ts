import { MAX_LONG_SIDE, fitLongSide, isImage, prepareUpload } from './image';

describe('image', () => {
  it('keeps small images as they are', () => {
    expect(fitLongSide(1200, 1600)).toEqual({ width: 1200, height: 1600 });
    expect(fitLongSide(MAX_LONG_SIDE, 100)).toEqual({ width: MAX_LONG_SIDE, height: 100 });
  });

  it('scales the long side down to 2500 px and keeps the aspect ratio', () => {
    expect(fitLongSide(4000, 3000)).toEqual({ width: 2500, height: 1875 });
    expect(fitLongSide(3024, 4032)).toEqual({ width: 1875, height: 2500 });
    expect(fitLongSide(10000, 3)).toEqual({ width: 2500, height: 1 });
  });

  it('only treats JPEG, PNG and WebP as images', () => {
    expect(isImage(new File([], 'a.jpg', { type: 'image/jpeg' }))).toBe(true);
    expect(isImage(new File([], 'a.png', { type: 'image/png' }))).toBe(true);
    expect(isImage(new File([], 'a.webp', { type: 'image/webp' }))).toBe(true);
    expect(isImage(new File([], 'a.pdf', { type: 'application/pdf' }))).toBe(false);
    expect(isImage(new File([], 'a.gif', { type: 'image/gif' }))).toBe(false);
  });

  it('returns PDFs unchanged', async () => {
    const pdf = new File(['%PDF-1.4'], 'brief.pdf', { type: 'application/pdf' });
    expect(await prepareUpload(pdf)).toBe(pdf);
  });

  it('returns the original image when createImageBitmap is unavailable', async () => {
    const original = (globalThis as { createImageBitmap?: unknown }).createImageBitmap;
    (globalThis as { createImageBitmap?: unknown }).createImageBitmap = undefined;
    try {
      const photo = new File(['x'], 'foto.jpg', { type: 'image/jpeg' });
      expect(await prepareUpload(photo)).toBe(photo);
    } finally {
      (globalThis as { createImageBitmap?: unknown }).createImageBitmap = original;
    }
  });

  it('paints a white background before drawing (transparent PNG to JPEG) and uploads a scaled JPEG', async () => {
    const g = globalThis as { createImageBitmap?: unknown };
    const original = g.createImageBitmap;
    const calls: string[] = [];
    const ctx = {
      fillStyle: '',
      fillRect: (x: number, y: number, w: number, h: number) => calls.push(`fill ${ctx.fillStyle} ${x},${y},${w},${h}`),
      drawImage: (_b: unknown, x: number, y: number, w: number, h: number) => calls.push(`draw ${x},${y},${w},${h}`),
    };
    const getContext = vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(ctx as never);
    const toBlob = vi.spyOn(HTMLCanvasElement.prototype, 'toBlob')
      .mockImplementation((cb: BlobCallback, type?: string) => cb(new Blob(['jpg'], { type })));
    g.createImageBitmap = () => Promise.resolve({ width: 5000, height: 4000, close: () => undefined });
    try {
      const out = await prepareUpload(new File(['x'], 'scan.png', { type: 'image/png' }));
      expect(calls).toEqual(['fill #ffffff 0,0,2500,2000', 'draw 0,0,2500,2000']);
      expect(out.type).toBe('image/jpeg');
      expect(out.name).toBe('scan.jpg');
    } finally {
      g.createImageBitmap = original;
      getContext.mockRestore();
      toBlob.mockRestore();
    }
  });

  it('returns the original image when the browser cannot decode it', async () => {
    const original = (globalThis as { createImageBitmap?: unknown }).createImageBitmap;
    (globalThis as { createImageBitmap?: unknown }).createImageBitmap = () => Promise.reject(new Error('decode'));
    try {
      const photo = new File(['x'], 'foto.webp', { type: 'image/webp' });
      expect(await prepareUpload(photo)).toBe(photo);
    } finally {
      (globalThis as { createImageBitmap?: unknown }).createImageBitmap = original;
    }
  });
});
