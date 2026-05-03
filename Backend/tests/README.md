# k6 Scripts

## 1. Health Smoke Test

Start the backend first, then run:

```powershell
k6 run .\tests\health_smoke.js
```

You can override the base URL:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:8080 .\tests\health_smoke.js
```

## 2. Seed Students Course Selection Test

This script logs in the three seeded students and lets each of them submit one
selection request for the same course.

Before running:

1. Start the backend.
2. Create a test course manually and remember the course ID.

Run:

```powershell
k6 run -e COURSE_ID=12 .\tests\select_seed_students.js
```

You can also override the API base URL:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:8080/api/v1 -e COURSE_ID=12 .\tests\select_seed_students.js
```

## 3. 500 Students Grab 50 Seats

### Step 1. Seed 500 stress-test students

Run the SQL in:

```text
tests/seed_stress_students_500.sql
```

It creates students:

```text
20990001 ~ 20990500
```

Password for all of them:

```text
123456
```

If you already ran a previous round and want to reuse the same 500 students,
run the reset SQL first:

```text
tests/reset_stress_students_500.sql
```

This reset script only targets the 500 stress-test students and will:

- delete their enrollments
- delete their selection request history
- delete their notifications
- reset their `credit_used` to `0`
- recalculate `courses.selected_count`, `courses.like_count`, `courses.comment_count`

Important:

- After running the SQL, also clear Redis selection cache before the next round.
- If your Redis logical DB is dedicated to this project, the simplest option is `FLUSHDB`.
- If Redis is shared with other apps, do not `FLUSHDB`; instead delete this app's selection-cache keys or restart with a clean dedicated DB.

### Step 2. Create a fresh test course

Use the admin API to create a new course for this round.

Recommended settings:

```json
{
  "course_name": "k6-500-students",
  "teacher_name": "Load Test Teacher",
  "capacity": 50,
  "time_slot": 13,
  "credit": 2,
  "status": 1
}
```

Remember the returned course ID.

Recommended when re-running:

- always create a brand-new test course
- use a `time_slot` that was not used by the previous round

That avoids old successful enrollments on stress-test students causing time-conflict failures in the new round.

### Step 3. Run the load test

```powershell
k6 run -e BASE_URL=http://127.0.0.1:9000/api/v1 -e COURSE_ID=12 -e USERS=500 .\tests\select_many_students.js
```

Optional debug mode:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:9000/api/v1 -e COURSE_ID=12 -e USERS=500 -e DEBUG=1 .\tests\select_many_students.js
```

Optional smooth-spread mode for localhost / Windows:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:9000/api/v1 -e COURSE_ID=12 -e USERS=500 -e SPREAD_MS=2000 .\tests\select_many_students.js
```

`SPREAD_MS=2000` means the 500 students will submit across a 2-second window instead of all at the exact same millisecond.
This is useful when you want to stress the selection workflow itself, not the local machine's TCP accept queue.

If you are running on Windows and seeing host-port refusal errors under high
concurrency, prefer running `k6` inside Docker on the same Compose network:

```powershell
docker compose --profile tools run --rm k6 run -e BASE_URL=http://backend:8080/api/v1 -e COURSE_ID=12 -e USERS=500 -e SPREAD_MS=2000 /work/tests/select_many_students.js
```

For larger tests, `setup()` now supports:

- `LOGIN_BATCH_SIZE`: how many student logins to perform concurrently in each setup batch
- `SETUP_TIMEOUT`: max allowed setup duration

### Step 4. Verify final state in MySQL

Replace `12` with the real course ID.

1. Check request status summary:

```sql
SELECT status, COUNT(*)
FROM selection_requests
WHERE course_id = 12
GROUP BY status;
```

2. Check the course row:

```sql
SELECT id, capacity, selected_count, status
FROM courses
WHERE id = 12;
```

3. Check the real number of successful enrollments:

```sql
SELECT COUNT(*) AS actual_selected
FROM enrollments
WHERE course_id = 12 AND status = 1;
```

4. Check duplicate successful enrollments:

```sql
SELECT student_id, COUNT(*) AS cnt
FROM enrollments
WHERE course_id = 12 AND status = 1
GROUP BY student_id
HAVING COUNT(*) > 1;
```

Expected result:

- `selected_count = 50`
- `actual_selected = 50`
- duplicate query returns empty
- no oversell

## 4. 1000 Students Grab 100 Seats in 1 Second

### Step 1. Seed 1000 stress-test students

Run:

```text
tests/seed_stress_students_1000.sql
```

It creates students:

```text
20990001 ~ 20991000
```

If you want to reuse the same 1000 students for another round, reset them first:

```text
tests/reset_stress_students_1000.sql
```

### Step 2. Create a fresh 100-seat course

Recommended settings:

```json
{
  "course_name": "k6-1000-students",
  "teacher_name": "Load Test Teacher",
  "capacity": 100,
  "time_slot": 14,
  "credit": 2,
  "status": 1
}
```

### Step 3. Run the load test

To approximate "1000 students submit within 1 second", use:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:9000/api/v1 -e COURSE_ID=12 -e USERS=1000 -e SPREAD_MS=1000 -e LOGIN_BATCH_SIZE=100 -e SETUP_TIMEOUT=15m .\tests\select_many_students.js
```

Interpretation:

- `USERS=1000`: 1000 distinct student accounts
- `SPREAD_MS=1000`: all submissions are spread across a 1-second window

### Step 4. Verify final state

Expected result:

- `selected_count = 100`
- `actual_selected = 100`
- duplicate query returns empty
- no oversell

## 5. 10000 Students Grab 1000 Seats in 1 Second

### Step 1. Seed 10000 stress-test students

Run:

```text
tests/seed_stress_students_10000.sql
```

It creates students:

```text
22000001 ~ 22010000
```

If you want to reuse the same 10000 students for another round, reset them first:

```text
tests/reset_stress_students_10000.sql
```

### Step 2. Create a fresh 1000-seat course

Recommended settings:

```json
{
  "course_name": "k6-10000-students",
  "teacher_name": "Load Test Teacher",
  "capacity": 1000,
  "time_slot": 15,
  "credit": 2,
  "status": 1
}
```

### Step 3. Run the load test

To approximate "10000 students submit within 1 second", use:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:9000/api/v1 -e COURSE_ID=12 -e USERS=10000 -e SPREAD_MS=1000 -e STUDENT_PREFIX=220 -e STUDENT_PAD_WIDTH=5 -e LOGIN_BATCH_SIZE=100 -e SETUP_TIMEOUT=30m .\tests\select_many_students.js
```

Interpretation:

- `USERS=10000`: 10000 distinct student accounts
- `SPREAD_MS=1000`: all submissions are spread across a 1-second window
- `STUDENT_PREFIX=220`
- `STUDENT_PAD_WIDTH=5`
- `LOGIN_BATCH_SIZE=100`
- `SETUP_TIMEOUT=30m`

### Step 4. Verify final state

Expected result:

- `selected_count = 1000`
- `actual_selected = 1000`
- duplicate query returns empty
- no oversell

## 6. 5000 Students Grab 500 Seats in 1 Second

### Step 1. Seed 5000 stress-test students

Run:

```text
tests/seed_stress_students_5000.sql
```

It creates students:

```text
21990001 ~ 21995000
```

If you want to reuse the same 5000 students for another round, reset them first:

```text
tests/reset_stress_students_5000.sql
```

### Step 2. Create a fresh 500-seat course

Recommended settings:

```json
{
  "course_name": "k6-5000-students",
  "teacher_name": "Load Test Teacher",
  "capacity": 500,
  "time_slot": 16,
  "credit": 2,
  "status": 1
}
```

### Step 3. Run the load test

To approximate "5000 students submit within 1 second", use:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:8080/api/v1 -e COURSE_ID=12 -e USERS=5000 -e SPREAD_MS=1000 -e STUDENT_PREFIX=2199 -e STUDENT_PAD_WIDTH=4 -e LOGIN_BATCH_SIZE=100 -e SETUP_TIMEOUT=30m .\tests\select_many_students.js
```

Interpretation:

- `USERS=5000`: 5000 distinct student accounts
- `SPREAD_MS=1000`: all submissions are spread across a 1-second window
- `STUDENT_PREFIX=2199`
- `STUDENT_PAD_WIDTH=4`
- `LOGIN_BATCH_SIZE=100`
- `SETUP_TIMEOUT=30m`

### Step 4. Verify final state

Expected result:

- `selected_count = 500`
- `actual_selected = 500`
- duplicate query returns empty
- no oversell

## 7. 7500 Students Grab 250 Seats in 1 Second

### Step 1. Seed 7500 stress-test students

Run:

```text
tests/seed_stress_students_7500.sql
```

It creates students:

```text
22990001 ~ 22997500
```

If you want to reuse the same 7500 students for another round, reset them first:

```text
tests/reset_stress_students_7500.sql
```

### Step 2. Create a fresh 250-seat course

Recommended settings:

```json
{
  "course_name": "k6-7500-students",
  "teacher_name": "Load Test Teacher",
  "capacity": 250,
  "time_slot": 17,
  "credit": 2,
  "status": 1
}
```

### Step 3. Run the load test

To approximate "7500 students submit within 1 second", use:

```powershell
k6 run -e BASE_URL=http://localhost:18080/api/v1 -e COURSE_ID=12 -e USERS=7500 -e SPREAD_MS=1000 -e STUDENT_PREFIX=2299 -e STUDENT_PAD_WIDTH=4 -e LOGIN_BATCH_SIZE=100 -e SETUP_TIMEOUT=30m .\tests\select_many_students.js
```

Interpretation:

- `USERS=7500`: 7500 distinct student accounts
- `SPREAD_MS=1000`: all submissions are spread across a 1-second window
- `STUDENT_PREFIX=2299`
- `STUDENT_PAD_WIDTH=4`
- `LOGIN_BATCH_SIZE=100`
- `SETUP_TIMEOUT=30m`

### Step 4. Verify final state

Expected result:

- `selected_count = 250`
- `actual_selected = 250`
- duplicate query returns empty
- no oversell
