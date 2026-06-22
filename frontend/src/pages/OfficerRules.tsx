import { useEffect, useState } from 'react';


interface Rules {
  base_amount: Record<string, number>;
  time_multiplier: Array<{start: string, end: string, multiplier: number}>;
  repeat_multiplier: Array<{prior_unpaid: number, multiplier: number}>;
}

export default function OfficerRules() {
  const [baseRules, setBaseRules] = useState<Array<{key: string, value: number}>>([]);
  const [timeRules, setTimeRules] = useState<Array<{start: string, end: string, multiplier: number}>>([]);
  const [repeatRules, setRepeatRules] = useState<Array<{prior_unpaid: number, multiplier: number}>>([]);
  const [version, setVersion] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);


  useEffect(() => {
    fetch('/api/rules/active')
      .then(res => res.json())
      .then(data => {
        const rules: Rules = data.rules;
        setBaseRules(Object.entries(rules.base_amount || {}).map(([k, v]) => ({key: k, value: v})));
        setTimeRules(rules.time_multiplier || []);
        setRepeatRules(rules.repeat_multiplier || []);
        setVersion(data.version);
      });
  }, []);

  const handleUpdate = async () => {
    setLoading(true);
    
    // Validate time rules overlap
    const minutes = new Array(1440).fill(false);
    let hasOverlap = false;

    for (const tr of timeRules) {
      if (!tr.start || !tr.end) continue;
      const [sh, sm] = tr.start.split(':').map(Number);
      const [eh, em] = tr.end.split(':').map(Number);
      const startMin = sh * 60 + sm;
      const endMin = eh * 60 + em;

      const checkAndSet = (m: number) => {
        if (minutes[m]) hasOverlap = true;
        minutes[m] = true;
      };

      if (startMin <= endMin) {
        for (let m = startMin; m < endMin; m++) checkAndSet(m);
      } else {
        for (let m = startMin; m < 1440; m++) checkAndSet(m);
        for (let m = 0; m < endMin; m++) checkAndSet(m);
      }
    }

    if (hasOverlap) {
      alert("Time rules cannot overlap!");
      setLoading(false);
      return;
    }

    // Construct final rules object
    const finalBaseAmount: Record<string, number> = {};
    baseRules.forEach(br => {
      if (br.key.trim() !== '') finalBaseAmount[br.key] = br.value;
    });

    const finalRules: Rules = {
      base_amount: finalBaseAmount,
      time_multiplier: timeRules,
      repeat_multiplier: repeatRules
    };

    try {
      const res = await fetch('/api/rules/', {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json' 
        },
        body: JSON.stringify(finalRules)
      });
      if (res.ok) {
        const data = await res.json();
        setVersion(data.version);
        alert('Rules updated! New version: ' + data.version);
      } else {
        alert('Failed to update rules');
      }
    } catch {
      alert('Error updating rules');
    } finally {
      setLoading(false);
    }
  };

  if (version === null) return <div>Loading rules...</div>;

  return (
    <div className="glass-panel" style={{ maxWidth: '800px', margin: '0 auto' }}>
      <div className="flex justify-between items-center mb-4">
        <h2>Active Fine Rules</h2>
        <span className="badge info">Version: {version}</span>
      </div>
      
      <div className="mb-4">
        <div className="flex justify-between items-center mb-2">
          <h3>Base Amounts (IDR)</h3>
          <button className="outline" onClick={() => setBaseRules([...baseRules, {key: '', value: 0}])} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8rem' }}>+ Add</button>
        </div>
        <div style={{ display: 'grid', gap: '1rem', gridTemplateColumns: '1fr 1fr' }}>
          {baseRules.map((br, idx) => (
            <div key={idx} className="flex gap-2 items-center">
              <div style={{ flex: 1, display: 'flex', gap: '0.5rem', flexDirection: 'column' }}>
                <input 
                  type="text" 
                  placeholder="Rule Name (e.g. wrong_parking)"
                  value={br.key} 
                  onChange={e => {
                    const newBase = [...baseRules];
                    newBase[idx].key = e.target.value;
                    setBaseRules(newBase);
                  }} 
                  style={{ marginBottom: 0, padding: '0.5rem' }}
                />
                <input 
                  type="text" 
                  value={br.value === 0 ? '' : new Intl.NumberFormat('id-ID').format(br.value)} 
                  placeholder="0"
                  onChange={e => {
                    const rawValue = e.target.value.replace(/\D/g, '');
                    const newBase = [...baseRules];
                    newBase[idx].value = rawValue ? parseInt(rawValue, 10) : 0;
                    setBaseRules(newBase);
                  }} 
                  style={{ marginBottom: 0, padding: '0.5rem' }}
                />
              </div>
              <button className="danger outline" onClick={() => {
                const newBase = [...baseRules];
                newBase.splice(idx, 1);
                setBaseRules(newBase);
              }} style={{ padding: '0.5rem' }}>X</button>
            </div>
          ))}
        </div>
      </div>

      <div className="mb-4">
        <div className="flex justify-between items-center mb-2">
          <h3>Time Multipliers</h3>
          <button className="outline" onClick={() => setTimeRules([...timeRules, {start: "00:00", end: "23:59", multiplier: 1.0}])} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8rem' }}>+ Add</button>
        </div>
        {timeRules.map((tm, idx) => (
          <div key={idx} className="flex gap-2 items-center mb-2">
            <input type="time" value={tm.start} onChange={e => {
              const newTime = [...timeRules]; newTime[idx].start = e.target.value; setTimeRules(newTime);
            }} style={{ marginBottom: 0 }} />
            <span className="text-muted">to</span>
            <input type="time" value={tm.end} onChange={e => {
               const newTime = [...timeRules]; newTime[idx].end = e.target.value; setTimeRules(newTime);
            }} style={{ marginBottom: 0 }} />
            <input type="number" step="0.1" value={tm.multiplier} onChange={e => {
               let val = e.target.value.replace(/^0+(?=\d)/, '');
               if (val.startsWith('.')) val = '0' + val;
               if (e.target.value !== val) e.target.value = val;
               const newTime = [...timeRules]; 
               newTime[idx].multiplier = parseFloat(val) || 0; 
               setTimeRules(newTime);
            }} style={{ marginBottom: 0, width: '100px' }} />
            <button className="danger outline" onClick={() => {
              const newTime = [...timeRules]; newTime.splice(idx, 1); setTimeRules(newTime);
            }} style={{ padding: '0.5rem' }}>X</button>
          </div>
        ))}
      </div>

      <div className="mb-4">
        <div className="flex justify-between items-center mb-2">
          <h3>Past Repetition Multipliers</h3>
          <button className="outline" onClick={() => setRepeatRules([...repeatRules, {prior_unpaid: 0, multiplier: 1.0}])} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8rem' }}>+ Add</button>
        </div>
        {repeatRules.map((rm, idx) => (
          <div key={idx} className="flex gap-2 items-center mb-2">
            <span className="text-muted">Prior Unpaid &gt;=</span>
            <input type="number" value={rm.prior_unpaid} onChange={e => {
              const val = e.target.value.replace(/^0+(?=\d)/, '');
              if (e.target.value !== val) e.target.value = val;
              const newRep = [...repeatRules]; 
              newRep[idx].prior_unpaid = parseInt(val) || 0; 
              setRepeatRules(newRep);
            }} style={{ marginBottom: 0, width: '100px' }} />
            <span className="text-muted">Multiplier:</span>
            <input type="number" step="0.1" value={rm.multiplier} onChange={e => {
              let val = e.target.value.replace(/^0+(?=\d)/, '');
              if (val.startsWith('.')) val = '0' + val;
              if (e.target.value !== val) e.target.value = val;
              const newRep = [...repeatRules]; 
              newRep[idx].multiplier = parseFloat(val) || 0; 
              setRepeatRules(newRep);
            }} style={{ marginBottom: 0, width: '100px' }} />
            <button className="danger outline" onClick={() => {
              const newRep = [...repeatRules]; newRep.splice(idx, 1); setRepeatRules(newRep);
            }} style={{ padding: '0.5rem' }}>X</button>
          </div>
        ))}
      </div>

      <button onClick={handleUpdate} disabled={loading} style={{ width: '100%' }}>
        {loading ? 'Publishing...' : 'Publish New Rule Version'}
      </button>
    </div>
  );
}
