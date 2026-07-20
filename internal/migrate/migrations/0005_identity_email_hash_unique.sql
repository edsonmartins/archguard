-- 0005: unicidade de `identity.email_hash` (RFC-0002 §2.1, pacote 002 T-003).
--
-- `email_hash` é o HMAC (chave de deployment, via KeyCustodian) do e-mail
-- normalizado — sustenta unicidade e login SEM descriptografar nem comparar
-- plaintext. O índice é ÚNICO e PARCIAL (WHERE email_hash IS NOT NULL): contas
-- de serviço e identidades deprovisionadas (crypto-shredding, ADR-0014) têm
-- email_hash nulo e não devem colidir entre si nem impedir umas às outras.
--
-- Este índice também é o caminho de busca do login por hash (lookup por
-- email_hash), além da garantia de unicidade.
CREATE UNIQUE INDEX IF NOT EXISTS identity_email_hash_key
    ON identity (email_hash)
    WHERE email_hash IS NOT NULL;
