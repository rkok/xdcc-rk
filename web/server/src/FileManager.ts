import { readdir, stat, statfs, unlink } from 'fs/promises';
import { join, normalize, resolve, sep } from 'path';

export interface FileInfo {
  name: string;
  size: number;
}

export interface ListFilesResponse {
  files: FileInfo[];
  diskSpace: {
    available: number;
    total: number;
  };
}

/**
 * FileManager class handles file operations in the downloads directory.
 * All operations are secured against path traversal attacks.
 */
class FileManager {
  private downloadsPath: string;

  /**
   * Creates an instance of FileManager.
   * @param downloadsPath Absolute path to the downloads directory
   */
  constructor(downloadsPath: string) {
    this.downloadsPath = downloadsPath;
  }

  /**
   * Lists all files in the downloads directory with their sizes and disk space info.
   * Only returns regular files, skips directories and hidden files.
   * @returns Promise resolving to ListFilesResponse with files and disk space info
   */
  async listFiles(): Promise<ListFilesResponse> {
    try {
      const entries = await readdir(this.downloadsPath, { withFileTypes: true });
      const files: FileInfo[] = [];

      for (const entry of entries) {
        // Skip directories and hidden files
        if (!entry.isFile() || entry.name.startsWith('.')) {
          continue;
        }

        const filePath = join(this.downloadsPath, entry.name);
        const stats = await stat(filePath);

        files.push({
          name: entry.name,
          size: stats.size,
        });
      }

      // Get disk space information
      const fsStats = await statfs(this.downloadsPath);
      const diskSpace = {
        available: fsStats.bavail * fsStats.bsize,
        total: fsStats.blocks * fsStats.bsize,
      };

      return { files, diskSpace };
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === 'ENOENT') {
        // Downloads directory doesn't exist yet, return empty array with zero disk space
        return {
          files: [],
          diskSpace: { available: 0, total: 0 },
        };
      }
      throw error;
    }
  }

  /**
   * Deletes a file from the downloads directory.
   * Validates the filename to prevent path traversal attacks.
   * @param filename The name of the file to delete
   * @throws Error if filename is invalid or file doesn't exist
   */
  async deleteFile(filename: string): Promise<void> {
    const filePath = this.validateAndGetFilePath(filename);

    // Verify it's a file (not a directory)
    const stats = await stat(filePath);
    if (!stats.isFile()) {
      throw new Error('Not a file');
    }

    await unlink(filePath);
  }

  /**
   * Gets the validated absolute path for a file in the downloads directory.
   * @param filename The name of the file
   * @returns The absolute path to the file
   * @throws Error if filename is invalid or path traversal is detected
   */
  getFilePath(filename: string): string {
    return this.validateAndGetFilePath(filename);
  }

  /**
   * Validates a filename and returns its absolute path.
   * Prevents path traversal attacks by ensuring the resolved path
   * stays within the downloads directory.
   * @param filename The filename to validate
   * @returns The validated absolute path
   * @throws Error if validation fails
   */
  private validateAndGetFilePath(filename: string): string {
    // Reject empty filenames
    if (!filename || filename.trim() === '') {
      throw new Error('Filename cannot be empty');
    }

    // Reject filenames with path separators
    if (filename.includes('/') || filename.includes('\\') || filename.includes(sep)) {
      throw new Error('Filename cannot contain path separators');
    }

    // Reject filenames with path traversal attempts
    if (filename.includes('..')) {
      throw new Error('Filename cannot contain ".."');
    }

    // Normalize and resolve the full path
    const normalizedFilename = normalize(filename);
    const fullPath = resolve(this.downloadsPath, normalizedFilename);

    // Ensure the resolved path is within the downloads directory
    // This is the critical security check
    if (!fullPath.startsWith(this.downloadsPath + sep) && fullPath !== this.downloadsPath) {
      throw new Error('Invalid file path');
    }

    return fullPath;
  }
}

export default FileManager;

