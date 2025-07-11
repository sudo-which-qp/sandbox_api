ALTER TABLE
    users
ADD
    COLUMN role_id INT UNSIGNED,
ADD
    FOREIGN KEY (role_id) REFERENCES roles(id);