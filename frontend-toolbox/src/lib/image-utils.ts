// =============================================================================
// 图片处理工具库 —— 基于 Canvas 2D
// 注意：此模块依赖 DOM（Canvas），不可在 node --test 下直接测。
// =============================================================================

import { recompressPNG, quantizeAndEncodePNG } from './png-optimize.ts';

/** 按目标尺寸缩放，保持宽高比。 */
export function scaleFit(
  width: number,
  height: number,
  maxW: number,
  maxH: number,
): { width: number; height: number } {
  if (width <= maxW && height <= maxH) return { width, height };
  const ratio = Math.min(maxW / width, maxH / height);
  return { width: Math.round(width * ratio), height: Math.round(height * ratio) };
}

/** 加载图片文件为 HTMLImageElement。 */
export function loadImage(file: Blob): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('图片加载失败'));
    };
    img.src = url;
  });
}

export type ImageFormat =
  | 'image/png'
  | 'image/jpeg'
  | 'image/webp'
  | 'image/png-best'   // 自建 PNG 编码器，max-deflate + 最佳滤波
  | 'image/png8';       // 色板量化 ≤256 色，大幅缩小截图

/** 把 HTMLImageElement 画到 canvas 并导出 Blob。 */
export function imageToBlob(
  img: HTMLImageElement,
  opts: { format: ImageFormat; quality?: number; width?: number; height?: number },
): Promise<Blob> {
  const w = opts.width ?? img.naturalWidth;
  const h = opts.height ?? img.naturalHeight;
  const canvas = document.createElement('canvas');
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext('2d');
  if (!ctx) return Promise.reject(new Error('无法获取 2D 上下文'));
  // jpeg 需要白底（透明会变黑）
  if (opts.format === 'image/jpeg') {
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, w, h);
  }
  ctx.drawImage(img, 0, 0, w, h);
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (blob) resolve(blob);
        else reject(new Error('toBlob 返回空'));
      },
      opts.format,
      opts.quality,
    );
  });
}

/** 压缩图片：按最大尺寸 + 质量。返回新 Blob。 */
export async function compressImage(
  file: Blob,
  opts: { maxWidth?: number; maxHeight?: number; quality?: number; format?: ImageFormat },
): Promise<{ blob: Blob; width: number; height: number }> {
  const img = await loadImage(file);
  const maxW = opts.maxWidth ?? img.naturalWidth;
  const maxH = opts.maxHeight ?? img.naturalHeight;
  const { width, height } = scaleFit(img.naturalWidth, img.naturalHeight, maxW, maxH);
  const format = opts.format ?? (file.type === 'image/png' ? 'image/png' : 'image/jpeg');

  let blob: Blob;
  if (format === 'image/png-best') {
    try {
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext('2d');
      if (!ctx) throw new Error('无法获取 2D 上下文');
      ctx.drawImage(img, 0, 0, width, height);
      const imageData = ctx.getImageData(0, 0, width, height);
      const pngBytes = await recompressPNG(imageData);
      const resultBlob = new Blob([pngBytes as BlobPart], { type: 'image/png' });
      // 如果优化后比原图还大且没缩尺寸，保留原图
      if (resultBlob.size > file.size && width === img.naturalWidth && height === img.naturalHeight) {
        blob = file;
      } else {
        blob = resultBlob;
      }
    } catch {
      blob = await imageToBlob(img, { format: 'image/png', width, height });
    }
  } else if (format === 'image/png8') {
    try {
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext('2d');
      if (!ctx) throw new Error('无法获取 2D 上下文');
      ctx.drawImage(img, 0, 0, width, height);
      const imageData = ctx.getImageData(0, 0, width, height);
      const pngBytes = await quantizeAndEncodePNG(imageData, 256);
      const resultBlob = new Blob([pngBytes as BlobPart], { type: 'image/png' });
      // 量化后比原图大且没缩尺寸 → 保留原图
      if (resultBlob.size > file.size && width === img.naturalWidth && height === img.naturalHeight) {
        blob = file;
      } else {
        blob = resultBlob;
      }
    } catch {
      blob = await imageToBlob(img, { format: 'image/png', width, height });
    }
  } else {
    blob = await imageToBlob(img, {
      format,
      width,
      height,
      quality: opts.quality ?? 0.8,
    });
  }

  // 释放对象 URL
  if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src);
  return { blob, width, height };
}

/** 格式转换。 */
export async function convertImage(
  file: Blob,
  format: ImageFormat,
  quality = 0.9,
): Promise<Blob> {
  const img = await loadImage(file);
  const blob = await imageToBlob(img, { format, quality });
  if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src);
  return blob;
}

/** 缩放图片到指定宽高。 */
export async function resizeImage(
  file: Blob,
  width: number,
  height: number,
  format?: ImageFormat,
  quality = 0.9,
): Promise<Blob> {
  const img = await loadImage(file);
  const fmt: ImageFormat =
    format ?? (file.type === 'image/png' ? 'image/png' : 'image/jpeg');
  const blob = await imageToBlob(img, { format: fmt, width, height, quality });
  if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src);
  return blob;
}

/** 裁剪图片到指定区域。 */
export async function cropImage(
  file: Blob,
  rect: { x: number; y: number; width: number; height: number },
  format?: ImageFormat,
  quality = 0.9,
): Promise<Blob> {
  const img = await loadImage(file);
  const canvas = document.createElement('canvas');
  canvas.width = rect.width;
  canvas.height = rect.height;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('无法获取 2D 上下文');
  ctx.drawImage(
    img,
    rect.x,
    rect.y,
    rect.width,
    rect.height,
    0,
    0,
    rect.width,
    rect.height,
  );
  const fmt: ImageFormat =
    format ?? (file.type === 'image/png' ? 'image/png' : 'image/jpeg');
  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(
      (b) => (b ? resolve(b) : reject(new Error('toBlob 返回空'))),
      fmt,
      quality,
    );
  });
  if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src);
  return blob;
}
