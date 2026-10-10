import type { Toast as ToastData } from '../hooks/useToast';
import './Toast.css';

/** Transient notification. The live region is always mounted so that updates are announced. */
export function Toast({ toast }: { toast: ToastData | null }) {
  return (
    <div className="toast-region" role="status">
      {toast && (
        <div key={toast.id} className="toast">
          {toast.message}
        </div>
      )}
    </div>
  );
}
