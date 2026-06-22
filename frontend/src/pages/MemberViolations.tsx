import { useEffect, useState } from 'react';


interface Violation {
  id: string;
  license_plate: string;
  type: string;
  timestamp: string;
  location: string;
  photo_url?: string;
  applied_rule_set_version?: number;
  fine_amount?: number;
  status: string;
}

export default function MemberViolations() {
  const [violations, setViolations] = useState<Violation[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);


  const loadHistory = async (cursor: string | null = null) => {
    setLoading(true);
    let url = '/api/violations/history';
    if (cursor) url += `?cursor=${cursor}`;
    
    try {
      const res = await fetch(url);
      const data = await res.json();
      
      if (cursor) {
        setViolations(prev => [...prev, ...(data.data || [])]);
      } else {
        setViolations(data.data || []);
      }
      setNextCursor(data.next_cursor || null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadHistory();
  }, []);

  return (
    <div className="glass-panel">
      <h2>Transaction History</h2>
      
      <div style={{ overflowX: 'auto' }}>
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>License Plate</th>
              <th>Type</th>
              <th>Date</th>
              <th>Rule Version</th>
              <th>Fine Amount</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {violations.map(v => (
              <tr key={v.id}>
                <td>{v.id}</td>
                <td>{v.license_plate}</td>
                <td>{v.type.replace(/_/g, ' ')}</td>
                <td>{new Date(v.timestamp).toLocaleString()}</td>
                <td>{v.applied_rule_set_version ? `v${v.applied_rule_set_version}` : 'N/A'}</td>
                <td>{v.fine_amount ? `Rp ${v.fine_amount.toLocaleString()}` : 'Calculating...'}</td>
                <td>
                  <span className={`badge ${v.status === 'PAID' ? 'success' : v.status === 'UNPAID' ? 'danger' : 'info'}`}>
                    {v.status}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {nextCursor && (
        <div className="text-center mt-4">
          <button className="outline" onClick={() => loadHistory(nextCursor)} disabled={loading}>
            {loading ? 'Loading...' : 'Load More'}
          </button>
        </div>
      )}
    </div>
  );
}
