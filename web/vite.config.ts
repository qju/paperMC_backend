import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
    plugins: [react()],
    server: {
        proxy: {
            // Proxy API endpoints (e.g. fetch('/login'))
            '/login': 'http://localhost:8080',
            '/status': 'http://localhost:8080',
            '/start': 'http://localhost:8080',
            '/stop': 'http://localhost:8080',
            '/command': 'http://localhost:8080',
            '/config': 'http://localhost:8080',
            '/whitelist_add': 'http://localhost:8080',
            '/update': 'http://localhost:8080',

            // Proxy WebSocket
            '/ws': {
                target: 'ws://localhost:8080',
                ws: true,
                configure: (proxy) => {
                    proxy.on('error', (err: any) => {
                        if (err.code === 'ECONNRESET' || err.code === 'EPIPE') return;
                        console.error('[vite ws proxy error]', err);
                    });
                    proxy.on('proxyReqWs', (_proxyReq, _req, socket: any) => {
                        socket.on('error', (err: any) => {
                            if (err.code === 'ECONNRESET' || err.code === 'EPIPE') return;
                            console.error('[vite ws socket error]', err);
                        });
                    });
                }
            },
            '/api': {
                target: 'http://localhost:8080',
                changeOrigin: true,
                secure: false,
            },
        }
    }
})
