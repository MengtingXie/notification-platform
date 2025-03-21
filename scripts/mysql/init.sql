-- create the databases
CREATE DATABASE IF NOT EXISTS `notification`;

-- create the users for each database
CREATE USER 'notification'@'%' IDENTIFIED BY 'notification';
GRANT CREATE, ALTER, INDEX, LOCK TABLES, REFERENCES, UPDATE, DELETE, DROP, SELECT, INSERT ON `notification`.* TO 'notification'@'%';

FLUSH PRIVILEGES;