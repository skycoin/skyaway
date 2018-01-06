-- Users do not get deleted from the database. Only `enlisted` switches to
-- false if the user leaves the group.
CREATE TABLE botuser (
  id         INT PRIMARY KEY NOT NULL, -- telegram user id
  username   TEXT,
  first_name TEXT,
  last_name  TEXT,
  enlisted   BOOL            NOT NULL DEFAULT TRUE, -- is in the group
  banned     BOOL            NOT NULL DEFAULT FALSE, -- is disabled even if in the group
  admin      BOOL            NOT NULL DEFAULT FALSE  -- can issue commands
);

-- Only one event with null `ended_at` should exist, it is considered the
-- current event (scheduled or started).
-- `scheduled_at`, `started_at`, `ended_at` should never be null simultaneously.
CREATE TABLE event (
  id             SERIAL PRIMARY KEY,
  duration       BIGINT  NOT NULL, -- nanoseconds
  scheduled_at   TIMESTAMP WITH TIME zone, -- null if started without schedule
  started_at     TIMESTAMP WITH TIME zone, -- null if not started yet or canceled
  ended_at       TIMESTAMP WITH TIME zone, -- null if current event
  coins          INT     NOT NULL,
  surprise       BOOLEAN NOT NULL -- no automatic announcements
);

-- This table keeps track of user claims in events. The current list of users
-- is added to this table every time an event starts (with null `claimed_at`).
-- The number of coins for each user is calculated at the start, and then each
-- claim just sets `claimed_at`.
CREATE TABLE participant (
  event_id   INT NOT NULL REFERENCES event (id),
  user_id    INT NOT NULL REFERENCES botuser (id),
  username   TEXT,
  coins      INT NOT NULL, -- precalculated number of coins for the user
  claimed_at TIMESTAMP WITH TIME zone, -- null if not claimed yet
  PRIMARY KEY (event_id, user_id)
);
