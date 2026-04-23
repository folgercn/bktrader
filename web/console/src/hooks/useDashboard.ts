import { useDashboardRealtime } from './useDashboardRealtime';
import { useDashboardState } from './useDashboardState';
import { useDashboardConfig } from './useDashboardConfig';

export function useDashboard() {
  const { loadRealtime } = useDashboardRealtime();
  const { loadState } = useDashboardState();
  const { loadConfig } = useDashboardConfig();

  async function loadDashboard() {
    // Manually trigger all three if needed (e.g. for forced refresh)
    await Promise.all([
      loadRealtime(),
      loadState(),
      loadConfig()
    ]);
  }

  return { loadDashboard };
}
