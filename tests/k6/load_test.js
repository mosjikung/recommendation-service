// tests/k6/load_test.js
// Run: k6 run tests/k6/load_test.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const cacheHitRate = new Rate('cache_hits');

// ── Scenario 1: Single-user load test (100 RPS for 1 minute) ──────────────
export let options = {
  scenarios: {
    single_user_load: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
    batch_stress: {
      executor: 'ramping-vus',
      startTime: '1m10s',   // after single_user_load finishes
      stages: [
        { duration: '20s', target: 10 },
        { duration: '40s', target: 30 },
        { duration: '20s', target: 0 },
      ],
      exec: 'batchTest',
    },
    cache_effectiveness: {
      executor: 'constant-vus',
      vus: 5,
      duration: '30s',
      startTime: '2m30s',   // after batch_stress finishes
      exec: 'cacheTest',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed:   ['rate<0.05'],   // allow up to 5% (incl. simulated 1.5% model failures)
    errors:            ['rate<0.05'],
  },
};

const BASE = 'http://localhost:8080';

// ── Scenario 1: Single-user recommendations ───────────────────────────────
export default function () {
  const userId = Math.floor(Math.random() * 20) + 1;
  const res = http.get(`${BASE}/users/${userId}/recommendations?limit=10`);

  const ok = check(res, {
    'status 200':            (r) => r.status === 200,
    'has recommendations':   (r) => {
      try { return JSON.parse(r.body).recommendations.length > 0; }
      catch { return false; }
    },
  });

  errorRate.add(!ok);

  if (res.status === 200) {
    try {
      const body = JSON.parse(res.body);
      cacheHitRate.add(body.metadata?.cache_hit === true);
    } catch {}
  }

  sleep(0.1);
}

// ── Scenario 2: Batch endpoint stress ─────────────────────────────────────
export function batchTest() {
  const page  = Math.floor(Math.random() * 3) + 1;
  const limit = [5, 10, 20][Math.floor(Math.random() * 3)];
  const res   = http.get(`${BASE}/recommendations/batch?page=${page}&limit=${limit}`);

  check(res, {
    'batch status 200':   (r) => r.status === 200,
    'has results':        (r) => {
      try { return JSON.parse(r.body).results.length > 0; }
      catch { return false; }
    },
    'has summary':        (r) => {
      try { return JSON.parse(r.body).summary !== undefined; }
      catch { return false; }
    },
  });

  sleep(1);
}

// ── Scenario 3: Cache effectiveness ───────────────────────────────────────
// Hit the same 3 users repeatedly — second+ requests should be cache hits.
export function cacheTest() {
  const userId = (Math.floor(Math.random() * 3) + 1); // users 1-3 only
  const res    = http.get(`${BASE}/users/${userId}/recommendations?limit=10`);

  if (res.status === 200) {
    try {
      const body = JSON.parse(res.body);
      cacheHitRate.add(body.metadata?.cache_hit === true);

      check(res, {
        'cache hit after warm-up': (r) => body.metadata?.cache_hit === true,
      });
    } catch {}
  }

  sleep(0.5);
}
