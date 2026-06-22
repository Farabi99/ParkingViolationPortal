import React, { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../AuthContext';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const { login, isAuthenticated } = useAuth();

  if (isAuthenticated) {
    return <Navigate to="/" />;
  }

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
      });
      if (!res.ok) throw new Error('Invalid credentials');
      
      const data = await res.json();
      login(data.role);
    } catch {
      setError('Invalid username or password (use password: "password")');
    }
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
      <div className="glass-panel" style={{ width: '100%', maxWidth: '400px' }}>
        <h2 className="text-center mb-4" style={{ color: 'var(--primary)' }}>Parking Portal</h2>
        {error && <div className="badge danger mb-2 text-center" style={{ display: 'block' }}>{error}</div>}
        <form onSubmit={handleLogin}>
          <div className="mb-2">
            <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>Username</label>
            <input 
              type="text" 
              placeholder="officer1 or member1" 
              value={username} 
              onChange={e => setUsername(e.target.value)} 
              required 
            />
          </div>
          <div className="mb-4">
            <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>Password</label>
            <input 
              type="password" 
              placeholder="password" 
              value={password} 
              onChange={e => setPassword(e.target.value)} 
              required 
            />
          </div>
          <button type="submit" style={{ width: '100%' }}>Login</button>
        </form>
      </div>
    </div>
  );
}
