import { SortDirection } from './useSortableData';

interface SortIconProps {
  direction: SortDirection;
}

/**
 * Component to display sort direction indicator
 * @param direction - Current sort direction (ascending, descending, or null)
 */
export const SortIcon = ({ direction }: SortIconProps) => {
  if (direction === 'ascending') {
    return <span className="sort-icon">▲</span>;
  }
  if (direction === 'descending') {
    return <span className="sort-icon">▼</span>;
  }
  return <span className="sort-icon sort-icon-neutral">⬍</span>;
};

