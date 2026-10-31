import { createRouter, createWebHistory } from 'vue-router';
import { serverState, testConnection } from '../stores/server';

function getRouterBase(): string {
  // If hosted under a subpath like /antigravity-stats/ on GitHub Pages
  if (typeof window !== 'undefined' && window.location.pathname.startsWith('/antigravity-stats')) {
    return '/antigravity-stats/';
  }
  return '/';
}

const router = createRouter({
  history: createWebHistory(getRouterBase()),
  routes: [
    {
      path: '/',
      name: 'root',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('../views/DashboardView.vue'),
      meta: { requiresConnection: true },
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/login',
    },
  ],
});

let initialReconnectionAttempted = false;

// Navigation Guard
router.beforeEach(async (to, _from, next) => {
  const isPreviouslyConnected =
    typeof localStorage !== 'undefined' && localStorage.getItem('ag_connected') === 'true';

  // 1. On page load / refresh: if not yet connected in memory, but was previously connected
  // (or user lands directly on protected route /dashboard or root /), attempt auto-reconnect
  if (!serverState.isConnected && !initialReconnectionAttempted) {
    initialReconnectionAttempted = true;

    if (isPreviouslyConnected || to.meta.requiresConnection || to.path === '/') {
      const res = await testConnection();
      if (!res.success) {
        localStorage.removeItem('ag_connected');
      }
    }
  }

  // 2. Root route handling
  if (to.path === '/') {
    if (serverState.isConnected) {
      next({ name: 'dashboard' });
    } else {
      next({ name: 'login' });
    }
    return;
  }

  // 3. Protected route handling
  if (to.meta.requiresConnection) {
    if (serverState.isConnected) {
      next();
    } else {
      next({ name: 'login' });
    }
    return;
  }

  next();
});

export default router;
