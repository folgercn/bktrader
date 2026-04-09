/**
 * Helper to resolve Zustand-style updaters (either a value or a function that produces a value from the previous state).
 */
export function resolveUpdater<T>(valOrUpdater: T | ((prev: T) => T), prev: T): T {
  return typeof valOrUpdater === 'function'
    ? (valOrUpdater as (prev: T) => T)(prev)
    : valOrUpdater;
}
