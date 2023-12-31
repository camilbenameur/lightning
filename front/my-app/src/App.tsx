import React, { useState } from 'react';
import './App.css';

function App() {
  const URL = "http://162.19.69.29:8080";
  const [result, setResult] = useState('');

  const fetchData = async () => {
    try {
      const res = await fetch(URL + "/run", {
        method: "POST",
        body: JSON.stringify({
          "language": "python",
          "script": `print('Hello World', 5+5)`,
        }),
        headers: {
          "Content-Type": "application/json"
        }
      });

      if (res.ok) {
        const data = await res.json();
        setResult('Request successful: ' + JSON.stringify(data));
      } else {
        setResult('Request failed with status: ' + res.status);
      }
    } catch (error) {
      setResult('An error occurred: ' + (error as Error).message);
    }
  };

  return (
    <div>
      <button onClick={fetchData}>Make Request</button>
      <textarea value={result} readOnly rows={4} cols={50} />
    </div>
  );
}

export default App;
