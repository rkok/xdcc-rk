import express from 'express';
import XdccCli from './XdccCli.js';

async function startServer() {
  const xdccCli = new XdccCli();
  await xdccCli.initialize();

  const app = express();
  const port = 3000;

  app.set('view engine', 'ejs');
  app.set('views', './views');

  app.use(express.json());
  app.use(express.static('public'));

  app.get('/', (req, res) => {
    res.render('index', { title: 'XDCC CLI' });
  });

  app.get('/api/health', (req, res) => {
    res.json({ status: 'ok' });
  });

  app.get('/api/search', async (req, res) => {
    const searchString = req.query.searchString;

    if (!searchString || typeof searchString !== 'string') {
      res.status(400).json({ error: 'searchString query parameter is required' });
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

  app.listen(port, () => {
    console.log(`Server running at http://localhost:${port}`);
  });
}

startServer().catch((error) => {
  console.error('Failed to start server:', error);
  process.exit(1);
});

