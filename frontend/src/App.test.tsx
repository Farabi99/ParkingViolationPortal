import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import App from './App';

beforeEach(() => {
  localStorage.clear();
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation(query => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });

  window.fetch = vi.fn(() =>
    Promise.resolve({
      json: () => Promise.resolve({ data: [] }),
    })
  ) as unknown as typeof fetch;
});

describe('RBAC Routing', () => {
  it('redirects unauthenticated users to login', () => {
    render(<App />);
    expect(screen.getByText(/Parking Portal/i)).toBeInTheDocument();
  });

  it('allows OFFICER to access OFFICER routes', () => {
    localStorage.setItem('token', 'mock_token');
    localStorage.setItem('role', 'OFFICER');
    localStorage.setItem('isAuthenticated', 'true');
    window.history.pushState({}, 'Officer Submit', '/officer/submit');
    render(<App />);
    expect(screen.getAllByText('Submit Violation')[0]).toBeInTheDocument();
  });

  it('prevents MEMBER from accessing OFFICER routes', () => {
    localStorage.setItem('token', 'mock_token');
    localStorage.setItem('role', 'MEMBER');
    localStorage.setItem('isAuthenticated', 'true');
    window.history.pushState({}, 'Officer Submit', '/officer/submit');
    render(<App />);
    expect(screen.getByText('Transaction History')).toBeInTheDocument();
  });
});
