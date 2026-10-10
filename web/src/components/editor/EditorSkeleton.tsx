const LINE_WIDTHS = [38, 62, 0, 24, 71, 55, 66, 0, 18, 47, 30];

/** Lightweight placeholder shown while the Monaco chunk downloads. */
export function EditorSkeleton() {
  return (
    <div className="editor-skeleton" role="progressbar" aria-label="Loading editor">
      {LINE_WIDTHS.map((width, index) => (
        <div key={index} className="editor-skeleton__line">
          <span className="editor-skeleton__gutter">{index + 1}</span>
          {width > 0 && <span className="editor-skeleton__bar" style={{ width: `${width}%` }} />}
        </div>
      ))}
    </div>
  );
}
