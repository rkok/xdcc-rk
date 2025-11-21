import { useState, useMemo } from 'react';

export type SortDirection = 'ascending' | 'descending' | null;

export interface SortConfig<T> {
  key: keyof T;
  direction: SortDirection;
}

export interface UseSortableDataResult<T> {
  items: T[];
  requestSort: (key: keyof T) => void;
  sortConfig: SortConfig<T> | null;
}

/**
 * Custom hook for sorting table data
 * @param items - Array of items to sort
 * @param config - Optional initial sort configuration
 * @returns Sorted items, sort request function, and current sort config
 */
export function useSortableData<T>(
  items: T[],
  config: SortConfig<T> | null = null
): UseSortableDataResult<T> {
  const [sortConfig, setSortConfig] = useState<SortConfig<T> | null>(config);

  const sortedItems = useMemo(() => {
    const sortableItems = [...items];
    
    if (sortConfig !== null) {
      sortableItems.sort((a, b) => {
        const aValue = a[sortConfig.key];
        const bValue = b[sortConfig.key];

        // Handle null/undefined values
        if (aValue == null && bValue == null) return 0;
        if (aValue == null) return 1;
        if (bValue == null) return -1;

        // Handle numeric values
        if (typeof aValue === 'number' && typeof bValue === 'number') {
          return sortConfig.direction === 'ascending' 
            ? aValue - bValue 
            : bValue - aValue;
        }

        // Handle string values with locale-aware comparison
        const aString = String(aValue);
        const bString = String(bValue);
        
        const comparison = aString.localeCompare(bString, 'en', {
          numeric: true,
          sensitivity: 'base'
        });

        return sortConfig.direction === 'ascending' ? comparison : -comparison;
      });
    }
    
    return sortableItems;
  }, [items, sortConfig]);

  const requestSort = (key: keyof T) => {
    let direction: SortDirection = 'ascending';
    
    if (
      sortConfig &&
      sortConfig.key === key &&
      sortConfig.direction === 'ascending'
    ) {
      direction = 'descending';
    }
    
    setSortConfig({ key, direction });
  };

  return { items: sortedItems, requestSort, sortConfig };
}

