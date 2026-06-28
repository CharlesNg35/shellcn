const MIME = "video/webm";

export const WEBM_REPLACEMENT_CHUNK_BYTES = 6 << 20;

export async function fixWebmDurationMetadata(
  chunks: Blob[],
  durationMs: number,
): Promise<Blob | null> {
  if (!chunks.length || durationMs <= 0) return null;
  const original = new Blob(chunks, { type: MIME });
  const { fixWebmDuration } = await import("@fix-webm-duration/fix");
  const fixed = await fixWebmDuration(original, durationMs, { logger: false });
  if (!fixed || fixed.size === 0) return null;
  if (fixed === original && fixed.size === original.size) return null;
  return fixed;
}

export function webmReplacementChunks(blob: Blob): Blob[] {
  const chunks: Blob[] = [];
  for (
    let offset = 0;
    offset < blob.size;
    offset += WEBM_REPLACEMENT_CHUNK_BYTES
  ) {
    chunks.push(
      blob.slice(offset, offset + WEBM_REPLACEMENT_CHUNK_BYTES, MIME),
    );
  }
  return chunks;
}
