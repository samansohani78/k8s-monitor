-- K8sWatch Database Health Check User Creation Scripts
-- These scripts create read-only users with minimal permissions for health checks
-- 
-- IMPORTANT: Replace <STRONG_PASSWORD> with a generated secure password before running
-- Generate password: openssl rand -base64 32

--------------------------------------------------------------------------------
-- PostgreSQL
--------------------------------------------------------------------------------
-- Health check: SELECT 1 (no table access needed)
-- Connection: Requires CONNECT on database
-- Minimal grants - no table permissions required

-- Create read-only health check user
CREATE ROLE k8swatch_reader WITH 
  LOGIN 
  PASSWORD '<STRONG_PASSWORD>'
  CONNECTION LIMIT 5;  -- Limit concurrent connections

-- Grant connect to database (adjust database name as needed)
GRANT CONNECT ON DATABASE postgres TO k8swatch_reader;

-- Optional: Grant usage on schema (not needed for SELECT 1)
-- GRANT USAGE ON SCHEMA public TO k8swatch_reader;

-- Verify user creation
\du k8swatch_reader

-- Test connection (run as k8swatch_reader):
-- psql -U k8swatch_reader -d postgres -c "SELECT 1;"

-- To drop the user:
-- REVOKE CONNECT ON DATABASE postgres FROM k8swatch_reader;
-- DROP ROLE k8swatch_reader;

--------------------------------------------------------------------------------
-- MySQL / MariaDB
--------------------------------------------------------------------------------
-- Health check: SELECT 1 (no table access needed)
-- MySQL users don't need any grants for SELECT 1

-- Create health check user (MySQL 5.7+)
CREATE USER 'k8swatch_reader'@'%' 
  IDENTIFIED BY '<STRONG_PASSWORD>'
  WITH MAX_USER_CONNECTIONS 5;  -- Limit concurrent connections

-- No grants needed for SELECT 1
-- The user can connect and run SELECT 1 without any privileges

-- For MySQL 8.0+ with caching_sha2_password (default):
-- CREATE USER 'k8swatch_reader'@'%' 
--   IDENTIFIED WITH caching_sha2_password BY '<STRONG_PASSWORD>';

-- Verify user creation
SELECT User, Host, plugin FROM mysql.user WHERE User = 'k8swatch_reader';

-- Test connection:
-- mysql -u k8swatch_reader -p -e "SELECT 1;"

-- To drop the user:
-- DROP USER 'k8swatch_reader'@'%';

--------------------------------------------------------------------------------
-- MongoDB
--------------------------------------------------------------------------------
-- Health check: db.runCommand({ ping: 1 })
-- Requires: Built-in roles that allow ping command

-- Switch to admin database
use admin

-- Create health check user with minimal role
-- The "read" role on admin database allows ping command
db.createUser({
  user: "k8swatch_reader",
  pwd: "<STRONG_PASSWORD>",
  roles: [
    { role: "read", db: "admin" }
  ],
  // Optional: Add authentication constraints
  // mechanisms: ["SCRAM-SHA-256"],
  // restrictions: {
  //   subject: "CN=k8swatch-agent"  // For mTLS
  // }
});

-- Alternative: Create custom role with only ping permission
// use admin
// db.createRole({
//   role: "k8swatch_healthcheck",
//   privileges: [
//     {
//       resource: { cluster: true },
//       actions: ["listDatabases", "listCollections"]
//     }
//   ],
//   roles: []
// });
// 
// db.createUser({
//   user: "k8swatch_reader",
//   pwd: "<STRONG_PASSWORD>",
//   roles: [{ role: "k8swatch_healthcheck", db: "admin" }]
// });

-- Verify user creation
db.getUser("k8swatch_reader");

-- Test connection:
// mongo -u k8swatch_reader -p <password> --authenticationDatabase admin
// db.runCommand({ ping: 1 });

-- To drop the user:
// db.dropUser("k8swatch_reader");

--------------------------------------------------------------------------------
-- Redis
--------------------------------------------------------------------------------
-- Health check: PING, then AUTH, then INFO server
-- Redis 6.0+ supports ACLs

-- For Redis 6.0+ with ACL
ACL SETUSER k8swatch_reader on >"<STRONG_PASSWORD>" ~* +@all -@dangerous -@admin

-- Explanation:
-- on: Enable user
-- >"<PASSWORD>": Set password
// ~*: Access to all keys (needed for PING)
// +@all: Allow all command categories
// -@dangerous: Block dangerous commands (DEBUG, SHUTDOWN, etc.)
// -@admin: Block admin commands (CONFIG, ACL, etc.)

-- Verify user creation
ACL LIST
ACL GETUSER k8swatch_reader

-- Test connection:
// redis-cli -u k8swatch_reader:<password>@localhost:6379 PING

-- For Redis < 6.0 (no ACL support):
// Set requirepass in redis.conf:
// requirepass <STRONG_PASSWORD>
// All clients will need to AUTH with this password

-- To drop the user (Redis 6.0+):
// ACL DELUSER k8swatch_reader

--------------------------------------------------------------------------------
-- Elasticsearch / OpenSearch
--------------------------------------------------------------------------------
-- Health check: GET /_cluster/health
-- Requires: Cluster health monitoring permissions

-- Create role with minimal permissions (Elasticsearch Native Realm)
PUT /_security/role/k8swatch_healthcheck
{
  "cluster": [
    "cluster:monitor/health",
    "cluster:monitor/main"
  ],
  "indices": [],
  "applications": [],
  "run_as": [],
  "metadata": {
    "description": "K8sWatch health check role",
    "created_by": "k8swatch"
  }
}

-- Create user with health check role
PUT /_security/user/k8swatch_reader
{
  "password": "<STRONG_PASSWORD>",
  "roles": ["k8swatch_healthcheck"],
  "full_name": "K8sWatch Health Check",
  "email": "k8swatch@example.com",
  "metadata": {
    "created_by": "k8swatch"
  },
  "enabled": true
}

-- Verify user creation
GET /_security/user/k8swatch_reader

-- Test connection:
// curl -u k8swatch_reader:<password> http://localhost:9200/_cluster/health

-- Alternative: Use API Key (more secure, supports rotation)
POST /_security/api_key
{
  "name": "k8swatch-healthcheck-key",
  "role_descriptors": {
    "k8swatch_healthcheck": {
      "cluster": ["cluster:monitor/health", "cluster:monitor/main"]
    }
  },
  "expiration": "365d"
}

-- Response:
// {
//   "id": "<api_key_id>",
//   "name": "k8swatch-healthcheck-key",
//   "api_key": "<api_key>",
//   "encoded": "<base64_encoded_api_key>"
// }

-- Use encoded API key in Authorization header:
// Authorization: ApiKey <encoded>

-- To delete API key:
// DELETE /_security/api_key/<api_key_id>

--------------------------------------------------------------------------------
-- ClickHouse
--------------------------------------------------------------------------------
-- Health check: SELECT 1
-- Minimal permissions needed

-- Create read-only user (ClickHouse 20.4+)
CREATE USER k8swatch_reader IDENTIFIED BY '<STRONG_PASSWORD>'
  SETTINGS 
    max_execution_time = 10,
    readonly = 1;  -- Read-only mode

-- No additional grants needed for SELECT 1

-- Verify user creation
SELECT name, auth_type FROM system.users WHERE name = 'k8swatch_reader';

-- Test connection:
// clickhouse-client --user k8swatch_reader --password <password> --query "SELECT 1"

-- To drop the user:
// DROP USER k8swatch_reader;

--------------------------------------------------------------------------------
-- Microsoft SQL Server (MSSQL)
--------------------------------------------------------------------------------
-- Health check: SELECT 1
-- Requires: CONNECT SQL permission

-- Create login
CREATE LOGIN k8swatch_reader WITH PASSWORD = '<STRONG_PASSWORD>';

-- Create user in master database
USE master;
CREATE USER k8swatch_reader FOR LOGIN k8swatch_reader;

-- Grant minimal permissions
GRANT CONNECT SQL TO k8swatch_reader;
GRANT VIEW SERVER STATE TO k8swatch_reader;  -- Optional: for additional health info

-- No table grants needed for SELECT 1

-- Verify user creation
SELECT name, type_desc, is_disabled FROM sys.server_principals WHERE name = 'k8swatch_reader';

-- Test connection:
// sqlcmd -S localhost -U k8swatch_reader -P <password> -Q "SELECT 1"

-- To drop the user:
// USE master;
// DROP USER k8swatch_reader;
// DROP LOGIN k8swatch_reader;

--------------------------------------------------------------------------------
-- Password Generation
--------------------------------------------------------------------------------
-- Generate strong password (32 characters):
-- openssl rand -base64 32

-- Generate password for PostgreSQL:
-- psql -c "SELECT md5(random()::text || clock_timestamp()::text);"

-- Generate password using Python:
-- python3 -c "import secrets; print(secrets.token_urlsafe(32))"

-- Generate password using Go:
-- go run -c 'package main; import ("crypto/rand"; "encoding/base64"; "fmt"); func main() { b := make([]byte, 32); rand.Read(b); fmt.Println(base64.StdEncoding.EncodeToString(b)) }'

--------------------------------------------------------------------------------
-- Security Best Practices
--------------------------------------------------------------------------------
-- 1. Use strong, randomly generated passwords
-- 2. Rotate passwords regularly (every 90 days recommended)
-- 3. Store passwords in Kubernetes Secrets, never in code
-- 4. Use TLS for all database connections
-- 5. Restrict source IPs in database firewall rules
-- 6. Monitor failed authentication attempts
-- 7. Use connection limits to prevent DoS
-- 8. Audit user permissions regularly
-- 9. Use SCRAM-SHA-256 or better authentication when available
-- 10. Consider using external secrets managers (Vault, AWS Secrets Manager)
