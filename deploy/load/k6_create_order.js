import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    steady: {
      executor: 'constant-arrival-rate',
      rate: 50, timeUnit: '1s', duration: '30s',
      preAllocatedVUs: 20, maxVUs: 50,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

const TOKEN = __ENV.ACME_TOKEN;

export default function () {
  const payload = JSON.stringify({
    query: `mutation CreateOrder($input: CreateOrderInput!) {
      createOrder(input: $input) { id product totalUsd status }
    }`,
    variables: {
      input: {
        product: 'Bunker Pro Plan',
        quantity: 1,
      },
    },
  });

  const res = http.post('http://localhost:8080/query', payload, {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${TOKEN}`,
    },
  });

  check(res, {
    'status 200': (r) => r.status === 200,
    'no errors': (r) => {
      try { return !JSON.parse(r.body).errors; } catch { return false; }
    },
  });

  sleep(0.1);
}