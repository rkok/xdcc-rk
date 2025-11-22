import express from 'express';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';
import XdccCli from './XdccCli.js';
import FileManager from './FileManager.js';

async function startServer() {
  const xdccCli = new XdccCli();
  await xdccCli.initialize();

  const fileManager = new FileManager(xdccCli.resolveDownloadsPath());

  const app = express();
  const port = 3000;

  app.use(express.json());

  // Serve static files from the built React app (in production)
  const __dirname = dirname(fileURLToPath(import.meta.url));
  const publicPath = join(__dirname, '..', 'public');
  app.use(express.static(publicPath));

  app.get('/api/health', (req, res) => {
    res.json({ status: 'ok' });
  });

  app.post('/api/search', async (req, res) => {
    const { searchString } = req.body;

    if (!searchString || typeof searchString !== 'string') {
      res.status(400).json({ error: 'searchString is required in request body' });
      return;
    }

    try {
      const results = await xdccCli.search(searchString);
      res.json(results);
    } catch (error) {
      console.error('Search error:', error);
      res.status(500).json({ error: 'Search failed', details: error instanceof Error ? error.message : String(error) });
    }
  });

  app.get('/api/download', (req, res) => {
    const url = req.query.url;

    if (!url || typeof url !== 'string') {
      res.status(400).json({ error: 'url query parameter is required' });
      return;
    }

    // Set headers for Server-Sent Events
    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');

    // Spawn the download process
    const child = xdccCli.spawnDownload(url);

    // Stream stdout (JSONL events) to the client
    child.stdout?.on('data', (data) => {
      const lines = data.toString().split('\n');
      for (const line of lines) {
        if (line.trim()) {
          // Send each JSONL line as an SSE event
          res.write(`data: ${line}\n\n`);
        }
      }
    });

    // Handle stderr
    child.stderr?.on('data', (data) => {
      console.error('Download stderr:', data.toString());
    });

    // Handle process completion
    child.on('close', (code) => {
      console.log(`Download process exited with code ${code}`);
      res.end();
    });

    // Handle process errors
    child.on('error', (error) => {
      console.error('Download process error:', error);
      res.write(`data: ${JSON.stringify({ type: 'error', error: error.message, errorType: 'process', fatal: true })}\n\n`);
      res.end();
    });

    // Clean up when client disconnects
    req.on('close', () => {
      if (!child.killed) {
        child.kill();
      }
    });
  });

  // File management endpoints
  app.get('/api/files/list', async (req, res) => {
    try {
      const files = await fileManager.listFiles();
      res.json(files);
    } catch (error) {
      console.error('List files error:', error);
      res.status(500).json({ error: 'Failed to list files', details: error instanceof Error ? error.message : String(error) });
    }
  });

  app.post('/api/files/delete', async (req, res) => {
    const { filename } = req.body;

    if (!filename || typeof filename !== 'string') {
      res.status(400).json({ error: 'filename is required in request body' });
      return;
    }

    try {
      await fileManager.deleteFile(filename);
      res.json({ success: true });
    } catch (error) {
      console.error('Delete file error:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);

      // Return 404 for file not found, 400 for validation errors, 500 for other errors
      if (errorMessage.includes('ENOENT') || errorMessage.includes('Not a file')) {
        res.status(404).json({ error: 'File not found' });
      } else if (errorMessage.includes('cannot') || errorMessage.includes('Invalid')) {
        res.status(400).json({ error: errorMessage });
      } else {
        res.status(500).json({ error: 'Failed to delete file', details: errorMessage });
      }
    }
  });

  app.get('/api/files/download', async (req, res) => {
    const filename = req.query.file;

    if (!filename || typeof filename !== 'string') {
      res.status(400).json({ error: 'file query parameter is required' });
      return;
    }

    try {
      const filePath = fileManager.getFilePath(filename);

      // Use Express's res.download() for secure file streaming
      res.download(filePath, filename, (err) => {
        if (err) {
          console.error('Download file error:', err);
          if (!res.headersSent) {
            res.status(404).json({ error: 'File not found' });
          }
        }
      });
    } catch (error) {
      console.error('Get file path error:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);

      if (errorMessage.includes('cannot') || errorMessage.includes('Invalid')) {
        res.status(400).json({ error: errorMessage });
      } else {
        res.status(500).json({ error: 'Failed to download file', details: errorMessage });
      }
    }
  });

  // SPA fallback - serve index.html for all other routes (must be last)
  app.get('*', (req, res) => {
    res.sendFile(join(publicPath, 'index.html'));
  });

  app.listen(port, () => {
    console.log(`Server running at http://localhost:${port}`);
  });
}

startServer().catch((error) => {
  console.error('Failed to start server:', error);
  process.exit(1);
});

