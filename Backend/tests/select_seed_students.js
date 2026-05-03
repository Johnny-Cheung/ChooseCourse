import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8080/api/v1';
const COURSE_ID = __ENV.COURSE_ID;

const students = [
  { student_no: '20230001', password: '123456' },
  { student_no: '20230002', password: '123456' },
  { student_no: '20230003', password: '123456' },
];

export const options = {
  scenarios: {
    select_once: {
      executor: 'per-vu-iterations',
      vus: students.length,
      iterations: 1,
      maxDuration: '1m',
    },
  },
};

export function setup() {
  if (!COURSE_ID) {
    throw new Error('COURSE_ID is required, for example: k6 run -e COURSE_ID=12 tests/select_seed_students.js');
  }

  const tokens = [];

  for (const student of students) {
    const res = http.post(
      `${BASE_URL}/auth/student/login`,
      JSON.stringify(student),
      {
        headers: { 'Content-Type': 'application/json' },
        tags: { name: 'student_login' },
      }
    );

    check(res, {
      'login http status is 200': (r) => r.status === 200,
      'login business code is 0': (r) => r.json('code') === 0,
    });

    if (res.status !== 200 || res.json('code') !== 0) {
      throw new Error(`login failed for ${student.student_no}: ${res.body}`);
    }

    tokens.push(res.json('data.access_token'));
  }

  return { tokens };
}

export default function (data) {
  const token = data.tokens[__VU - 1];

  const res = http.post(
    `${BASE_URL}/student/courses/${COURSE_ID}/selections`,
    null,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
      responseCallback: http.expectedStatuses(200, 400, 409),
      tags: { name: 'select_course' },
    }
  );

  check(res, {
    'submit returns expected http status': (r) => [200, 400, 409].includes(r.status),
  });

  console.log(`vu=${__VU} status=${res.status} body=${res.body}`);
}
