create table botuser (
	id int primary key not null,
	username text,
	first_name text,
	last_name text,
	joined_at timestamp with time zone,
	left_at timestamp with time zone,
	banned_at timestamp with time zone,
	admin bool not null default false
);
