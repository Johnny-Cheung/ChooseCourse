-- Seed 5000 stress-test students for k6 load tests.
--
-- Student number pattern:
-- 21990001 ~ 21995000
--
-- Default password: 123456
-- bcrypt hash:
-- $2a$10$VA86e6YC19PMwyiIcyF/jueBSoZTzpx6Vtvcv4ZSiBbbj4HgJrN46

INSERT IGNORE INTO `students` (
  `student_no`,
  `password_hash`,
  `name`,
  `phone`,
  `credit_limit`,
  `credit_used`,
  `status`
)
SELECT
  CONCAT('2199', LPAD(nums.n, 4, '0')) AS student_no,
  '$2a$10$VA86e6YC19PMwyiIcyF/jueBSoZTzpx6Vtvcv4ZSiBbbj4HgJrN46' AS password_hash,
  CONCAT('StressStudent', LPAD(nums.n, 4, '0')) AS name,
  CONCAT('137', LPAD(nums.n, 8, '0')) AS phone,
  25 AS credit_limit,
  0 AS credit_used,
  1 AS status
FROM (
  SELECT
    ones.n + tens.n * 10 + hundreds.n * 100 + thousands.n * 1000 + 1 AS n
  FROM (
    SELECT 0 AS n UNION ALL
    SELECT 1 UNION ALL
    SELECT 2 UNION ALL
    SELECT 3 UNION ALL
    SELECT 4 UNION ALL
    SELECT 5 UNION ALL
    SELECT 6 UNION ALL
    SELECT 7 UNION ALL
    SELECT 8 UNION ALL
    SELECT 9
  ) AS ones
  CROSS JOIN (
    SELECT 0 AS n UNION ALL
    SELECT 1 UNION ALL
    SELECT 2 UNION ALL
    SELECT 3 UNION ALL
    SELECT 4 UNION ALL
    SELECT 5 UNION ALL
    SELECT 6 UNION ALL
    SELECT 7 UNION ALL
    SELECT 8 UNION ALL
    SELECT 9
  ) AS tens
  CROSS JOIN (
    SELECT 0 AS n UNION ALL
    SELECT 1 UNION ALL
    SELECT 2 UNION ALL
    SELECT 3 UNION ALL
    SELECT 4 UNION ALL
    SELECT 5 UNION ALL
    SELECT 6 UNION ALL
    SELECT 7 UNION ALL
    SELECT 8 UNION ALL
    SELECT 9
  ) AS hundreds
  CROSS JOIN (
    SELECT 0 AS n UNION ALL
    SELECT 1 UNION ALL
    SELECT 2 UNION ALL
    SELECT 3 UNION ALL
    SELECT 4
  ) AS thousands
) AS nums
WHERE nums.n BETWEEN 1 AND 5000;
