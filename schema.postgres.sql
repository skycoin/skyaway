create table botuser (
	id         int primary key not null,
	username   text,
	first_name text,
	last_name  text,
	enlisted   bool not null default true,
	banned     bool not null default false,
	admin      bool not null default false
);

create table event (
	id           serial primary key,
	duration     bigint not null, -- nanoseconds
	scheduled_at timestamp with time zone,
	started_at   timestamp with time zone,
	ended_at     timestamp with time zone,
	coins        int not null,
	surprise     boolean not null
);

--create type chattype as enum ('private', 'group', 'supergroup', 'channel');
--create table chat (
--	id int primary key not null,
--	title text,
--	chattype chattype not null,
--	username text,
--	first_name text,
--	last_name text,
--);