import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:9000';

export const options = {
  vus: 1,
  iterations: 1,
};

export default function () {
  const res = http.get(`${BASE_URL}/health`, {
    tags: { name: 'health' },
  });

  check(res, {
    'health status is 200': (r) => r.status === 200,
  });

  console.log(`status=${res.status} body=${res.body}`);
}
