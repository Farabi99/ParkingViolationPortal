import { useState, useEffect } from 'react';

import ExifReader from 'exifreader';

export default function OfficerSubmit() {
  const [plate, setPlate] = useState('');
  const [type, setType] = useState('expired_meter');
  const [location, setLocation] = useState('');
  const [photo, setPhoto] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [captureTimestamp, setCaptureTimestamp] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<{text: string, type: 'success' | 'danger'} | null>(null);
  const [activeRules, setActiveRules] = useState<Record<string, any> | null>(null);
  const [resetKey, setResetKey] = useState(0);
  


  useEffect(() => {
    fetch('/api/rules/active')
      .then(res => res.json())
      .then(data => {
        setActiveRules(data.rules);
        const keys = Object.keys(data.rules.base_amount);
        if (keys.length > 0 && !keys.includes(type)) {
          setType(keys[0]);
        }
      })
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handlePhotoChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] || null;
    setPhoto(file);
    if (file) {
      setPreviewUrl(URL.createObjectURL(file));

      try {
        const arrayBuffer = await file.arrayBuffer();
        const tags = ExifReader.load(arrayBuffer);
        if (tags['DateTimeOriginal'] && tags['DateTimeOriginal'].description) {
          const dateStr = tags['DateTimeOriginal'].description.replace(/\0/g, '').trim();
          const parts = dateStr.split(' ');
          if (parts.length === 2) {
             const datePart = parts[0].replace(/:/g, '-');
             const timePart = parts[1];
             const isoString = `${datePart}T${timePart}`;
             
             let offset = '';
             if (tags['OffsetTimeOriginal'] && tags['OffsetTimeOriginal'].description) {
                 offset = tags['OffsetTimeOriginal'].description.replace(/\0/g, '').trim();
             }
             
             if (offset) {
                 setCaptureTimestamp(new Date(isoString + offset).toISOString());
             } else {
                 setCaptureTimestamp(new Date(isoString).toISOString());
             }
             return;
          }
        }
      } catch {
        // Silently ignore EXIF parsing errors and fallback to file time
      }
      
      // Fallback to the file's last modified time if EXIF is unavailable
      setCaptureTimestamp(new Date(file.lastModified).toISOString());
    } else {
      setCaptureTimestamp(null);
      setPreviewUrl(null);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setMessage(null);

    const formData = new FormData();
    formData.append('license_plate', plate);
    formData.append('type', type);
    formData.append('location', location);
    if (photo) formData.append('photo', photo);
    if (captureTimestamp) {
      formData.append('timestamp', new Date(captureTimestamp).toISOString());
    }

    try {
      const res = await fetch('/api/violations', {
        method: 'POST',
        body: formData
      });

      if (!res.ok) {
        let errorMsg = 'Submission failed';
        try {
          const errorData = await res.text();
          if (errorData) errorMsg += `: ${errorData}`;
        } catch { /* ignore */ }
        throw new Error(errorMsg);
      }
      const data = await res.json();
      
      setMessage({ text: `Violation submitted successfully at ${new Date(data.timestamp).toLocaleString()}!`, type: 'success' });
      setPlate(''); 
      setLocation(''); 
      setPhoto(null);
      setPreviewUrl(null);
      setCaptureTimestamp(null);
      setResetKey(prev => prev + 1);
    } catch (err: unknown) {
      const e = err as Error;
      setMessage({ text: e.message || 'Failed to submit violation.', type: 'danger' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="glass-panel" style={{ maxWidth: '600px', margin: '0 auto' }}>
      <h2>Submit Violation</h2>
      {message && <div className={`badge ${message.type} mb-4`} style={{ display: 'inline-block' }}>{message.text}</div>}
      <form onSubmit={handleSubmit}>
        <div className="mb-2">
          <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>License Plate</label>
          <input type="text" value={plate} onChange={e => setPlate(e.target.value)} required />
        </div>
        <div className="mb-2">
          <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>Violation Type</label>
          <select value={type} onChange={e => setType(e.target.value)}>
            {activeRules && Object.keys(activeRules.base_amount).map(k => (
              <option key={k} value={k}>{k.replace(/_/g, ' ')}</option>
            ))}
            {!activeRules && <option value="expired_meter">Expired Meter</option>}
          </select>
        </div>
        <div className="mb-2">
          <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>Location</label>
          <input type="text" value={location} onChange={e => setLocation(e.target.value)} required />
        </div>
        <div className="mb-4">
          <label className="text-muted mb-1" style={{ display: 'block', fontSize: '0.875rem' }}>Photo Evidence</label>
          <input key={resetKey} type="file" accept="image/*" onChange={handlePhotoChange} required />
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>Max file size: 10MB</div>
        </div>
        
        {previewUrl && captureTimestamp && (
          <div className="mb-4" style={{ padding: '1rem', backgroundColor: 'var(--surface-hover)', borderRadius: '8px' }}>
            <p className="mb-2" style={{ fontWeight: 600, fontSize: '0.9rem' }}>
              📸 Captured at: <span style={{ color: 'var(--primary)' }}>{new Date(captureTimestamp).toLocaleString()}</span>
            </p>
            <img src={previewUrl} alt="Preview" style={{ width: '100%', maxHeight: '300px', objectFit: 'contain', borderRadius: '4px', border: '1px solid var(--border)' }} />
          </div>
        )}

        <button type="submit" disabled={loading} style={{ width: '100%' }}>
          {loading ? 'Submitting...' : 'Submit Violation'}
        </button>
      </form>
    </div>
  );
}
