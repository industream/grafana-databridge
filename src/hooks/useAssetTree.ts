import { useEffect, useMemo, useState } from 'react';

import { DataSource } from '../datasource';
import { AssetNode, AssetTree, CatalogEntry, Label } from '../types';

export interface FlatTreeNode {
  id: string;
  name: string;
  parentId: string | null;
  depth: number;
  entryCount: number;
  isExpanded: boolean;
  isLoading: boolean;
  entries: CatalogEntry[];
  children: FlatTreeNode[];
  hasChildren: boolean;
}

interface UseAssetTreeResult {
  trees: AssetTree[];
  labels: Label[];
  loading: boolean;
  error: string | null;
  flatNodes: FlatTreeNode[];
  expandedNodeIds: Set<string>;
  toggleNode: (nodeId: string) => void;
  expandAll: () => void;
  collapseAll: () => void;
  searchQuery: string;
  setSearchQuery: (query: string) => void;
  labelFilter: string | null;
  setLabelFilter: (label: string | null) => void;
  filteredEntries: CatalogEntry[];
  selectedTreeId: string | null;
  setSelectedTreeId: (id: string | null) => void;
  refresh: () => void;
  assetPaths: Record<string, string>;
}

export function useAssetTree(datasource: DataSource): UseAssetTreeResult {
  const [trees, setTrees] = useState<AssetTree[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [allEntries, setAllEntries] = useState<CatalogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<string>>(new Set());
  const [nodeEntries, setNodeEntries] = useState<Record<string, CatalogEntry[]>>({});
  const [loadingNodeIds, setLoadingNodeIds] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState('');
  const [labelFilter, setLabelFilter] = useState<string | null>(null);
  const [selectedTreeId, setSelectedTreeId] = useState<string | null>(null);
  const [refreshCounter, setRefreshCounter] = useState(0);

  // Load trees, labels, and all entries on mount (and on manual refresh)
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    const load = refreshCounter > 0
      ? datasource.clearCache().then(() => Promise.allSettled([
          datasource.getAssetTree(),
          datasource.getLabels(),
          datasource.getCatalogEntries({}),
        ]))
      : Promise.allSettled([
          datasource.getAssetTree(),
          datasource.getLabels(),
          datasource.getCatalogEntries({}),
        ]);

    load
      .then(([treesResult, labelsResult, entriesResult]) => {
        if (cancelled) {
          return;
        }

        // Check if any request failed
        const failures = [treesResult, labelsResult, entriesResult].filter(
          (r): r is PromiseRejectedResult => r.status === 'rejected'
        );

        if (failures.length > 0) {
          const reason = failures[0].reason;
          const detail = reason?.data?.error ?? reason?.data?.message ?? reason?.message ?? '';
          const isUnreachable = detail.includes('dial tcp') || detail.includes('connection refused')
            || detail.includes('no such host') || reason?.status === 502 || reason?.status === 503;
          setError(
            isUnreachable
              ? 'DataCatalog is unreachable. Check that the DataCatalog API is running.'
              : detail || 'Failed to load asset tree'
          );
          setLoading(false);
          return;
        }

        setTrees((treesResult as PromiseFulfilledResult<AssetTree[]>).value ?? []);
        setLabels((labelsResult as PromiseFulfilledResult<Label[]>).value ?? []);
        setAllEntries((entriesResult as PromiseFulfilledResult<CatalogEntry[]>).value ?? []);
        setLoading(false);
      });

    return () => { cancelled = true; };
  }, [datasource, refreshCounter]);

  // Filter entries by search and label
  const filteredEntries = useMemo(() => {
    let entries = allEntries;

    if (labelFilter) {
      entries = entries.filter((e) => e.labels.some((l) => l.name === labelFilter));
    }

    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase().trim();
      entries = entries.filter((e) => {
        const name = e.name.toLowerCase();
        const tag = e.metadata?.tagLevel1?.toLowerCase() ?? '';
        const descEn = e.metadata?.description?.['en-US']?.toLowerCase() ?? '';
        const descDe = e.metadata?.description?.['de-DE']?.toLowerCase() ?? '';
        return name.includes(query) || tag.includes(query) || descEn.includes(query) || descDe.includes(query);
      });
    }

    return entries;
  }, [allEntries, searchQuery, labelFilter]);

  // Build flat tree for rendering, filtered by selected dictionary
  const flatNodes = useMemo(() => {
    const result: FlatTreeNode[] = [];
    const visibleTrees = selectedTreeId
      ? trees.filter((t) => t.id === selectedTreeId)
      : trees;

    const flatten = (nodes: AssetNode[], depth: number) => {
      for (const node of nodes) {
        const hasChildren = (node.children && node.children.length > 0) || node.entryCount > 0;
        const flatNode: FlatTreeNode = {
          id: node.id,
          name: node.name,
          parentId: node.parentId,
          depth,
          entryCount: node.entryCount,
          isExpanded: expandedNodeIds.has(node.id),
          isLoading: loadingNodeIds.has(node.id),
          entries: nodeEntries[node.id] ?? [],
          children: [],
          hasChildren,
        };
        result.push(flatNode);

        if (expandedNodeIds.has(node.id)) {
          if (node.children && node.children.length > 0) {
            flatten(node.children, depth + 1);
          }
        }
      }
    };

    for (const tree of visibleTrees) {
      flatten(tree.nodes, 0);
    }

    return result;
  }, [trees, selectedTreeId, expandedNodeIds, nodeEntries, loadingNodeIds]);

  const toggleNode = (nodeId: string) => {
    setExpandedNodeIds((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });

    // Load entries for nodes that don't have children (leaf nodes)
    if (!expandedNodeIds.has(nodeId) && !nodeEntries[nodeId]) {
      setLoadingNodeIds((prev) => new Set([...prev, nodeId]));
      datasource.getNodeEntries(nodeId).then((entries) => {
        setNodeEntries((prev) => ({ ...prev, [nodeId]: entries ?? [] }));
        setLoadingNodeIds((prev) => {
          const next = new Set(prev);
          next.delete(nodeId);
          return next;
        });
      }).catch((err) => {
        console.error('Failed to load node entries', err);
        setLoadingNodeIds((prev) => {
          const next = new Set(prev);
          next.delete(nodeId);
          return next;
        });
      });
    }
  };

  const collectAllNodeIds = (nodes: AssetNode[]): string[] => {
    const ids: string[] = [];
    for (const node of nodes) {
      ids.push(node.id);
      if (node.children) {
        ids.push(...collectAllNodeIds(node.children));
      }
    }
    return ids;
  };

  const expandAll = () => {
    const allIds = trees.flatMap((t) => collectAllNodeIds(t.nodes));
    setExpandedNodeIds(new Set(allIds));
  };

  const collapseAll = () => {
    setExpandedNodeIds(new Set());
  };

  const refresh = () => {
    setNodeEntries({});
    setRefreshCounter((c) => c + 1);
  };

  // Build entryId -> asset path map from loaded trees
  const assetPaths = useMemo(() => {
    const paths: Record<string, string> = {};
    const walk = (nodes: AssetNode[], ancestors: string[]) => {
      for (const node of nodes) {
        const currentPath = [...ancestors, node.name];
        const pathStr = currentPath.join(' > ');
        for (const entryId of node.entryIds ?? []) {
          paths[entryId] = pathStr;
        }
        if (node.children) {
          walk(node.children, currentPath);
        }
      }
    };
    for (const tree of trees) {
      walk(tree.nodes, []);
    }
    return paths;
  }, [trees]);

  return {
    trees,
    labels,
    loading,
    error,
    flatNodes,
    expandedNodeIds,
    toggleNode,
    expandAll,
    collapseAll,
    searchQuery,
    setSearchQuery,
    labelFilter,
    setLabelFilter,
    filteredEntries,
    selectedTreeId,
    setSelectedTreeId,
    refresh,
    assetPaths,
  };
}
