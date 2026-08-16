import React, { useEffect } from "react";
import LiquidGlass from "liquid-glass-react";
import { CheckCircle2, AlertTriangle, Info, X } from "lucide-react";

export interface ToastMessage {
  id: string;
  type: "success" | "error" | "info";
  title: string;
  message: string;
}

interface ToastProps {
  toasts: ToastMessage[];
  onDismiss: (id: string) => void;
}

export const ToastContainer: React.FC<ToastProps> = ({ toasts, onDismiss }) => {
  return (
    <div className="fixed bottom-5 right-5 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onDismiss={onDismiss} />
      ))}
    </div>
  );
};

const ToastItem: React.FC<{ toast: ToastMessage; onDismiss: (id: string) => void }> = ({
  toast,
  onDismiss,
}) => {
  useEffect(() => {
    const timer = setTimeout(() => {
      onDismiss(toast.id);
    }, 5000);
    return () => clearTimeout(timer);
  }, [toast, onDismiss]);

  return (
    <LiquidGlass cornerRadius={12} padding="0px" blurAmount={0.02} displacementScale={20}>
    <div className="pointer-events-auto bg-[#1e1e22]/55 rounded-lg p-3 flex items-start gap-3 font-mono transition-all animate-slide-up">
      {toast.type === "success" && (
        <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
      )}
      {toast.type === "error" && (
        <AlertTriangle className="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
      )}
      {toast.type === "info" && (
        <Info className="w-4 h-4 text-indigo-400 shrink-0 mt-0.5" />
      )}

      <div className="flex-1 text-xs">
        <h4 className="font-bold text-white uppercase tracking-wider text-[11px]">{toast.title}</h4>
        <p className="text-zinc-400 mt-0.5 text-[11px] leading-snug">{toast.message}</p>
      </div>

      <button
        onClick={() => onDismiss(toast.id)}
        className="text-zinc-500 hover:text-white p-0.5 rounded hover:bg-zinc-800 transition-colors"
      >
        <X className="w-3.5 h-3.5" />
      </button>
    </div>
    </LiquidGlass>
  );
};
