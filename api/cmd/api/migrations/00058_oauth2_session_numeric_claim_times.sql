-- +goose Up
-- +goose StatementBegin
-- ory/fosite stored ID-token claim times as RFC 3339 strings and absent
-- claims as null; its Authelia fork decodes only Unix seconds and rejects
-- "ext": null. Rewrite sessions stored before the switch so they still
-- refresh: times become Unix seconds, zero times and null claims are dropped.
CREATE FUNCTION pg_temp.numeric_claim_times(claims jsonb) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
    SELECT coalesce(
        jsonb_object_agg(
            claim.key,
            CASE
                WHEN
                    claim.key IN ('exp', 'iat', 'auth_time', 'rat')
                    AND jsonb_typeof(claim.value) = 'string'
                    THEN to_jsonb(
                        floor(
                            extract(EPOCH FROM (claim.value #>> '{}')::timestamptz)
                        )::bigint
                    )
                ELSE claim.value
            END
        ),
        '{}'::jsonb
    )
    FROM jsonb_each(claims) AS claim
    WHERE
        claim.value != 'null'::jsonb
        AND NOT (
            claim.key IN ('exp', 'iat', 'auth_time', 'rat')
            AND claim.value #>> '{}' LIKE '0001-01-01%'
        )
$$;

UPDATE auth.oauth2_auth_codes
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.numeric_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims,iat}') = 'string';

UPDATE auth.oauth2_pkce_requests
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.numeric_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims,iat}') = 'string';

UPDATE auth.oauth2_oidc_sessions
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.numeric_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims,iat}') = 'string';

UPDATE auth.oauth2_access_tokens
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.numeric_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims,iat}') = 'string';

UPDATE auth.oauth2_refresh_tokens
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.numeric_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims,iat}') = 'string';

DROP FUNCTION pg_temp.numeric_claim_times(jsonb);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Unix-second claim times back to RFC 3339, the shape ory/fosite decodes; a
-- rollback runs this before the old binary starts.
CREATE FUNCTION pg_temp.rfc3339_claim_times(claims jsonb) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
    SELECT coalesce(
        jsonb_object_agg(
            claim.key,
            CASE
                WHEN
                    claim.key IN ('exp', 'iat', 'auth_time', 'rat')
                    AND jsonb_typeof(claim.value) = 'number'
                    THEN to_jsonb(
                        to_char(
                            to_timestamp((claim.value #>> '{}')::numeric)
                            AT TIME ZONE 'UTC',
                            'YYYY-MM-DD"T"HH24:MI:SS"Z"'
                        )
                    )
                ELSE claim.value
            END
        ),
        '{}'::jsonb
    )
    FROM jsonb_each(claims) AS claim
$$;

UPDATE auth.oauth2_auth_codes
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.rfc3339_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims}') = 'object';

UPDATE auth.oauth2_pkce_requests
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.rfc3339_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims}') = 'object';

UPDATE auth.oauth2_oidc_sessions
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.rfc3339_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims}') = 'object';

UPDATE auth.oauth2_access_tokens
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.rfc3339_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims}') = 'object';

UPDATE auth.oauth2_refresh_tokens
SET
    request = jsonb_set(
        request, '{session,id_token_claims}',
        pg_temp.rfc3339_claim_times(request #> '{session,id_token_claims}')
    )
WHERE jsonb_typeof(request #> '{session,id_token_claims}') = 'object';

DROP FUNCTION pg_temp.rfc3339_claim_times(jsonb);
-- +goose StatementEnd
