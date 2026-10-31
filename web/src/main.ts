import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import './style.css';

// Track start timestamp from index.html using high-res performance timer
const startTime = (window as unknown as { __appStartTime?: number }).__appStartTime ?? performance.now();
const MINIMUM_LOADING_MS = 2000;

const app = createApp(App);
app.use(router);

function dismissLoader() {
  const loader = document.getElementById('initial-loader');
  if (loader && loader.style.opacity !== '0') {
    loader.style.opacity = '0';
    loader.style.pointerEvents = 'none';
    setTimeout(() => {
      loader.remove();
    }, 400);
  }
}

// Ensure router is ready and enforce minimum display time of 2s
router.isReady().then(() => {
  app.mount('#app');

  const elapsed = performance.now() - startTime;
  const remaining = Math.max(0, MINIMUM_LOADING_MS - elapsed);

  setTimeout(dismissLoader, remaining);
}).catch((err) => {
  console.error('Router initialization error:', err);
  app.mount('#app');
  setTimeout(dismissLoader, MINIMUM_LOADING_MS);
});
