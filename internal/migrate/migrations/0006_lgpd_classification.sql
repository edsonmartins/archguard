-- 0006: classificação LGPD dos campos pessoais introduzidos por 0002 (identity)
-- e 0004 (membership). Cumpre o I-3.3 (pétreo) e o ADR-0014 §2: todo campo com
-- dado pessoal declara categoria, finalidade, base legal e prazo de retenção no
-- modelo de dados. A classificação vive no catálogo (COMMENT ON COLUMN), logo é
-- consultável e viaja com o esquema.
--
-- Formato do comentário: `LGPD | categoria=… | finalidade=… | base_legal=… |
-- retencao=…`. A base legal final é do CONTROLADOR (o cliente, ADR-0014 §1/§3);
-- aqui declaramos a base típica de identidade/segurança de acesso privilegiado.
--
-- Migration nova (não edita 0002/0004, já versionadas) — roda em instalação nova
-- e existente. NOTA: falta ainda um gate automatizado que REJEITE campo pessoal
-- sem esta classificação (I-3.3); enquanto não existe, a classificação é
-- disciplina de revisão. Follow-up registrado no pacote 002/010.

COMMENT ON COLUMN identity.primary_email_enc IS
	'LGPD | categoria=contato (e-mail), pessoal direto | finalidade=identificação, autenticação e comunicação | base_legal=execução de contrato / legítimo interesse em segurança (LGPD Art. 7 V/IX; controlador decide) | retencao=enquanto identidade ativa; eliminação por crypto-shredding da chave por titular (ADR-0014)';

COMMENT ON COLUMN identity.email_hash IS
	'LGPD | categoria=pseudônimo derivado de e-mail (HMAC chave de deployment), pessoal | finalidade=unicidade e login sem descriptografar | base_legal=execução de contrato / legítimo interesse em segurança (controlador decide) | retencao=enquanto identidade ativa; limpo no deprovisionamento (ADR-0014)';

COMMENT ON COLUMN identity.display_name_enc IS
	'LGPD | categoria=nome de exibição, pessoal direto | finalidade=apresentação e identificação na interface | base_legal=execução de contrato / legítimo interesse em segurança (controlador decide) | retencao=enquanto identidade ativa; eliminação por crypto-shredding (ADR-0014)';

COMMENT ON COLUMN membership.attributes_enc IS
	'LGPD | categoria=atributos corporativos do vínculo (matrícula, centro de custo), pessoais no contexto do tenant | finalidade=gestão do vínculo do titular com a organização | base_legal=legítimo interesse do controlador (tenant) | retencao=enquanto o membership existir; segregado por tenant, nunca compartilhado';
