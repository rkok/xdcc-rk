import { access, constants } from 'fs/promises';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';
import { ChildProcess, spawn } from 'child_process';

/**
 * XdccCli class handles interaction with the xdcc-cli binary.
 */
class XdccCli {
  private binaryPath: string;

  /**
   * Creates an instance of XdccCli.
   * The binary path is resolved from XDCC_PATH environment variable,
   * or defaults to <webdir>/../bin/xdcc.
   * The binary is verified to exist and be executable upon construction.
   */
  constructor() {
    this.binaryPath = this.resolveBinaryPath();
  }

  /**
   * Initializes the XdccCli instance by verifying the binary exists and is executable.
   * This must be called after construction and before using the instance.
   */
  async initialize(): Promise<void> {
    await this.verifyBinary();
  }

  /**
   * Resolves the path to the xdcc binary.
   * Checks XDCC_PATH environment variable first, otherwise uses default path.
   */
  private resolveBinaryPath(): string {
    let xdccPath = process.env.XDCC_PATH;
    if (!xdccPath) {
      // Default: <webdir>/../bin/xdcc (from dist/server/XdccCli.js -> ../../bin/xdcc)
      xdccPath = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', 'bin', 'xdcc');
    }
    return xdccPath;
  }

  /**
   * Resolves the path to the downloads directory.
   * Checks XDCC_DOWNLOADS_PATH environment variable first, otherwise uses default path.
   */
  resolveDownloadsPath(): string {
    let downloadsPath = process.env.XDCC_DOWNLOADS_PATH;
    if (!downloadsPath) {
      // Default: <webdir>/downloads (from dist/server/XdccCli.js -> ../../downloads)
      downloadsPath = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..', 'downloads');
    }
    return downloadsPath;
  }

  /**
   * Verifies that the XDCC binary exists and is executable.
   * Exits the process if the binary is not found or not executable.
   */
  private async verifyBinary(): Promise<void> {
    try {
      await access(this.binaryPath, constants.F_OK | constants.X_OK);
      console.log(`✓ XDCC binary found at: ${this.binaryPath}`);
    } catch (error) {
      console.error(`✗ XDCC binary not found or not executable at: ${this.binaryPath}`);
      console.error('Please ensure xdcc-cli is built (run "make") or set XDCC_PATH environment variable.');
      process.exit(1);
    }
  }

  /**
   * Executes a search command and returns the JSON results.
   * @param searchString The search query string
   * @returns Promise resolving to the parsed JSON search results
   */
  async search(searchString: string): Promise<any> {
    return new Promise((resolve, reject) => {
      const args = ['search', searchString, '--format', 'json'];
      const child = spawn(this.binaryPath, args);

      let stdout = '';
      let stderr = '';

      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });

      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });

      child.on('close', (code) => {
        if (code !== 0) {
          reject(new Error(`xdcc search failed with code ${code}: ${stderr}`));
          return;
        }

        try {
          const result = JSON.parse(stdout);
          resolve(result);
        } catch (error) {
          reject(new Error(`Failed to parse JSON output: ${error}`));
        }
      });

      child.on('error', (error) => {
        reject(new Error(`Failed to spawn xdcc process: ${error.message}`));
      });
    });
  }

  /**
   * Spawns a download process for the given URL with JSONL output.
   * Returns the child process for streaming output.
   * @param url The IRC URL to download
   * @returns The spawned child process
   */
  spawnDownload(url: string): ChildProcess {
    const args = ['get', url, '--format', 'jsonl', '-o', this.resolveDownloadsPath(), '--sanitize-filenames'];
    return spawn(this.binaryPath, args);
  }
}

export default XdccCli;
