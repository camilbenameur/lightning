import { Panel } from './Panel';
import './StdinPanel.css';

interface StdinPanelProps {
  value: string;
  onChange: (stdin: string) => void;
}

export function StdinPanel({ value, onChange }: StdinPanelProps) {
  return (
    <Panel title="Input">
      <textarea
        className="stdin"
        aria-label="Standard input"
        placeholder="Text passed to your program on stdin"
        spellCheck={false}
        autoCapitalize="off"
        autoComplete="off"
        autoCorrect="off"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </Panel>
  );
}
