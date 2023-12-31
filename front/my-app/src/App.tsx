import React, { useState } from 'react';
import './App.css';

function App() {
  const URL = "http://162.19.69.29:8080";
  const [script, setScript] = useState('print("Hello, world!")');
  const [result, setResult] = useState('');

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
        setResult(JSON.stringify(data));
      } else {
        setResult('Request failed with status: ' + res.status);
      }
    } catch (error) {
      setResult('An error occurred: ' + (error as Error).message);
    }
  };

  const handleScriptChange = (event) => {
    setScript(event.target.value);
  };

  return (
    <div className="container">
      <div className="textarea-container">
        <label>
          Enter Python script:
          <textarea
            value={script}
            onChange={handleScriptChange}
            className="textarea"
          />
        </label>
        <button type="button" onClick={fetchData}>Run</button>
      </div>
      <div className="result-container">
        <label>Result:</label>
        <div>
          <textarea value={result} readOnly className="result-textarea" />
        </div>
      </div>
    </div>
  );
}

export default App;
