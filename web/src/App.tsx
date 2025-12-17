import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import DashboardLayout from './layouts/DashboardLayout';
import Console from './pages/Console';
import type { JSX } from 'react';

// Placeholder Pages (We will build these one by one)
const PlayersPage = () => <h1 className="text-2xl font-pixel text-mc-gold">Player Management</h1>;

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
                    <Route path="players" element={<PlayersPage />} />
                    {/* Add Config, Backups later */}
                </Route>
            </Routes>
        </BrowserRouter>
    );
}

export default App;

