-- Assertion helpers shared by the files in this directory.
--
-- Included with \ir so the path resolves relative to the including script rather
-- than to the caller's working directory.
--
-- The functions live in pg_temp: they exist for the session only and never become
-- part of the schema a migration has to account for.

CREATE FUNCTION pg_temp.assert_rejected(statement text, expected_sqlstate text, description text)
RETURNS void AS $$
BEGIN
    BEGIN
        EXECUTE statement;
    EXCEPTION WHEN others THEN
        IF SQLSTATE = expected_sqlstate THEN
            RAISE NOTICE 'ok  %', description;
            RETURN;
        END IF;

        -- Failing for the wrong reason is not a pass: a foreign key error where a
        -- privilege error was expected would hide a broken grant.
        RAISE EXCEPTION 'FAIL % (expected SQLSTATE %, got %: %)',
            description, expected_sqlstate, SQLSTATE, SQLERRM;
    END;

    RAISE EXCEPTION 'FAIL % (the statement was accepted)', description;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION pg_temp.assert_accepted(statement text, description text)
RETURNS void AS $$
BEGIN
    EXECUTE statement;
    RAISE NOTICE 'ok  %', description;
EXCEPTION WHEN others THEN
    RAISE EXCEPTION 'FAIL % (rejected with %: %)', description, SQLSTATE, SQLERRM;
END;
$$ LANGUAGE plpgsql;
