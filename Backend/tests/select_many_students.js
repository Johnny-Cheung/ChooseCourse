import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:9000/api/v1';
const COURSE_ID = __ENV.COURSE_ID;
const COURSE_IDS = (__ENV.COURSE_IDS || '')
  .split(',')
  .map((value) => value.trim())
  .filter((value) => value !== '');
const USERS = Number(__ENV.USERS || 500);
const START_INDEX = Number(__ENV.START_INDEX || 1);
const STUDENT_PREFIX = __ENV.STUDENT_PREFIX || '2099';
const STUDENT_PAD_WIDTH = Number(__ENV.STUDENT_PAD_WIDTH || 4);
const PASSWORD = __ENV.PASSWORD || '123456';
const DEBUG = __ENV.DEBUG === '1';
const SPREAD_MS = Number(__ENV.SPREAD_MS || 0);
const LOGIN_BATCH_SIZE = Number(__ENV.LOGIN_BATCH_SIZE || 100);
const SETUP_TIMEOUT = __ENV.SETUP_TIMEOUT || '15m';
const NO_CONNECTION_REUSE = __ENV.NO_CONNECTION_REUSE === '1';
const NO_VU_CONNECTION_REUSE = __ENV.NO_VU_CONNECTION_REUSE === '1';
const VU_RAMP_DURATION = __ENV.VU_RAMP_DURATION || '';
const HOLD_AFTER_SUBMIT_SECONDS = Number(__ENV.HOLD_AFTER_SUBMIT_SECONDS || 60);
const PRINT_SELECT_PHASE_SUMMARY = __ENV.PRINT_SELECT_PHASE_SUMMARY !== '0';

let hasSubmitted = false;

function buildScenarios() {
  if (VU_RAMP_DURATION !== '') {
    return {
      select_once: {
        executor: 'ramping-vus',
        startVUs: 0,
        stages: [
          { duration: VU_RAMP_DURATION, target: USERS },
          { duration: '1s', target: USERS },
          { duration: '1s', target: 0 },
        ],
        gracefulRampDown: '0s',
      },
    };
  }

  return {
    select_once: {
      executor: 'per-vu-iterations',
      vus: USERS,
      iterations: 1,
      maxDuration: '10m',
    },
  };
}

export const options = {
  noConnectionReuse: NO_CONNECTION_REUSE,
  noVUConnectionReuse: NO_VU_CONNECTION_REUSE,
  setupTimeout: SETUP_TIMEOUT,
  scenarios: buildScenarios(),
};

function buildStudent(index) {
  return {
    student_no: `${STUDENT_PREFIX}${String(index).padStart(STUDENT_PAD_WIDTH, '0')}`,
    password: PASSWORD,
  };
}

function buildCourseTargets() {
  if (COURSE_IDS.length > 0) {
    return COURSE_IDS;
  }

  if (COURSE_ID) {
    return [String(COURSE_ID)];
  }

  return [];
}

const COURSE_TARGETS = buildCourseTargets();

function courseIdForVu(vu) {
  if (COURSE_TARGETS.length === 0) {
    throw new Error('no course targets configured');
  }

  return COURSE_TARGETS[(vu - 1) % COURSE_TARGETS.length];
}

export function setup() {
  if (COURSE_TARGETS.length === 0) {
    throw new Error('COURSE_ID or COURSE_IDS is required, for example: k6 run -e COURSE_ID=12 tests/select_many_students.js');
  }

  const tokens = [];
  const batchSize = Math.max(1, LOGIN_BATCH_SIZE);

  for (let offset = 0; offset < USERS; offset += batchSize) {
    const batchRequests = [];
    const batchStudents = [];

    for (let i = offset; i < Math.min(offset + batchSize, USERS); i++) {
      const student = buildStudent(START_INDEX + i);
      batchStudents.push(student);
      batchRequests.push([
        'POST',
        `${BASE_URL}/auth/student/login`,
        JSON.stringify(student),
        {
          headers: { 'Content-Type': 'application/json' },
          tags: { name: 'student_login' },
        },
      ]);
    }

    const responses = http.batch(batchRequests);

    for (let i = 0; i < responses.length; i++) {
      const student = batchStudents[i];
      const res = responses[i];

      check(res, {
        'login http status is 200': (r) => r.status === 200,
        'login business code is 0': (r) => r.json('code') === 0,
      });

      if (res.status !== 200 || res.json('code') !== 0) {
        throw new Error(`login failed for ${student.student_no}: ${res.body}`);
      }

      tokens.push(res.json('data.access_token'));
    }
  }

  return {
    tokens,
    selectPhaseStartedAtMs: Date.now(),
    expectedSelectSubmissions: USERS,
  };
}

export default function (data) {
  if (VU_RAMP_DURATION !== '' && hasSubmitted) {
    sleep(HOLD_AFTER_SUBMIT_SECONDS);
    return;
  }

  // On Windows localhost, hundreds of VUs opening connections at the exact same
  // millisecond can overflow the local accept queue before the app logic even runs.
  // SPREAD_MS lets us distribute the single request from each VU across a short window.
  if (SPREAD_MS > 0 && USERS > 1) {
    const delayMs = ((__VU - 1) * SPREAD_MS) / (USERS - 1);
    sleep(delayMs / 1000);
  }

  const token = data.tokens[__VU - 1];
  const courseID = courseIdForVu(__VU);

  if (VU_RAMP_DURATION !== '') {
    hasSubmitted = true;
  }

  const res = http.post(
    `${BASE_URL}/student/courses/${courseID}/selections`,
    null,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
      tags: {
        name: 'select_course',
        course_id: String(courseID),
      },
      responseCallback: http.expectedStatuses(200, 400, 409),
    }
  );

  check(res, {
    'submit returns expected http status': (r) => [200, 400, 409].includes(r.status),
  });

  if (DEBUG) {
    console.log(`vu=${__VU} course_id=${courseID} status=${res.status} body=${res.body}`);
  }

  if (VU_RAMP_DURATION !== '') {
    sleep(HOLD_AFTER_SUBMIT_SECONDS);
  }
}

export function teardown(data) {
  if (!PRINT_SELECT_PHASE_SUMMARY || !data || !data.selectPhaseStartedAtMs) {
    return;
  }

  const durationMs = Date.now() - data.selectPhaseStartedAtMs;
  const durationSeconds = durationMs / 1000;
  const submissions = Number(data.expectedSelectSubmissions || USERS);
  const qps = durationSeconds > 0 ? submissions / durationSeconds : 0;

  console.log(
    `[select_phase_summary] submissions=${submissions} duration=${durationSeconds.toFixed(3)}s qps=${qps.toFixed(3)}/s`
  );
}
