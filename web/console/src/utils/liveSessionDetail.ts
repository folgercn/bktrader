import type { LiveSession } from '../types/domain';

const DETAIL_FIELDS_META_KEY = "__detailFields";

function detailFieldSet(session: LiveSession | undefined): Set<string> {
  const fields = session?.metadata?.[DETAIL_FIELDS_META_KEY];
  return new Set(Array.isArray(fields) ? fields.map((item) => String(item)) : []);
}

export function hasLiveSessionDetailFields(session: LiveSession | null | undefined, fields: string[]) {
  const loaded = detailFieldSet(session ?? undefined);
  return fields.every((field) => loaded.has(field));
}

function withDetailFields(session: LiveSession, fields: string[]): LiveSession {
  const merged = detailFieldSet(session);
  for (const field of fields) {
    if (field) {
      merged.add(field);
    }
  }
  return {
    ...session,
    metadata: {
      ...(session.metadata ?? {}),
      [DETAIL_FIELDS_META_KEY]: Array.from(merged).sort(),
    },
  };
}

function mergeStateList(existing: unknown, snapshot: unknown): unknown {
  if (!Array.isArray(existing) || !Array.isArray(snapshot)) {
    return snapshot ?? existing;
  }
  const byKey = new Map<string, unknown>();
  for (const item of existing) {
    byKey.set(JSON.stringify(item), item);
  }
  for (const item of snapshot) {
    byKey.set(JSON.stringify(item), item);
  }
  return Array.from(byKey.values());
}

function mergeDetailField(field: string, existingValue: unknown, snapshotValue: unknown): unknown {
  if (field === "timeline" || field === "breakoutHistory") {
    return mergeStateList(existingValue, snapshotValue);
  }
  return snapshotValue === undefined ? existingValue : snapshotValue;
}

export function mergeLiveSessionSnapshot(current: LiveSession[], snapshot: LiveSession[]): LiveSession[] {
  const currentById = new Map(current.map((item) => [item.id, item] as const));
  return snapshot.map((item) => {
    const existing = currentById.get(item.id);
    if (!existing) {
      return item;
    }
    const loadedFields = detailFieldSet(existing);
    if (loadedFields.size === 0) {
      return item;
    }
    const state = { ...(item.state ?? {}) };
    const existingState = existing.state ?? {};
    for (const field of loadedFields) {
      if (field in existingState) {
        state[field] = mergeDetailField(field, existingState[field], state[field]);
      }
    }
    return withDetailFields(
      {
        ...item,
        state,
        metadata: {
          ...(item.metadata ?? {}),
          ...(existing.metadata ?? {}),
        },
      },
      Array.from(loadedFields)
    );
  });
}

export function mergeLiveSessionDetail(
  current: LiveSession[],
  detail: LiveSession,
  fields: string[]
): LiveSession[] {
  let found = false;
  const next = current.map((item) => {
    if (item.id !== detail.id) {
      return item;
    }
    found = true;
    return withDetailFields(
      {
        ...item,
        ...detail,
        state: {
          ...(item.state ?? {}),
          ...(detail.state ?? {}),
        },
        metadata: {
          ...(item.metadata ?? {}),
          ...(detail.metadata ?? {}),
        },
      },
      fields
    );
  });
  return found ? next : [withDetailFields(detail, fields), ...next];
}
