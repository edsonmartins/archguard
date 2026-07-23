-- 0024: estado de enrolamento obrigatório na sessão (T-012).
--
-- Uma identidade PRIVILEGIADA que autentica sem fator forte registrado entra em
-- estado bloqueante: a sessão só permite operações de registro de fator até que
-- um fator forte seja enrolado (spec "Privilegiado sem fator" / ADR-0010). O
-- flag é decidido no login (RequiresEnrollment) e limpo após o enrolamento.
--
-- LGPD: sem campo pessoal — é um booleano de estado de sessão. Default false: a
-- ausência de exigência é o caso comum; o login LEVANTA o flag quando aplicável.

ALTER TABLE auth_session
    ADD COLUMN IF NOT EXISTS enrollment_required boolean NOT NULL DEFAULT false;
