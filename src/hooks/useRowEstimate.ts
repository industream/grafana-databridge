import { useMemo } from 'react';
import { TimeRange } from '@grafana/data';

export type SafetyLevel = 'safe' | 'warning' | 'danger' | 'blocked';

export interface RowEstimate {
  estimatedRows: number;
  estimatedSizeBytes: number;
  estimatedTimeMs: number;
  level: SafetyLevel;
  timeWindowLabel: string;
  optimizedRows: number;
  optimizedSizeBytes: number;
}

interface RowEstimateParams {
  timeRange: TimeRange | undefined;
  columnCount: number;
  optimizeDisplay: boolean;
  maxDataPoints: number;
  maxRawRows: number;
  hardLimitRows: number;
}

const BYTES_PER_CELL = 16;
const MS_PER_10K_ROWS = 50;
const DEFAULT_INTERVAL_SECONDS = 1;

export function useRowEstimate({
  timeRange,
  columnCount,
  optimizeDisplay,
  maxDataPoints,
  maxRawRows,
  hardLimitRows,
}: RowEstimateParams): RowEstimate | null {
  return useMemo(() => {
    if (!timeRange || columnCount === 0) {
      return null;
    }

    const rangeMs = timeRange.to.diff(timeRange.from, 'milliseconds');
    const rangeSeconds = rangeMs / 1000;

    // Raw row estimate: one row per second per column
    const rawRowsPerSeries = Math.ceil(rangeSeconds / DEFAULT_INTERVAL_SECONDS);
    const estimatedRows = rawRowsPerSeries * columnCount;

    // Optimized estimate
    const windowSeconds = computeNiceWindow(rangeSeconds, maxDataPoints || 1000);
    const optimizedRowsPerSeries = Math.ceil(rangeSeconds / windowSeconds);
    const optimizedRows = optimizedRowsPerSeries * columnCount;

    // Size estimates
    const estimatedSizeBytes = estimatedRows * BYTES_PER_CELL;
    const optimizedSizeBytes = optimizedRows * BYTES_PER_CELL;

    // Time estimate
    const estimatedTimeMs = Math.ceil((estimatedRows / 10000) * MS_PER_10K_ROWS);

    // Safety level (only applies to raw mode)
    let level: SafetyLevel = 'safe';
    if (!optimizeDisplay) {
      if (estimatedRows > hardLimitRows) {
        level = 'blocked';
      } else if (estimatedRows > maxRawRows * 2) {
        level = 'danger';
      } else if (estimatedRows > maxRawRows) {
        level = 'warning';
      }
    }

    const timeWindowLabel = formatTimeWindow(windowSeconds);

    return {
      estimatedRows,
      estimatedSizeBytes,
      estimatedTimeMs,
      level,
      timeWindowLabel,
      optimizedRows,
      optimizedSizeBytes,
    };
  }, [timeRange, columnCount, optimizeDisplay, maxDataPoints, maxRawRows, hardLimitRows]);
}

function computeNiceWindow(rangeSeconds: number, maxDataPoints: number): number {
  const raw = Math.ceil(rangeSeconds / maxDataPoints);
  if (raw <= 1) { return 1; }
  if (raw <= 5) { return 5; }
  if (raw <= 10) { return 10; }
  if (raw <= 30) { return 30; }
  if (raw <= 60) { return 60; }
  if (raw <= 300) { return 300; }
  if (raw <= 600) { return 600; }
  if (raw <= 1800) { return 1800; }
  if (raw <= 3600) { return 3600; }
  if (raw <= 21600) { return 21600; }
  if (raw <= 43200) { return 43200; }
  if (raw <= 86400) { return 86400; }
  return raw;
}

export function formatTimeWindow(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}s`;
  }
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)}min`;
  }
  if (seconds < 86400) {
    const hours = Math.floor(seconds / 3600);
    return `${hours}h`;
  }
  const days = Math.floor(seconds / 86400);
  return `${days}d`;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function formatNumber(n: number): string {
  if (n >= 1_000_000) {
    return `${(n / 1_000_000).toFixed(1)}M`;
  }
  if (n >= 1_000) {
    return `${(n / 1_000).toFixed(1)}K`;
  }
  return String(n);
}
