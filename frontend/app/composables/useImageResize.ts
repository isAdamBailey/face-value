/**
 * resizeImageFile downscales file client-side so its longest edge is at
 * most maxEdge pixels, to save upload bandwidth. The server re-resizes to
 * 1024px for the model regardless — this is belt and braces, not the
 * authoritative resize.
 */
export async function resizeImageFile(file: File, maxEdge = 2048): Promise<File> {
  if (!file.type.startsWith('image/')) {
    return file
  }

  let bitmap: ImageBitmap
  try {
    bitmap = await createImageBitmap(file)
  } catch {
    return file
  }

  const { width, height } = bitmap
  const scale = Math.min(1, maxEdge / Math.max(width, height))
  if (scale >= 1) {
    bitmap.close()
    return file
  }

  const targetWidth = Math.round(width * scale)
  const targetHeight = Math.round(height * scale)

  const canvas = document.createElement('canvas')
  canvas.width = targetWidth
  canvas.height = targetHeight
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    bitmap.close()
    return file
  }
  ctx.drawImage(bitmap, 0, 0, targetWidth, targetHeight)
  bitmap.close()

  const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.9))
  if (!blob) {
    return file
  }

  return new File([blob], file.name.replace(/\.\w+$/, '.jpg'), { type: 'image/jpeg' })
}
