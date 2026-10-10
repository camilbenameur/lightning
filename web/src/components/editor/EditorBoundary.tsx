import { Component, type ReactNode } from 'react';
import { AlertIcon } from '../icons';

interface EditorBoundaryProps {
  value: string;
  onChange: (value: string) => void;
  children: ReactNode;
}

/**
 * If Monaco fails to load (e.g. a flaky network while fetching its chunk), fall
 * back to a plain textarea so the playground stays usable.
 */
export class EditorBoundary extends Component<EditorBoundaryProps, { failed: boolean }> {
  override state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  override render() {
    if (!this.state.failed) return this.props.children;
    return (
      <div className="editor-fallback">
        <p className="editor-fallback__notice" role="alert">
          <AlertIcon />
          The code editor could not be loaded.
          <button
            type="button"
            className="editor-fallback__reload"
            onClick={() => window.location.reload()}
          >
            Reload
          </button>
        </p>
        <textarea
          className="plain-editor"
          aria-label="Code"
          spellCheck={false}
          autoCapitalize="off"
          autoComplete="off"
          autoCorrect="off"
          value={this.props.value}
          onChange={(event) => this.props.onChange(event.target.value)}
        />
      </div>
    );
  }
}
