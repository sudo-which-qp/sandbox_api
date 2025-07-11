INSERT INTO
    roles (name, level, description)
VALUES
    ('user', 1, 'A User can only create posts'),
    (
        'moderator',
        2,
        'A Moderator can update and not delete posts'
    ),
    ('admin', 3, 'An Admin can do anything');