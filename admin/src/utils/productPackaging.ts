export type CollectedPackagingRow = {
  specification: string;
  lengthCm: number | null;
  widthCm: number | null;
  heightCm: number | null;
  volumeCm3: number | null;
  weightG: number | null;
};

function measurement(value: unknown): number | null | undefined {
  if (value === null) return null;
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : undefined;
}

/** Read the optional browser-extension packaging block from persisted FullNormalizedJSON. */
export function collectedPackagingRowsFromRaw(rawData: unknown): CollectedPackagingRow[] {
  if (!rawData || typeof rawData !== 'object' || Array.isArray(rawData)) return [];
  const packaging = (rawData as Record<string, unknown>).packaging;
  if (!packaging || typeof packaging !== 'object' || Array.isArray(packaging)) return [];
  const sourceRows = (packaging as Record<string, unknown>).rows;
  if (!Array.isArray(sourceRows)) return [];

  const rows: CollectedPackagingRow[] = [];
  for (const sourceRow of sourceRows.slice(0, 200)) {
    if (!sourceRow || typeof sourceRow !== 'object' || Array.isArray(sourceRow)) continue;
    const raw = sourceRow as Record<string, unknown>;
    const specification = typeof raw.specification === 'string' ? raw.specification.trim() : '';
    const lengthCm = measurement(raw.lengthCm);
    const widthCm = measurement(raw.widthCm);
    const heightCm = measurement(raw.heightCm);
    const volumeCm3 = measurement(raw.volumeCm3);
    const weightG = measurement(raw.weightG);
    if (!specification || specification.length > 200 || [lengthCm, widthCm, heightCm, volumeCm3, weightG].some((value) => value === undefined)) {
      continue;
    }
    rows.push({
      specification,
      lengthCm: lengthCm!,
      widthCm: widthCm!,
      heightCm: heightCm!,
      volumeCm3: volumeCm3!,
      weightG: weightG!,
    });
  }
  return rows;
}
