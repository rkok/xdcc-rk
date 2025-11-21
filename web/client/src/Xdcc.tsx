import { useState, FormEvent } from 'react';
import { useSortableData } from './utils/useSortableData';
import { SortIcon } from './utils/SortIcon';
import './styles/table-sort.css';

type XdccProps = {}

type SearchResult = {
  fileName: string;
  size: number;
  url: string;
  server?: string; // Computed from URL
}

type SearchResponse = {
  results: SearchResult[];
}

type DownloadError = {
  message: string;
}

type DownloadState = {
  status: string;
  errors: DownloadError[];
}

const formatFileSize = (sizeInKB: number): string => {
  const units = ['KB', 'MB', 'GB', 'TB'];
  let size = sizeInKB;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  return `${size.toFixed(2)} ${units[unitIndex]}`;
}

const extractHostname = (ircUrl: string): string => {
  try {
    // IRC URL format: irc://hostname/channel/bot/slot
    const url = new URL(ircUrl);
    return url.hostname;
  } catch (err) {
    // If URL parsing fails, try to extract manually
    const match = ircUrl.match(/^irc:\/\/([^\/]+)/);
    return match ? match[1] : '';
  }
}

const Xdcc = ({}: XdccProps) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<SearchResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [downloads, setDownloads] = useState<Record<string, DownloadState>>({});
  const [eventSources, setEventSources] = useState<Record<string, EventSource>>({});

  // Sorting for search results
  const { items: sortedResults, requestSort, sortConfig } = useSortableData<SearchResult>(
    searchResults?.results || []
  );

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/search?q=${encodeURIComponent(searchQuery)}`);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      // Add computed server property to each result
      if (data.results) {
        data.results = data.results.map((result: SearchResult) => ({
          ...result,
          server: extractHostname(result.url)
        }));
      }
      setSearchResults(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
      setSearchResults(null);
    } finally {
      setIsLoading(false);
    }
  };

  const handleDownload = (url: string) => {
    // Initialize download state
    setDownloads(prev => ({
      ...prev,
      [url]: { status: 'Starting...', errors: [] }
    }));

    const eventSource = new EventSource(`/api/download?url=${encodeURIComponent(url)}`);

    // Store the EventSource reference so we can cancel it later
    setEventSources(prev => ({
      ...prev,
      [url]: eventSource
    }));

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log('Download event:', data);

        setDownloads(prev => {
          const currentDownload = prev[url] || { status: '', errors: [] };
          let newStatus = currentDownload.status;
          const newErrors = [...currentDownload.errors];

          switch (data.type) {
            case 'connecting':
              newStatus = 'Connecting...';
              break;
            case 'connected':
              newStatus = 'Connected';
              break;
            case 'started':
              newStatus = 'Started download';
              break;
            case 'progress':
              const transferred = formatFileSize(data.bytesTransferred / 1024);
              const total = formatFileSize(data.totalBytes / 1024);
              const percentage = data.percentage.toFixed(1);
              const rate = formatFileSize(data.transferRate / 1024);
              newStatus = `${transferred}/${total} (${percentage}%) - ${rate}/s`;
              break;
            case 'completed':
              newStatus = 'Done';
              eventSource.close();
              setEventSources(prev => {
                const newSources = { ...prev };
                delete newSources[url];
                return newSources;
              });
              break;
            case 'error':
              newErrors.push({ message: data.error });
              if (data.fatal) {
                eventSource.close();
                setEventSources(prev => {
                  const newSources = { ...prev };
                  delete newSources[url];
                  return newSources;
                });
              }
              break;
            case 'aborted':
              newStatus = `Aborted: ${data.reason}`;
              eventSource.close();
              setEventSources(prev => {
                const newSources = { ...prev };
                delete newSources[url];
                return newSources;
              });
              break;
          }

          return {
            ...prev,
            [url]: { status: newStatus, errors: newErrors }
          };
        });
      } catch (err) {
        console.error('Failed to parse event data:', err);
      }
    };

    eventSource.onerror = (err) => {
      console.error('EventSource error:', err);
      eventSource.close();
      setEventSources(prev => {
        const newSources = { ...prev };
        delete newSources[url];
        return newSources;
      });
      setDownloads(prev => ({
        ...prev,
        [url]: {
          ...prev[url],
          status: 'Connection error'
        }
      }));
    };
  };

  const handleCancelDownload = (url: string) => {
    const eventSource = eventSources[url];
    if (eventSource) {
      eventSource.close();
      setEventSources(prev => {
        const newSources = { ...prev };
        delete newSources[url];
        return newSources;
      });
      setDownloads(prev => ({
        ...prev,
        [url]: {
          ...prev[url],
          status: 'Cancelled'
        }
      }));
    }
  };

  return (
    <>
      <h1>XDCC Search</h1>
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Enter search query"
        />
        <button type="submit" disabled={isLoading}>
          {isLoading ? 'Searching...' : 'Search'}
        </button>
      </form>

      {error && <div>Error: {error}</div>}

      {searchResults && searchResults.results && searchResults.results.length > 0 && (
        <div>
          <h2>Results:</h2>
          <table>
            <thead>
              <tr>
                <th
                  className="sortable"
                  onClick={() => requestSort('fileName')}
                >
                  File Name
                  <SortIcon direction={sortConfig?.key === 'fileName' ? sortConfig.direction : null} />
                </th>
                <th
                  className="sortable"
                  onClick={() => requestSort('size')}
                >
                  Size
                  <SortIcon direction={sortConfig?.key === 'size' ? sortConfig.direction : null} />
                </th>
                <th
                  className="sortable"
                  onClick={() => requestSort('server')}
                >
                  Server
                  <SortIcon direction={sortConfig?.key === 'server' ? sortConfig.direction : null} />
                </th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {sortedResults.map((result, index) => {
                const downloadState = downloads[result.url];
                const isDownloading = eventSources[result.url] !== undefined;
                return (
                  <tr key={index}>
                    <td title={result.url}>{result.fileName}</td>
                    <td title={result.url}>{formatFileSize(result.size)}</td>
                    <td title={result.url}>{result.server}</td>
                    <td>
                      {downloadState ? (
                        <div>
                          {isDownloading && (
                            <button onClick={() => handleCancelDownload(result.url)}>Cancel</button>
                          )}
                          {downloadState.errors.map((error, errorIndex) => (
                            <span key={errorIndex} title={error.message}>⚠️ </span>
                          ))}
                          {downloadState.status}
                        </div>
                      ) : (
                        <button onClick={() => handleDownload(result.url)}>Download</button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {searchResults && searchResults.results && searchResults.results.length === 0 && (
        <div>No results found.</div>
      )}
    </>
  );
}

export default Xdcc;
