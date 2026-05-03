# Docker Local Run

This project can be run as a full Linux container stack with Docker Compose:

- MySQL 8
- Redis 7
- RabbitMQ 3 Management
- one-shot `migrate` job
- backend API server
- background `worker` for RabbitMQ selection consumer, notification consumer, and pending-request sweeping

## 1. Start the stack

From the repo root:

```powershell
docker compose up --build -d
```

Check status:

```powershell
docker compose ps
```

Watch backend logs:

```powershell
docker compose logs -f backend
```

Watch worker logs:

```powershell
docker compose logs -f worker
```

The backend is exposed on:

```text
http://127.0.0.1:9000
```

## 2. Default ports on the host

- backend: `18080`
- MySQL: `23307`
- Redis: `16380`
- RabbitMQ AMQP: `5673`
- RabbitMQ management UI: `15673`

## 3. Default seeded accounts

The `migrate` service runs `./migrate -seed`, so these demo accounts are available:

- admin: `A0001 / 123456`
- students: `20230001 ~ 20230003 / 123456`

For the 500-student load test, also execute:

```powershell
Get-Content .\tests\seed_stress_students_500.sql | docker exec -i choose-course-mysql mysql -uroot -phsp choose_course
```

## 4. Create a fresh load-test course

Use Postman or any HTTP client against:

```text
POST http://127.0.0.1:9000/api/v1/admin/courses
```

Recommended body:

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

## 5. Run k6 from the host

Use a fresh course ID each round:

```powershell
k6 run -e BASE_URL=http://127.0.0.1:9000/api/v1 -e COURSE_ID=12 -e USERS=500 -e SPREAD_MS=2000 .\tests\select_many_students.js
```

`SPREAD_MS=2000` spreads the submission requests across a 2-second window, which is often more stable for localhost testing on Windows hosts.

## 5.1 Run k6 inside Docker

When running high-concurrency tests on Windows, it is more stable to place `k6`
inside the same Docker network as the backend, instead of going through the
host-mapped port.

Use the Compose `k6` tool profile and target the backend container directly:

```powershell
docker compose --profile tools run --rm k6 run -e BASE_URL=http://backend:8080/api/v1 -e COURSE_ID=12 -e USERS=1000 -e SPREAD_MS=1000 -e LOGIN_BATCH_SIZE=100 -e SETUP_TIMEOUT=15m /work/tests/select_many_students.js
```

For larger rounds, adjust `USERS`, `SPREAD_MS`, `LOGIN_BATCH_SIZE`,
`SETUP_TIMEOUT`, and if needed `STUDENT_PREFIX` / `STUDENT_PAD_WIDTH`.

## 6. Verify the final result

```powershell
docker exec -it choose-course-mysql mysql -uroot -phsp choose_course
```

Then run:

```sql
SELECT status, COUNT(*)
FROM selection_requests
WHERE course_id = 12
GROUP BY status;

SELECT id, capacity, selected_count, status
FROM courses
WHERE id = 12;

SELECT COUNT(*) AS actual_selected
FROM enrollments
WHERE course_id = 12 AND status = 1;

SELECT student_id, COUNT(*) AS cnt
FROM enrollments
WHERE course_id = 12 AND status = 1
GROUP BY student_id
HAVING COUNT(*) > 1;
```

## 7. Reset the entire environment

Stop containers but keep data:

```powershell
docker compose down
```

Stop containers and delete all volumes:

```powershell
docker compose down -v
```

`down -v` is the simplest way to get a completely clean MySQL / Redis / RabbitMQ state for a new round of testing.
