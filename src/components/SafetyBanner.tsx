import React from 'react';
import { css } from '@emotion/css';
import { Alert, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import { RowEstimate, formatBytes, formatNumber } from '../hooks/useRowEstimate';

interface SafetyBannerProps {
  estimate: RowEstimate | null;
  optimizeDisplay: boolean;
  maxRawRows: number;
}

export function SafetyBanner({ estimate, optimizeDisplay, maxRawRows }: SafetyBannerProps) {
  const styles = useStyles2(getStyles);

  if (!estimate) {
    return null;
  }

  if (optimizeDisplay) {
    return (
      <div className={styles.container}>
        <div className={styles.infoLine}>
          <span>Max {formatNumber(maxRawRows)} pts</span>
          <span className={styles.safeTag}>Optimized</span>
        </div>
      </div>
    );
  }

  // Raw mode
  switch (estimate.level) {
    case 'safe':
      return (
        <div className={styles.container}>
          <div className={styles.infoLine}>
            <span>{formatNumber(estimate.estimatedRows)} rows</span>
            <span className={styles.separator}>&middot;</span>
            <span>{formatBytes(estimate.estimatedSizeBytes)}</span>
            <span className={styles.safeTag}>OK</span>
          </div>
        </div>
      );

    case 'warning':
      return (
        <Alert title="Large raw query" severity="warning" className={styles.alert}>
          <div className={styles.alertBody}>
            <div>
              ~{formatNumber(estimate.estimatedRows)} rows, ~{formatBytes(estimate.estimatedSizeBytes)},
              ~{estimate.estimatedTimeMs}ms
            </div>
            <div className={styles.hint}>
              LIMIT {formatNumber(maxRawRows)} will be applied. Switch to Optimize Display for better
              performance (~{formatNumber(estimate.optimizedRows)} pts).
            </div>
          </div>
        </Alert>
      );

    case 'danger':
      return (
        <Alert title="Very large raw query" severity="error" className={styles.alert}>
          <div className={styles.alertBody}>
            <div>
              ~{formatNumber(estimate.estimatedRows)} rows, ~{formatBytes(estimate.estimatedSizeBytes)} —
              this will overwhelm your browser.
            </div>
            <div className={styles.hint}>
              Reduce the time range or switch to Optimize Display
              (~{formatNumber(estimate.optimizedRows)} pts).
            </div>
            <div className={styles.limitNotice}>
              Max rows: {formatNumber(maxRawRows)} applied automatically.
            </div>
          </div>
        </Alert>
      );

    case 'blocked':
      return (
        <Alert title="Query blocked — exceeds hard limit" severity="error" className={styles.alert}>
          <div className={styles.alertBody}>
            <div>
              ~{formatNumber(estimate.estimatedRows)} rows exceeds the absolute limit.
              This query cannot be executed in raw mode.
            </div>
            <div className={styles.hint}>
              Use Optimize Display (~{formatNumber(estimate.optimizedRows)} pts,
              ~{formatBytes(estimate.optimizedSizeBytes)}).
            </div>
          </div>
        </Alert>
      );
  }
}

function getStyles(theme: GrafanaTheme2) {
  return {
    container: css({
      padding: `${theme.spacing(0.5)} ${theme.spacing(1)}`,
    }),
    infoLine: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.secondary,
    }),
    icon: css({
      fontFamily: 'monospace',
    }),
    separator: css({
      color: theme.colors.text.disabled,
    }),
    safeTag: css({
      backgroundColor: theme.colors.success.transparent,
      color: theme.colors.success.text,
      padding: `0 ${theme.spacing(0.5)}`,
      borderRadius: theme.shape.radius.default,
      fontSize: theme.typography.bodySmall.fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      marginLeft: theme.spacing(0.5),
    }),
    alert: css({
      marginBottom: 0,
    }),
    alertBody: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.5),
      fontSize: theme.typography.bodySmall.fontSize,
    }),
    hint: css({
      color: theme.colors.text.secondary,
    }),
    limitNotice: css({
      fontWeight: theme.typography.fontWeightMedium,
    }),
  };
}
