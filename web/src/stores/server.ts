import { reactive } from 'vue';
import type { ConfigResponse, GlobalStats, SessionMetric } from '../types';

export const serverState = reactive({
  endpoint: localStorage.getItem('ag_endpoint') || 'http://localhost:58763',
  token: localStorage.getItem('ag_token') || '',
  isConnected: false,
  authRequired: false,
  serverVersion: '',
  dataDir: '',
  stats: null as GlobalStats | null,
  loading: false,
  error: '',
});

function getCleanEndpoint(ep?: string): string {
  const target = (ep || serverState.endpoint || 'http://localhost:58763').trim();
  return target.replace(/\/+$/, '');
}

/**
 * Xoá sạch toàn bộ dữ liệu xác thực, cache stats và cờ kết nối khi gặp lỗi hoặc 401
 */
export function clearAuthAndData(errorMessage?: string): void {
  serverState.isConnected = false;
  serverState.stats = null;
  serverState.token = '';
  if (errorMessage !== undefined) {
    serverState.error = errorMessage;
  }
  if (typeof localStorage !== 'undefined') {
    localStorage.removeItem('ag_token');
    localStorage.removeItem('ag_connected');
  }
}

export async function testConnection(
  targetEp?: string,
  targetTok?: string
): Promise<{ success: boolean; authRequired?: boolean; error?: string }> {
  const cleanEp = getCleanEndpoint(targetEp);
  const token = targetTok !== undefined ? targetTok.trim() : serverState.token;
  serverState.error = '';

  try {
    const res = await fetch(`${cleanEp}/api/config`, {
      headers: { Accept: 'application/json' },
    });

    if (!res.ok) {
      const errMsg = `Mã phản hồi HTTP ${res.status}`;
      clearAuthAndData(errMsg);
      return { success: false, error: errMsg };
    }

    const cfg: ConfigResponse = await res.json();
    serverState.serverVersion = cfg.version || '1.1.0';
    serverState.authRequired = !!cfg.auth_required;
    serverState.dataDir = cfg.data_dir || '';

    // If server requires authentication, verify the token
    if (cfg.auth_required) {
      if (!token) {
        const errMsg = 'Máy chủ yêu cầu Token/API Key xác thực (401). Vui lòng nhập token.';
        clearAuthAndData(errMsg);
        return {
          success: false,
          authRequired: true,
          error: errMsg,
        };
      }

      // Test token with /api/stats
      const statsRes = await fetch(`${cleanEp}/api/stats`, {
        headers: {
          Accept: 'application/json',
          Authorization: `Bearer ${token}`,
          'X-API-Key': token,
        },
      });

      if (statsRes.status === 401) {
        const errMsg = 'Token xác thực không chính xác (401 Unauthorized)! Đã xoá sạch dữ liệu.';
        clearAuthAndData(errMsg);
        return { success: false, authRequired: true, error: errMsg };
      }

      if (!statsRes.ok) {
        const errMsg = `Lỗi kiểm tra quyền hạn: HTTP ${statsRes.status}`;
        clearAuthAndData(errMsg);
        return { success: false, authRequired: true, error: errMsg };
      }
    }

    // Success! Update state and localStorage
    serverState.endpoint = cleanEp;
    serverState.token = token;
    serverState.isConnected = true;
    serverState.error = '';

    localStorage.setItem('ag_endpoint', cleanEp);
    localStorage.setItem('ag_token', token);
    localStorage.setItem('ag_connected', 'true');

    return { success: true, authRequired: cfg.auth_required };
  } catch (err: any) {
    const errMsg = `Không thể kết nối đến máy chủ: ${cleanEp}. Hãy đảm bảo Go server đang chạy lệnh 'ag-stats serve'.`;
    clearAuthAndData(errMsg);
    return { success: false, error: errMsg };
  }
}

export async function fetchStats(_force = false): Promise<boolean> {
  if (!serverState.endpoint) return false;
  serverState.loading = true;

  try {
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (serverState.token) {
      headers['Authorization'] = `Bearer ${serverState.token}`;
      headers['X-API-Key'] = serverState.token;
    }

    const res = await fetch(`${serverState.endpoint}/api/stats`, { headers });
    if (!res.ok) {
      if (res.status === 401) {
        clearAuthAndData('Phiên làm việc hết hạn hoặc token không hợp lệ (401)! Đã xoá sạch dữ liệu.');
      } else {
        serverState.error = `Lỗi từ máy chủ: HTTP ${res.status}`;
      }
      return false;
    }

    serverState.stats = await res.json();
    return true;
  } catch (err) {
    console.error('Failed to fetch stats:', err);
    return false;
  } finally {
    serverState.loading = false;
  }
}

export async function fetchSessionDetail(id: string): Promise<SessionMetric | null> {
  if (!serverState.endpoint || !id) return null;

  try {
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (serverState.token) {
      headers['Authorization'] = `Bearer ${serverState.token}`;
      headers['X-API-Key'] = serverState.token;
    }

    const res = await fetch(`${serverState.endpoint}/api/sessions/${encodeURIComponent(id)}`, { headers });
    if (!res.ok) {
      if (res.status === 401) {
        clearAuthAndData('Phiên làm việc hết hạn hoặc token không hợp lệ (401)! Đã xoá sạch dữ liệu.');
      }
      return null;
    }
    return await res.json();
  } catch (err) {
    console.error('Failed to fetch session detail:', err);
    return null;
  }
}

export function disconnect(): void {
  clearAuthAndData();
}
