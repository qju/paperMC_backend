import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import DashboardLayout from './layouts/DashboardLayout';
import Console from './pages/Console';
import Players from './pages/Players';
import Worlds from './pages/Worlds';
import Updater from './pages/Updater';
import Users from './pages/Users';
import type { JSX } from 'react';

// Placeholder Pages (We will build these one by one)
//const Players = () => <h1 className="text-2xl font-pixel text-mc-gold">Player Management</h1>;

function ProtectedRoute({ children }: { children: JSX.Element }) {
    const token = localStorage.getItem('token');
    if (!token) return <Navigate to="/login" replace />;
    return children;
}

function App() {
    return (
        <BrowserRouter>
            <Routes>
                <Route path="/login" element={<Login />} />

                {/* The Dashboard Layout wraps these routes */}
                <Route path="/" element={
                    <ProtectedRoute>
                        <DashboardLayout />
                    </ProtectedRoute>
                }>
                    <Route index element={<Console />} />
                    <Route path="players" element={<Players />} />
                    <Route path="worlds" element={<Worlds />} />
                    <Route path="updater" element={<Updater />} />
                    <Route path="users" element={<Users />} />
                    <Route path="*" element={<Navigate to="/" replace />} />
                    {/* Add Config, Backups later */}
                </Route>
                <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
        </BrowserRouter>
    );
}

export default App;

