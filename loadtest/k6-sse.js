import { check, sleep } from 'k6';
import http from 'k6/http';
import { Gauge } from 'k6/metrics';

/**
 * MizuFlow Admin Trigger Script
 * ----------------------------
 * This script strictly handles triggering Admin POST updates.
 * The Go Load Generator (main.go) handles the 2000 SSE connections and latency measurement.
 */

// --- Configuration ---
const BASE_URL = 'http://localhost:8080';
const TARGET_ENV = 'dev';
const TARGET_NS = 'default';
const FEATURE_KEY = 'loadtest-latency-check'; 
const ADMIN_USER = 'admin';  
const ADMIN_PASS = 'admin123'; 

// --- Metrics ---
const serverHeapAlloc = new Gauge('server_heap_alloc_bytes');
const serverGoroutines = new Gauge('server_goroutines');

// --- Test Scenarios ---
export const options = {
    scenarios: {
        // Admin Broadcasts Updates
        admin: {
            executor: 'constant-arrival-rate',
            rate: 1, // 1 update every ...
            timeUnit: '2s', // ... 2 seconds (faster updates)
            duration: '10m', 
            preAllocatedVUs: 1,
            maxVUs: 1,
            exec: 'admin',
        },
        // Monitor Server Metrics
        monitor: {
            executor: 'constant-vus',
            vus: 1,
            duration: '10m',
            exec: 'monitor',
        }
    }
};

// --- Setup: Get Admin Token ---
export function setup() {
    const loginUrl = `${BASE_URL}/v1/auth/login`;
    const res = http.post(loginUrl, JSON.stringify({
        username: ADMIN_USER,
        password: ADMIN_PASS
    }), { headers: { 'Content-Type': 'application/json' } });

    if (res.status !== 200) {
        throw new Error(`Failed to login: ${res.body}`);
    }
    return { token: res.json('access_token') };
}

// --- Logic: Admin Update ---
export function admin(data) {
    const token = data.token;
    
    // We send current timestamp as the value
    const payload = {
        namespace: TARGET_NS,
        env: TARGET_ENV,
        key: FEATURE_KEY,
        value: Date.now().toString(), // <--- Timestamp as value for Go client to measure
        type: 'string' 
    };

    const url = `${BASE_URL}/v1/feature`;
    const res = http.post(url, JSON.stringify(payload), {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        }
    });

    check(res, {
        'admin update 200': (r) => r.status === 200,
    });
}

// --- Logic: Server Monitor ---
export function monitor() {
    const url = `${BASE_URL}/metrics`;
    const res = http.get(url);
    if (res.status === 200) {
        const body = res.body;
        
        const heapMatch = body.match(/go_memstats_heap_alloc_bytes ([0-9.e+]+)/);
        if (heapMatch) {
            serverHeapAlloc.set(parseFloat(heapMatch[1]));
        }

        const goroutineMatch = body.match(/go_goroutines ([0-9]+)/);
        if (goroutineMatch) {
            serverGoroutines.set(parseInt(goroutineMatch[1]));
        }
    }
    sleep(5);
}
