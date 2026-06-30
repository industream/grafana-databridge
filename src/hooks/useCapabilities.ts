import { useEffect, useState } from 'react';

import { DataSource } from '../datasource';
import { ProviderCapabilities } from '../types';

/**
 * Fetch the active DataBridge provider's capabilities once per
 * (datasource, connection) and cache them in component state.
 *
 * Returns `null` while loading, when capabilities are unknown (older image or
 * unreachable DataBridge), or on error — every consumer treats `null` as
 * "offer everything" (degrade-open). The fetch never throws into the editor.
 */
export function useCapabilities(
  datasource: DataSource,
  connectionId?: string
): ProviderCapabilities | null {
  const [capabilities, setCapabilities] = useState<ProviderCapabilities | null>(null);

  useEffect(() => {
    let cancelled = false;
    datasource
      .getCapabilities(connectionId)
      .then((caps) => {
        if (!cancelled) {
          setCapabilities(caps);
        }
      })
      .catch(() => {
        // Degrade open: keep capabilities null so all aggregations stay enabled.
        if (!cancelled) {
          setCapabilities(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource, connectionId]);

  return capabilities;
}
