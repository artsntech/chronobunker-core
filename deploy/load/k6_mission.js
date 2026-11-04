import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '20s', target: 10 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.05'],
  },
};

const TOKEN = __ENV.ACME_TOKEN;

export default function () {
  const payload = JSON.stringify({
    query: `mutation Invoke($input: InvokeAgentInput!) {
      invokeAgent(input: $input) { completion modelId }
    }`,
    variables: {
      input: {
        agentId: '5ca2ccb5-061f-4042-9222-9c076776a2e3',
        prompt: 'where is my order ORD-1234?',
        sessionId: `k6-${__VU}-${__ITER}`,
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
    'no graphql errors': (r) => {
      try { return !JSON.parse(r.body).errors; } catch { return false; }
    },
  });

  sleep(0.1);
}