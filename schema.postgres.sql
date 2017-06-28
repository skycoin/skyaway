create table botuser (
	id int primary key not null,
	username text,
	first_name text,
	last_name text,
	enlisted bool not null default true,
	banned bool not null default false,
	admin bool not null default false
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
