import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';

const healthyAndUnhealthyResponse = {
  checkedAt: '2026-10-21T10:30:00Z',
  services: [
    { name: 'user-service', status: 'UP' as const, details: '' },
    { name: 'order-service', status: 'DOWN' as const, details: 'Timeout' },
    { name: 'payment-service', status: 'UP' as const, details: '' },
  ],
};

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  } as Response;
}

describe('service health dashboard', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(healthyAndUnhealthyResponse)));
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('loads health results and displays status, failure details, and summary counts', async () => {
    render(<App />);

    expect(await screen.findByText('user-service')).toBeInTheDocument();
    expect(screen.getByText('order-service')).toBeInTheDocument();
    expect(screen.getByText('payment-service')).toBeInTheDocument();
    expect(screen.getByText('Timeout')).toBeInTheDocument();
    const summary = within(screen.getByRole('region', { name: 'Health summary' }));
    expect(summary.getByText('Healthy')).toBeInTheDocument();
    expect(summary.getByText('2')).toBeInTheDocument();
    expect(summary.getByText('1')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/health');
    expect(screen.queryByText(/ms/)).not.toBeInTheDocument();
  });

  it('refreshes health results when requested', async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock
      .mockResolvedValueOnce(jsonResponse(healthyAndUnhealthyResponse))
      .mockResolvedValueOnce(jsonResponse({
        checkedAt: '2026-10-21T10:31:00Z',
        services: [{ name: 'user-service', status: 'DOWN', details: 'HTTP 503' }],
      }));

    render(<App />);
    expect(await screen.findByText('order-service')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /refresh/i }));

    expect(await screen.findByText('HTTP 503')).toBeInTheDocument();
    expect(screen.queryByText('order-service')).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('shows a retryable error when the API returns an error', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({}, 503));
    render(<App />);

    expect(await screen.findByRole('alert')).toHaveTextContent('Backend unavailable');
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });

  it('shows an error when the API request rejects', async () => {
    vi.mocked(fetch).mockRejectedValue(new Error('network unavailable'));
    render(<App />);

    expect(await screen.findByRole('alert')).toHaveTextContent('Unable to reach the health-check backend');
  });

  it('retries the request from the backend error panel', async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock
      .mockRejectedValueOnce(new Error('network unavailable'))
      .mockResolvedValueOnce(jsonResponse(healthyAndUnhealthyResponse));
    render(<App />);

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /try again/i }));

    expect(await screen.findByText('user-service')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('shows the loading state and disables refresh while a check is in progress', async () => {
    let resolveFetch!: (response: Response) => void;
    vi.mocked(fetch).mockImplementation(() => new Promise((resolve) => {
      resolveFetch = resolve;
    }));
    render(<App />);

    expect(screen.getByRole('status')).toHaveTextContent('Checking configured services');
    expect(screen.getByRole('button', { name: /checking/i })).toBeDisabled();

    resolveFetch(jsonResponse(healthyAndUnhealthyResponse));
    expect(await screen.findByText('user-service')).toBeInTheDocument();
  });

  it('toggles and persists the selected theme', async () => {
    render(<App />);
    await screen.findByText('user-service');

    fireEvent.click(screen.getByRole('button', { name: 'Switch to dark theme' }));

    await waitFor(() => expect(document.documentElement).toHaveAttribute('data-theme', 'dark'));
    expect(localStorage.getItem('service-dashboard-theme')).toBe('dark');
    expect(screen.getByRole('button', { name: 'Switch to light theme' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('renders an empty-state message when there are no configured services', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({ checkedAt: '2026-10-21T10:30:00Z', services: [] }));
    render(<App />);

    expect(await screen.findByText('No service results are available.')).toBeInTheDocument();
  });

  it('keeps each service result in its own table row', async () => {
    render(<App />);
    await screen.findByText('user-service');

    const row = screen.getByText('order-service').closest('tr');
    expect(row).not.toBeNull();
    expect(within(row as HTMLTableRowElement).getByText('DOWN')).toBeInTheDocument();
    expect(within(row as HTMLTableRowElement).getByText('Timeout')).toBeInTheDocument();
  });
});
