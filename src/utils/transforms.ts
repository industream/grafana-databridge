import { Transform, TransformKind, TRANSFORM_ORDER } from '../types';

/** Returns the params object of the transform of the given kind, or undefined. */
export function getTransformParams(
  transforms: Transform[] | undefined,
  kind: TransformKind
): Record<string, unknown> | undefined {
  const found = transforms?.find((t) => t[kind] !== undefined);
  return found?.[kind] as Record<string, unknown> | undefined;
}

/**
 * Sets (or removes, when params is undefined) a transform of the given kind and
 * returns a new array in fixed pipeline order (TRANSFORM_ORDER). The result uses
 * the wrapper-object contract shape expected by DataBridge /records/query.
 */
export function setTransformInPipeline(
  transforms: Transform[] | undefined,
  kind: TransformKind,
  params: object | undefined
): Transform[] {
  const byKind = new Map<TransformKind, Transform>();
  for (const t of transforms ?? []) {
    for (const k of TRANSFORM_ORDER) {
      if (t[k] !== undefined) {
        byKind.set(k, t);
      }
    }
  }

  if (params === undefined) {
    byKind.delete(kind);
  } else {
    byKind.set(kind, { [kind]: params } as Transform);
  }

  return TRANSFORM_ORDER.filter((k) => byKind.has(k)).map((k) => byKind.get(k)!);
}
