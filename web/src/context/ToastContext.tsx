import { CheckCircle, XCircle } from 'lucide-react';
import { createContext, useContext, useState, type ReactNode } from 'react';

// Types
interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

interface ToastContextType {
    showToast: (message: string, type: 'success' | 'error') => void;
}

// Create Context
const ToastContext = createContext<ToastContextType | undefined>(undefined);

// The Provider Component
export function ToastProvider({ children }: { children: ReactNode }) {
    const [toasts, setToasts] = useState<Toast[]>([]);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);

        // Auto-remove after 3 seconds
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3000);
    };

    return (
        <ToastContext.Provider value={{ showToast }}>
            {children}

            {/* GLOBAL TOAST CONTAINER */}
            <div className="fixed bottom-6 right-6 z-[100] flex flex-col gap-2">
                {toasts.map(toast => (
                    <div
                        key={toast.id}
                        className={`
                            flex items-center gap-3 px-4 py-3 rounded shadow-lg border backdrop-blur-md animate-slide-in
                            ${toast.type === 'success'
                                ? 'bg-green-900/80 border-green-500/50 text-green-100'
                                : 'bg-red-900/80 border-red-500/50 text-red-100'}
                        `}
                    >
                        {toast.type === 'success' ? <CheckCircle size={18} /> : <XCircle size={18} />}
                        <span className="font-mono text-sm">{toast.message}</span>
                    </div>
                ))}
            </div>
        </ToastContext.Provider>
    );
}

// Custom Hook for easy usage
export function useToast() {
    const context = useContext(ToastContext);
    if (!context) {
        throw new Error('useToast must be used within a ToastProvider');
    }
    return context;
}

