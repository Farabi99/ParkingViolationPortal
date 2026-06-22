import React, { useState, useEffect } from 'react';


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

export default function Payment() {
  const [violations, setViolations] = useState<Violation[]>([]);
  const [activeRules, setActiveRules] = useState<Record<string, unknown> | null>(null);
  const [violationRules, setViolationRules] = useState<Record<string, unknown> | null>(null);
  const [selectedViolationId, setSelectedViolationId] = useState<string>('');
  const [scenario, setScenario] = useState('success');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<{text: string, type: 'success' | 'danger'} | null>(null);


  useEffect(() => {
    fetch('/api/rules/active')
      .then(res => res.json())
      .then(data => setActiveRules(data.rules))
      .catch(() => {});

    fetch('/api/violations/history')
      .then(res => res.json())
      .then(data => {
        const historyData = data.data || [];
        const unpaid = historyData.filter((v: Violation) => v.status === 'UNPAID');
        setViolations(unpaid);
        if (unpaid.length > 0) {
          setSelectedViolationId(unpaid[0].id.toString());
        }
      });
  }, []);

  const selectedViolation = violations.find(v => v.id.toString() === selectedViolationId);

  useEffect(() => {
    if (selectedViolation && selectedViolation.applied_rule_set_version) {
      fetch(`/api/rules/${selectedViolation.applied_rule_set_version}`)
        .then(res => res.json())
        .then(data => setViolationRules(data.rules))
        .catch(() => {});
    } else if (activeRules) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setViolationRules(activeRules);
    }
  }, [selectedViolation, activeRules]);

  let formulaDisplay = null;
  if (selectedViolation && violationRules && selectedViolation.fine_amount) {
    const rules = violationRules as { 
      base_amount: Record<string, number>, 
      time_multiplier: Array<{start: string, end: string, multiplier: number}> 
    };
    const base = rules.base_amount[selectedViolation.type] || 0;
    
    let timeMult = 1.0;
    let timeRangeLabel = 'Time';
    const vTime = new Date(selectedViolation.timestamp).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' });
    if (rules.time_multiplier) {
      for (const tr of rules.time_multiplier) {
        if (tr.start <= tr.end) {
          if (vTime >= tr.start && vTime <= tr.end) { timeMult = tr.multiplier; timeRangeLabel = `(${tr.start}-${tr.end})`; }
        } else {
          if (vTime >= tr.start || vTime <= tr.end) { timeMult = tr.multiplier; timeRangeLabel = `(${tr.start}-${tr.end})`; }
        }
      }
    }

    let repeatMult = 1.0;
    if (base > 0 && timeMult > 0) {
      // Calculate repeat multiplier by dividing total by known multipliers
      repeatMult = parseFloat((selectedViolation.fine_amount / (base * timeMult)).toFixed(2));
    }
    
    formulaDisplay = (
      <div style={{ padding: '1rem', backgroundColor: 'var(--bg-color)', borderRadius: '6px', marginTop: '0.5rem', fontFamily: 'monospace', color: 'var(--text-main)', fontSize: '0.9rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.25rem' }}>
          <span>{selectedViolation.type.replace(/_/g, ' ')} fine</span>
          <span>Rp {base.toLocaleString()}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.25rem' }}>
          <span>{timeRangeLabel} Multiplier</span>
          <span>{timeMult}x</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
          <span>Unpaid Fine Multiplier</span>
          <span>{repeatMult}x</span>
        </div>
        <div className="badge danger" style={{ marginTop: '0.5rem', display: 'flex', justifyContent: 'space-between', fontWeight: 'bold', fontSize: '1.1rem', padding: '0.75rem', borderRadius: '4px', width: '100%' }}>
          <span>Total ({base.toLocaleString()} × {timeMult} × {repeatMult})</span>
          <span>Rp {selectedViolation.fine_amount.toLocaleString()}</span>
        </div>
      </div>
    );
  }

  const handlePayment = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedViolation) return;
    
    setLoading(true);
    setMessage(null);

    // Deterministic idempotency key per violation
    const idempotencyKey = `pay-violation-${selectedViolation.id}`;

    try {
      const res = await fetch('/api/payment', {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          'Idempotency-Key': idempotencyKey
        },
        body: JSON.stringify({
          violation_id: selectedViolation.id,
          amount: selectedViolation.fine_amount || 0,
          scenario
        })
      });

      if (!res.ok) {
        if (res.status === 409) throw new Error('Payment already processing or completed.');
        throw new Error('Payment service error');
      }
      
      const data = await res.json();
      if (data.status === 'PAID') {
        setMessage({ text: `Payment Successful! TX ID: ${data.transaction_id}`, type: 'success' });
        setViolations(violations.filter(v => v.id.toString() !== selectedViolationId));
        setSelectedViolationId('');
      } else {
        setMessage({ text: `Payment Failed! TX ID: ${data.transaction_id}`, type: 'danger' });
      }
    } catch (err: unknown) {
      const e = err as Error;
      setMessage({ text: e.message || 'Failed to process payment.', type: 'danger' });
    } finally {
      setLoading(false);
    }
  };

  if (violations.length === 0 && !message) {
    return (
      <div className="glass-panel text-center" style={{ maxWidth: '500px', margin: '0 auto' }}>
        <h2>Mock Payment</h2>
        <p className="text-muted">You have no unpaid violations!</p>
      </div>
    );
  }

  return (
    <div className="glass-panel" style={{ maxWidth: '600px', margin: '0 auto' }}>
      <h2>Mock Payment</h2>
      {message && <div className={`badge ${message.type} mb-4`} style={{ display: 'inline-block' }}>{message.text}</div>}
      
      <form onSubmit={handlePayment}>
        <div className="mb-4">
          <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>Select Unpaid Violation</label>
          <select value={selectedViolationId} onChange={e => {
            setSelectedViolationId(e.target.value);
            setViolationRules(null);
          }}>
            {violations.map(v => (
              <option key={v.id} value={v.id}>
                {v.license_plate} - {v.type.replace(/_/g, ' ')} - {new Date(v.timestamp).toLocaleDateString('en-GB')}
              </option>
            ))}
          </select>
        </div>

        {selectedViolation && (
          <div className="mb-4" style={{ backgroundColor: 'var(--surface-hover)', padding: '1rem', borderRadius: '8px' }}>
            <h3 style={{ marginBottom: '0.5rem', color: 'var(--text-main)' }}>Violation Details</h3>
            <p className="text-muted mb-1"><strong>Type:</strong> <span style={{ color: 'var(--text-main)', fontWeight: 500 }}>{selectedViolation.type.replace(/_/g, ' ')}</span></p>
            <p className="text-muted mb-1"><strong>Time:</strong> <span style={{ color: 'var(--text-main)', fontWeight: 500 }}>{new Date(selectedViolation.timestamp).toLocaleString()}</span></p>
            <p className="text-muted mb-1"><strong>Location:</strong> <span style={{ color: 'var(--text-main)', fontWeight: 500 }}>{selectedViolation.location}</span></p>
            <p className="text-muted mb-1"><strong>Rule Version:</strong> <span style={{ color: 'var(--text-main)', fontWeight: 500 }}>{selectedViolation.applied_rule_set_version ? `v${selectedViolation.applied_rule_set_version}` : 'N/A'}</span></p>
            
            {selectedViolation.photo_url && (
              <>
                <p className="text-muted mb-1"><strong>Photo Evidence:</strong></p>
                <img src={selectedViolation.photo_url} alt="Violation" style={{ width: '100%', maxHeight: '200px', objectFit: 'contain', borderRadius: '4px', border: '1px solid var(--border)', marginBottom: '1rem' }} />
              </>
            )}

            <div style={{ padding: '0.5rem', backgroundColor: 'rgba(0,0,0,0.2)', borderRadius: '4px', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              {formulaDisplay}
            </div>
          </div>
        )}

        {selectedViolation && (
          <>
            <div className="mb-4">
              <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>Scenario Simulation</label>
              <select value={scenario} onChange={e => setScenario(e.target.value)}>
                <option value="success">Simulate Success</option>
                <option value="failed">Simulate Failure</option>
              </select>
            </div>
            <button type="submit" disabled={loading || !selectedViolation.fine_amount} style={{ width: '100%' }}>
              {loading ? 'Processing...' : 'Pay Now'}
            </button>
          </>
        )}
      </form>
    </div>
  );
}
