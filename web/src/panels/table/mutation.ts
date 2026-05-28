export interface RowMutation {
  key?: Record<string, unknown>;
  values?: Record<string, unknown>;
}

export function insertMutation(values: Record<string, unknown>): RowMutation {
  return { values };
}

export function updateMutation(
  key: Record<string, unknown>,
  values: Record<string, unknown>,
): RowMutation {
  return { key, values };
}

export function deleteMutation(key: Record<string, unknown>): RowMutation {
  return { key };
}
