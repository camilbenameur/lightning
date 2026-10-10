import type { ReactNode } from 'react';
import {
  Group,
  Panel,
  Separator,
  useDefaultLayout,
  type LayoutStorage,
} from 'react-resizable-panels';
import { useMediaQuery } from '../hooks/useMediaQuery';
import { readItem, writeItem } from '../lib/storage';
import './WorkspaceLayout.css';

const layoutStorage: LayoutStorage = { getItem: readItem, setItem: writeItem };

interface WorkspaceLayoutProps {
  editor: ReactNode;
  stdin: ReactNode;
  output: ReactNode;
}

/** Desktop: resizable editor | (stdin / output) split, remembered across visits. */
function SplitLayout({ editor, stdin, output }: WorkspaceLayoutProps) {
  const columns = useDefaultLayout({
    id: 'layout:columns',
    storage: layoutStorage,
    onlySaveAfterUserInteractions: true,
  });
  const rows = useDefaultLayout({
    id: 'layout:io',
    storage: layoutStorage,
    onlySaveAfterUserInteractions: true,
  });

  return (
    <Group
      className="workspace"
      orientation="horizontal"
      defaultLayout={columns.defaultLayout}
      onLayoutChanged={columns.onLayoutChanged}
    >
      <Panel id="editor" defaultSize="60%" minSize="25%">
        {editor}
      </Panel>
      <Separator className="split-handle" />
      <Panel id="io" minSize="20%">
        <Group
          orientation="vertical"
          defaultLayout={rows.defaultLayout}
          onLayoutChanged={rows.onLayoutChanged}
        >
          <Panel id="stdin" defaultSize="30%" minSize="10%">
            {stdin}
          </Panel>
          <Separator className="split-handle" />
          <Panel id="output" minSize="20%">
            {output}
          </Panel>
        </Group>
      </Panel>
    </Group>
  );
}

/** Resizable split on desktop, a simple scrolling stack on narrow screens. */
export function WorkspaceLayout(props: WorkspaceLayoutProps) {
  const isWide = useMediaQuery('(min-width: 768px)');
  if (isWide) return <SplitLayout {...props} />;
  return (
    <div className="workspace workspace--stacked">
      <div className="workspace__editor">{props.editor}</div>
      <div className="workspace__stdin">{props.stdin}</div>
      <div className="workspace__output">{props.output}</div>
    </div>
  );
}
