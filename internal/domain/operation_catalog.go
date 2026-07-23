// Copyright 2026 IntegrAllTech Ltda.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package domain

// This file is the SINGLE SOURCE OF TRUTH for API-operation classification
// (INV-8 / ADR-0010). Every assurance-gated operation is listed EXPLICITLY here
// with its level — nothing is derived, so adding an operation is a deliberate
// classification, and the INV-8 invariant test fails the build if a catalogued
// audit verb has neither an operation classification nor an explicit exemption
// (T-017: "operação sem classificação ⇒ build rejeitado").
//
// Operation ids REUSE the audit action verbs where an operation is audited, so a
// verb and its endpoint share one identifier and can never drift apart; a few
// read-only operations that emit no dedicated audit verb carry their own id.

// classifiedOperations is the explicit list of assurance-gated API operations.
// The order is documentation only; the catalog is keyed by id.
var classifiedOperations = []Operation{
	// Read / low-assurance (a valid session suffices).
	{ID: "profile.read", Level: L1, Description: "ler o próprio perfil"},
	{ID: "session.list", Level: L1, Description: "listar as próprias sessões"},
	{ID: string(ActionAuthLogout), Level: L1, Description: "encerrar a sessão"},
	{ID: string(ActionTenantSelect), Level: L1, Description: "selecionar o tenant ativo"},
	{ID: string(ActionMembershipAccept), Level: L1, Description: "aceitar convite de organização"},

	// Factor enrollment — permitted while a session is in mandatory enrollment.
	{ID: string(ActionFactorEnroll), Level: L2, Description: "registrar um fator", AllowedDuringEnrollment: true},

	// Mutations / tenant administration (strong factor within a freshness window).
	{ID: string(ActionTenantSwitch), Level: L2, Description: "trocar de tenant"},
	{ID: string(ActionMembershipInvite), Level: L2, Description: "convidar para a organização"},
	{ID: string(ActionMembershipRevoke), Level: L2, Description: "revogar membership"},
	{ID: string(ActionIdentitySuspend), Level: L2, Description: "suspender identidade"},
	{ID: string(ActionAdminMutation), Level: L2, Description: "mutação administrativa"},
	{ID: string(ActionRecoveryRequest), Level: L2, Description: "abrir recuperação de fator"},

	// Privileged (immediate phishing-resistant re-authentication).
	{ID: string(ActionIdentityDeprovision), Level: L3, Description: "desprovisionar identidade"},
	{ID: string(ActionPrivilegedSessionOpen), Level: L3, Description: "abrir sessão privilegiada"},
	{ID: string(ActionBreakglassRequest), Level: L3, Description: "solicitar break-glass"},
	{ID: string(ActionBreakglassApprove), Level: L3, Description: "aprovar break-glass"},
	{ID: string(ActionKeyRotate), Level: L3, Description: "rotacionar chave"},
	{ID: string(ActionAuditExport), Level: L3, Description: "exportar a trilha de auditoria"},
	{ID: string(ActionAuditVerify), Level: L3, Description: "verificar a trilha de auditoria"},
	{ID: string(ActionFactorRemove), Level: L3, Description: "remover um fator forte"},
	{ID: string(ActionRecoveryApprove), Level: L3, Description: "aprovar recuperação de fator"},
	{ID: string(ActionRecoveryReset), Level: L3, Description: "resetar fator via recuperação"},
}

// operationExemptActions are catalogued audit verbs that are NOT assurance-gated
// API operations, each for a stated reason. They are exempt from operation
// classification — but the exemption is EXPLICIT, so a new audit verb must be
// either classified as an operation or added here on purpose (the INV-8 gate
// leaves no silent gap).
var operationExemptActions = map[Action]string{
	ActionAuthLogin:       "entrada pré-autenticação: não há sessão cuja garantia avaliar",
	ActionAuthLoginDenied: "resultado de auditoria de um login que falhou, nunca uma operação invocada",
	ActionAuthStepUp:      "a própria cerimônia de step-up (reautenticação), gated pelo seu fluxo, não pelo middleware",
	ActionAuthLockout:     "evento emitido pelo sistema (bloqueio progressivo), não invocado",
	ActionAuthStuffing:    "alerta emitido pelo sistema (credential stuffing), não invocado",
}

// BuildOperationCatalog builds the canonical, fully-populated operation catalog.
// It returns an error if any classified operation is malformed or duplicated —
// so a wiring mistake surfaces at startup (and in the INV-8 test), never as an
// unprotected path at runtime.
func BuildOperationCatalog() (*OperationCatalog, error) {
	cat := NewOperationCatalog()
	for _, op := range classifiedOperations {
		if err := cat.Register(op); err != nil {
			return nil, err
		}
	}
	return cat, nil
}

// OperationExemptActions returns the audit verbs deliberately exempt from
// operation classification, for the INV-8 completeness check.
func OperationExemptActions() map[Action]string {
	out := make(map[Action]string, len(operationExemptActions))
	for a, reason := range operationExemptActions {
		out[a] = reason
	}
	return out
}
