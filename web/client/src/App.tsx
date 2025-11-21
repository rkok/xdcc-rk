import { Routes, Route } from 'react-router-dom'

import { useSearchParams } from "react-router";
import Xdcc from "./Xdcc.tsx";
import Files from "./Files.tsx";

function Home() {
  const [searchParams, setSearchParams] = useSearchParams();
  const currentTab = searchParams.get('tab') || 'xdcc';

  return (
    <div className="App">
      <nav>
        <button onClick={() => setSearchParams({ tab: 'xdcc' })} disabled={currentTab === 'xdcc'}>
          XDCC
        </button>
        <button onClick={() => setSearchParams({ tab: 'files' })} disabled={currentTab === 'files'}>
          Files
        </button>
      </nav>

      <div {...{hidden: currentTab !== 'xdcc'}}>
        <Xdcc />
      </div>

      <div {...{hidden: currentTab !== 'files'}}>
        <Files />
      </div>
    </div>
  )
}

function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
    </Routes>
  )
}

export default App

