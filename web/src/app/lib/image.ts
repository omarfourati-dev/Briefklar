/** Longest side of a photo we upload; more does not help OCR and costs bandwidth and server memory. */
export const MAX_LONG_SIDE = 2500;
const JPEG_QUALITY = 0.9;
const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp'];

/** Target size with the long side at most MAX_LONG_SIDE, aspect ratio kept, never upscaled. */
export function fitLongSide(width: number, height: number): { width: number; height: number } {
  const scale = Math.min(1, MAX_LONG_SIDE / Math.max(width, height));
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}

export function isImage(file: File): boolean {
  return IMAGE_TYPES.includes(file.type);
}

/**
 * Phone photos: apply the EXIF rotation and shrink to MAX_LONG_SIDE as JPEG. PDFs, and images the browser
 * cannot decode (or browsers without createImageBitmap), are uploaded unchanged – the server decides.
 */
export async function prepareUpload(file: File): Promise<File> {
  if (!isImage(file) || typeof globalThis.createImageBitmap !== 'function') return file;
  try {
    const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
    const { width, height } = fitLongSide(bitmap.width, bitmap.height);
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext('2d')!;
    ctx.fillStyle = '#ffffff'; // JPEG has no alpha: transparent PNG areas would turn black
    ctx.fillRect(0, 0, width, height);
    ctx.drawImage(bitmap, 0, 0, width, height);
    bitmap.close();
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', JPEG_QUALITY));
    if (!blob) return file;
    return new File([blob], file.name.replace(/\.[^.]*$/, '') + '.jpg', { type: 'image/jpeg' });
  } catch {
    return file;
  }
}
