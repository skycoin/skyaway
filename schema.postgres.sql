-- Users do not get deleted from the database. Only `enlisted` switches to
-- false if the user leaves the group.
create table botuser (
	id         int primary key not null,    -- telegram user id
	username   text,
	first_name text,
	last_name  text,
	enlisted   bool not null default true,  -- is in the group
	banned     bool not null default false, -- is disabled even if in the group
	admin      bool not null default false  -- can issue commands
);

-- Only one event with null `ended_at` should exist, it is considered the
-- current event (scheduled or started).
-- `scheduled_at`, `started_at`, `ended_at` should never be null simultaneously.
create table event (
	id           serial primary key,
	duration     bigint not null, -- nanoseconds
	scheduled_at timestamp with time zone, -- null if started without schedule
	started_at   timestamp with time zone, -- null if not started yet or canceled
	ended_at     timestamp with time zone, -- null if current event
	coins        int not null,
	surprise     boolean not null -- no automatic announcements
);

-- This table keeps track of user claims in events. The current list of users
-- is added to this table every time an event starts (with null `claimed_at`).
-- The number of coins for each user is calculated at the start, and then each
-- claim just sets `claimed_at`.
create table participant (
	event_id   int not null references event(id),
	user_id    int not null references botuser(id),
	coins      int not null, -- precalculated number of coins for the user
	claimed_at timestamp with time zone, -- null if not claimed yet
	primary key (event_id, user_id)
);
