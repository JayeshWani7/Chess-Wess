DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'ChessWess') THEN
    CREATE USER "ChessWess" WITH PASSWORD 'ChessWess';
  END IF;
END
$$;

SELECT 'User ready' AS status;

CREATE DATABASE "ChessWess" OWNER "ChessWess";
