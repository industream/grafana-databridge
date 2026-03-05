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
  entries: CatalogEntry[];
  children: FlatTreeNode[];
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
}

export function useAssetTree(datasource: DataSource): UseAssetTreeResult {
  const [trees, setTrees] = useState<AssetTree[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [allEntries, setAllEntries] = useState<CatalogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState('');
  const [labelFilter, setLabelFilter] = useState<string | null>(null);

  // Load trees, labels, and all entries on mount
  useEffect(() => {
    let cancelled = false;

    Promise.all([
      datasource.getAssetTree(),
      datasource.getLabels(),
      datasource.getCatalogEntries({}),
    ])
      .then(([treesData, labelsData, entriesData]) => {
        if (!cancelled) {
          setTrees(treesData ?? []);
          setLabels(labelsData ?? []);
          setAllEntries(entriesData ?? []);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err.message ?? 'Failed to load asset tree');
          setLoading(false);
        }
      });

    return () => { cancelled = true; };
  }, [datasource]);

  // Filter entries by search and label
  const filteredEntries = useMemo(() => {
    let entries = allEntries;

    if (labelFilter) {
      entries = entries.filter((e) => e.labels.includes(labelFilter));
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

  // Build flat tree for rendering
  const flatNodes = useMemo(() => {
    const result: FlatTreeNode[] = [];

    const flatten = (nodes: AssetNode[], depth: number) => {
      for (const node of nodes) {
        const flatNode: FlatTreeNode = {
          id: node.id,
          name: node.name,
          parentId: node.parentId,
          depth,
          entryCount: node.entryCount,
          isExpanded: expandedNodeIds.has(node.id),
          entries: [],
          children: [],
        };
        result.push(flatNode);

        if (node.children && node.children.length > 0 && expandedNodeIds.has(node.id)) {
          flatten(node.children, depth + 1);
        }
      }
    };

    for (const tree of trees) {
      flatten(tree.nodes, 0);
    }

    return result;
  }, [trees, expandedNodeIds]);

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
  };
}
