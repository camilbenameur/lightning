import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
// Global styles first so that component styles can build on them.
import './styles/global.css';
import './styles/controls.css';
import { App } from './App';

const container = document.getElementById('root');
if (!container) throw new Error('Missing #root element');

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
