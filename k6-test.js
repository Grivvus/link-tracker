import http from 'k6/http';
import { Counter } from 'k6/metrics';
import { check, sleep } from 'k6';

const BASE_URL = 'http://0.0.0.0:8001';
const CPU_CORES = 12;
const VUS = CPU_CORES * 2;

const READ_RATIO = 100;
const WRITE_RATIO = 1;
const WRITE_PROBABILITY = WRITE_RATIO / (READ_RATIO + WRITE_RATIO);

export const status200 = new Counter('status_200');
export const status500 = new Counter('status_500');
export const status502504 = new Counter('status_502_504');

export const readRequests = new Counter('read_requests');
export const writeRequests = new Counter('write_requests');

export const options = {
  scenarios: {
    main: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: VUS },
        { duration: '10m', target: VUS },
      ],
    },
  },

  summaryTrendStats: ['avg', 'med', 'p(50)', 'p(95)', 'p(99)'],

  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(99)<2000'],
  },
};

export default function () {
  if (Math.random() < WRITE_PROBABILITY) {
    addTrackedLink();
  } else {
    getLinks();
  }

  sleep(1);
}

function getTgChatId() {
  return String(Math.floor(Math.random() * 100)); // 0..99
}

function getLinks() {
  const endpoint = 'GET /links';

  const res = http.get(`${BASE_URL}/links`, {
    headers: {
      'Tg-Chat-Id': getTgChatId(),
    },
    tags: {
      endpoint,
      operation: 'read',
    },
  });

  readRequests.add(1, { endpoint });
  collectStatusMetrics(res, endpoint);

  check(res, {
    'GET /links status is 200': (r) => r.status === 200,
  });
}

function addTrackedLink() {
  const endpoint = 'POST /links';
  const url = `https://github.com/test-${__VU}-${__ITER}/${Date.now()}`;

  const res = http.post(
    `${BASE_URL}/links`,
    JSON.stringify({
      link: url,
      tags: [],
    }),
    {
      headers: {
        'Content-Type': 'application/json',
        'Tg-Chat-Id': getTgChatId(),
      },
      tags: {
        endpoint,
        operation: 'write',
      },
    },
  );

  writeRequests.add(1, { endpoint });
  collectStatusMetrics(res, endpoint);

  check(res, {
    'POST /links status is 200 or 201': (r) => r.status === 200 || r.status === 201,
  });
}

function collectStatusMetrics(res, endpoint) {
  if (res.status === 200) {
    status200.add(1, { endpoint });
  }

  if (res.status === 500) {
    status500.add(1, { endpoint });
  }

  if (res.status === 502 || res.status === 504) {
    status502504.add(1, { endpoint });
  }
}

export function handleSummary(data) {
  return {
    stdout: textSummary(data),
    '/scripts/summary.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  const metrics = data.metrics;

  return `
Load test summary

Configuration:
- CPU cores: ${CPU_CORES}
- VUs: ${VUS}
- Ramp up duration: 1m
- Stage duration: 5m
- Operation ratio read:write = ${READ_RATIO}:${WRITE_RATIO}
- Tg-Chat-Id range: 0..99
- Base URL: ${BASE_URL}

HTTP:
- Total requests: ${metricCount(metrics.http_reqs)}
- Failed request rate: ${metricRate(metrics.http_req_failed)}
- Avg request duration, ms: ${metricValue(metrics.http_req_duration, 'avg')}
- p50 request duration, ms: ${metricValue(metrics.http_req_duration, 'p(50)') || metricValue(metrics.http_req_duration, 'med')}
- p99 request duration, ms: ${metricValue(metrics.http_req_duration, 'p(99)')}

Status codes:
- 200: ${metricCount(metrics.status_200)}
- 500: ${metricCount(metrics.status_500)}
- 502/504: ${metricCount(metrics.status_502504)}

Operations:
- Read requests: ${metricCount(metrics.read_requests)}
- Write requests: ${metricCount(metrics.write_requests)}
`;
}

function metricCount(metric) {
  return metric?.values?.count ?? 0;
}

function metricRate(metric) {
  return metric?.values?.rate ?? 'n/a';
}

function metricValue(metric, key) {
  return metric?.values?.[key] ?? 'n/a';
}
