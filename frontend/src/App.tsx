
import { useCallback, useEffect, useState } from 'react';
import type { HealthResponse, ServiceResult } from './types';
import './App.css';

const API_URL = '/api/health';

function SunIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M2 12h2m16 0h2M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="M20.2 15.2A8.5 8.5 0 0 1 8.8 3.8 8.5 8.5 0 1 0 20.2 15.2Z" />
    </svg>
  );
}

function ServiceRow({ service }: { service: ServiceResult }) {
  const isHealthy = service.status === 'UP';

  return (
    <tr>
      <td className="service-name">{service.name}</td>
      <td>
        <span
          className={`status-badge ${isHealthy ? 'status-up' : 'status-down'}`}
        >
          <span className="status-dot" aria-hidden="true" />
          {service.status}
        </span>
      </td>
      <td>
        {isHealthy ? 'Healthy' : service.details || 'Unknown failure'}
      </td>
    </tr>
  );
}

export default function App() {
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return localStorage.getItem('service-dashboard-theme') === 'dark'
      ? 'dark'
      : 'light';
  });
  const [services, setServices] = useState<ServiceResult[]>([]);
  const [checkedAt, setCheckedAt] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const refresh = useCallback(async () => {
    setLoading(true);
    setError('');

    try {
      const response = await fetch(API_URL);

      if (!response.ok) {
        throw new Error(`Backend returned HTTP ${response.status}`);
      }

      const data: HealthResponse = await response.json();

      setServices(data.services);
      setCheckedAt(data.checkedAt);
    } catch {
      setError(
        'Unable to reach the health-check backend. Check that it is running and try again.'
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('service-dashboard-theme', theme);
  }, [theme]);

  return (
    <main className="dashboard">
      <button
        className="theme-toggle"
        type="button"
        aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`}
        aria-pressed={theme === 'dark'}
        onClick={() => setTheme((current) => current === 'dark' ? 'light' : 'dark')}
      >
        <span className={`theme-mode-icon ${theme === 'light' ? 'is-active' : ''}`}>
          <SunIcon />
        </span>
        <span className={`theme-switch-track ${theme === 'dark' ? 'is-dark' : ''}`} aria-hidden="true">
          <span className="theme-switch-thumb" />
        </span>
        <span className={`theme-mode-icon ${theme === 'dark' ? 'is-active' : ''}`}>
          <MoonIcon />
        </span>
      </button>

      <header className="dashboard-header">
        <div>
          <p className="eyebrow">MONITORING</p>
          <h1>Service Health Dashboard</h1>
          <p className="subtitle">
            Monitor the availability and health of your services.
          </p>
        </div>

        <button
          className="refresh-button"
          onClick={() => void refresh()}
          disabled={loading}
        >
          <span aria-hidden="true">{loading ? '◌' : '↻'}</span>
          {loading ? 'Checking…' : 'Refresh'}
        </button>
      </header>

      {error ? (
        <section className="error-panel" role="alert">
          <strong>Backend unavailable</strong>
          <p>{error}</p>
          <button onClick={() => void refresh()} disabled={loading}>
            Try again
          </button>
        </section>
      ) : (
        <>
          <section className="summary" aria-label="Health summary">
            <div className="summary-card">
              <span>Total services</span>
              <strong>{services.length}</strong>
            </div>
            <div className="summary-card">
              <span>Healthy</span>
              <strong className="healthy-count">
                {services.filter((s) => s.status === 'UP').length}
              </strong>
            </div>
            <div className="summary-card">
              <span>Unhealthy</span>
              <strong className="unhealthy-count">
                {services.filter((s) => s.status === 'DOWN').length}
              </strong>
            </div>
          </section>

          <section className="services-panel">
            <div className="panel-header">
              <div>
                <h2>Service Status</h2>
                <p>Live results from the latest health check</p>
              </div>
              <span className="last-checked">
                {checkedAt
                  ? `Last checked ${new Date(checkedAt).toLocaleTimeString()}`
                  : 'Waiting for results'}
              </span>
            </div>

            {loading && services.length === 0 ? (
              <p className="empty-state" role="status">
                Checking configured services…
              </p>
            ) : services.length === 0 ? (
              <p className="empty-state">
                No service results are available.
              </p>
            ) : (
              <div className="table-container">
                <table>
                  <thead>
                    <tr>
                      <th>Service</th>
                      <th>Status</th>
                      <th>Details</th>
                    </tr>
                  </thead>
                  <tbody>
                    {services.map((service) => (
                      <ServiceRow
                        key={service.name}
                        service={service}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}

      <footer>
        Health checks are performed by the backend. Each request has a
        five-second timeout.
      </footer>
    </main>
  );
}
