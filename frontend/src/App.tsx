import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, Link } from 'react-router-dom';
import { AuthProvider, useAuth } from './AuthContext';
import Login from './pages/Login';
import OfficerSubmit from './pages/OfficerSubmit';
import OfficerRules from './pages/OfficerRules';
import MemberViolations from './pages/MemberViolations';
import Payment from './pages/Payment';

const ProtectedRoute = ({ children, allowedRole }: { children: React.ReactNode, allowedRole: string }) => {
  const { isAuthenticated, role } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" />;
  if (role !== allowedRole) return <Navigate to="/" />;
  return <>{children}</>;
};

const Navigation = () => {
  const { role, logout } = useAuth();
  if (!role) return null;

  return (
    <nav className="navbar">
      <h2 style={{ margin: 0, fontSize: '1.5rem' }}>🅿️ Parking Portal</h2>
      <div className="nav-links items-center">
        {role === 'OFFICER' && (
          <>
            <Link to="/officer/submit">Submit Violation</Link>
            <Link to="/officer/rules">Manage Rules</Link>
          </>
        )}
        {role === 'MEMBER' && (
          <>
            <Link to="/member/violations">My History</Link>
            <Link to="/member/pay">Make Payment</Link>
          </>
        )}
        <button className="outline" onClick={() => {
          const newTheme = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
          document.documentElement.setAttribute('data-theme', newTheme);
          localStorage.setItem('theme', newTheme);
        }} style={{ padding: '0.5rem', borderRadius: '50%' }}>🌓</button>
        <button className="outline" onClick={logout} style={{ padding: '0.5rem 1rem' }}>Logout</button>
      </div>
    </nav>
  );
};

const DefaultRedirect = () => {
  const { role } = useAuth();
  if (role === 'OFFICER') return <Navigate to="/officer/submit" />;
  if (role === 'MEMBER') return <Navigate to="/member/violations" />;
  return <Navigate to="/login" />;
};

function App() {
  React.useEffect(() => {
    const savedTheme = localStorage.getItem('theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    document.documentElement.setAttribute('data-theme', savedTheme);
  }, []);

  return (
    <AuthProvider>
      <BrowserRouter>
        <Navigation />
        <div className="container">
          <Routes>
            <Route path="/login" element={<Login />} />
            
            <Route path="/officer/submit" element={
              <ProtectedRoute allowedRole="OFFICER"><OfficerSubmit /></ProtectedRoute>
            } />
            <Route path="/officer/rules" element={
              <ProtectedRoute allowedRole="OFFICER"><OfficerRules /></ProtectedRoute>
            } />
            
            <Route path="/member/violations" element={
              <ProtectedRoute allowedRole="MEMBER"><MemberViolations /></ProtectedRoute>
            } />
            <Route path="/member/pay" element={
              <ProtectedRoute allowedRole="MEMBER"><Payment /></ProtectedRoute>
            } />

            <Route path="*" element={<DefaultRedirect />} />
          </Routes>
        </div>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
