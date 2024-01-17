import React, { useState } from 'react';
import Editor, { OnChange } from '@monaco-editor/react';
import './App.css';
import Sidebar from './Sidebar';

function App(): JSX.Element {
  const URL = "http://162.19.69.29:8080";
  const [script, setScript] = useState<string>('print("Hello, world!")');
  const [result, setResult] = useState<string>('');

  const fetchData = async () => {
    try {
      const res = await fetch(URL + "/run", {
        method: "POST",
        body: JSON.stringify({
          "language": "python",
          "script": script,
        }),
        headers: {
          "Content-Type": "application/json"
        }
      });

      if (res.ok) {
        const data = await res.json();
        const formattedResult = data.result
          .replace(/[\u0000-\u001F]+/g, "") // Remove control characters
        setResult(formattedResult);
      } else {
        setResult('Request failed with status: ' + res.status);
      }
    } catch (error) {
      setResult('An error occurred: ' + (error as Error).message);
    }
  };

  const handleEditorChange: OnChange = (value, event) => {
    setScript(value || '');
  };

  return (
    <div className="app">
      <Sidebar /> 
      <div className="main-content">
        <div className="editor-container">
          <Editor
            language="python"
            theme="vs-dark"
            value={script}
            onChange={handleEditorChange}
          />
          <button className='run-button' onClick={fetchData}>Run</button>
        </div>
        <div className="result-container">
          <pre className="result-textarea">{result}</pre>
        </div>
      </div>
    </div>
  );
}

export default App;