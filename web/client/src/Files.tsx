import { useEffect, useState } from 'react';

interface FileInfo {
  name: string;
  size: number;
}

interface DiskSpace {
  available: number;
  total: number;
}

interface ListFilesResponse {
  files: FileInfo[];
  diskSpace: DiskSpace;
}

type FilesProps = {}

const Files = ({}: FilesProps) => {
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [diskSpace, setDiskSpace] = useState<DiskSpace | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchFiles = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/files/list');
      if (!response.ok) {
        throw new Error(`Failed to fetch files: ${response.statusText}`);
      }
      const data: ListFilesResponse = await response.json();
      setFiles(data.files);
      setDiskSpace(data.diskSpace);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load files');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchFiles();
  }, []);

  const handleDownload = (filename: string) => {
    // Create a temporary anchor element to trigger download
    const url = `/api/files/download?file=${encodeURIComponent(filename)}`;
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  const handleDelete = async (filename: string) => {
    if (!confirm(`Are you sure you want to delete "${filename}"?`)) {
      return;
    }

    try {
      const response = await fetch('/api/files/delete', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ filename }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to delete file');
      }

      // Refresh the file list after successful deletion
      await fetchFiles();
    } catch (err) {
      alert(`Error deleting file: ${err instanceof Error ? err.message : 'Unknown error'}`);
    }
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
  };

  if (loading) {
    return <div>Loading files...</div>;
  }

  if (error) {
    return (
      <div>
        <p>Error: {error}</p>
        <button onClick={fetchFiles}>Retry</button>
      </div>
    );
  }

  return (
    <div>
      <div>
        <h2>Downloaded Files</h2>
        <button onClick={fetchFiles}>Refresh</button>
      </div>

      {diskSpace && (
        <div>
          <p><strong>Disk Space Available:</strong> {formatFileSize(diskSpace.available)} / {formatFileSize(diskSpace.total)} {diskSpace.total > 0 && (
            <span> ({((diskSpace.available / diskSpace.total) * 100).toFixed(1)}% free)</span>
          )}</p>
        </div>
      )}

      {files.length === 0 ? (
        <p>No files in downloads directory.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Filename</th>
              <th>Size</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {files.map((file) => (
              <tr key={file.name}>
                <td>{file.name}</td>
                <td>{formatFileSize(file.size)}</td>
                <td>
                  <button onClick={() => handleDownload(file.name)}>
                    Download
                  </button>
                  <button onClick={() => handleDelete(file.name)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

export default Files;
