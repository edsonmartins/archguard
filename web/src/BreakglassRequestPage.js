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

// BreakglassRequestPage: solicitação de acesso de emergência (break-glass) do pacote 008
// (T-007). Consome POST /api/v1/breakglass/request (op L3 → step-up transparente da T-005).
// O sujeito é o próprio operador (a org e o membership vêm da sessão no backend); aqui só
// o alvo opaco, a justificativa/incidente obrigatórios e a janela do acesso. Fail-closed:
// sem canal de notificação no tenant o backend nega (503) — o alerta é pré-condição.
import React from "react";
import {Alert, Button, Card, Input, Select, Space, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "./backend/ControlPlaneBackend";

const {TextArea} = Input;

class BreakglassRequestPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      targetType: "",
      targetId: "",
      targetScope: "",
      justification: "",
      incidentRef: "",
      durationHours: 4,
      submitting: false,
    };
  }

  set(field, value) {
    this.setState({[field]: value});
  }

  canSubmit() {
    const {targetType, targetId, justification, incidentRef} = this.state;
    return targetType.trim() && targetId.trim() && justification.trim() && incidentRef.trim();
  }

  submit() {
    if (!this.canSubmit()) {
      message.warning(i18next.t("general:Fill target, justification and incident"));
      return;
    }
    const {targetType, targetId, targetScope, justification, incidentRef, durationHours} = this.state;
    const expiresAt = Math.floor(Date.now() / 1000) + durationHours * 3600;
    this.setState({submitting: true});
    ControlPlane.requestBreakglass({
      target_type: targetType.trim(),
      target_id: targetId.trim(),
      target_scope: targetScope.trim(),
      justification: justification.trim(),
      incident_ref: incidentRef.trim(),
      expires_at: expiresAt,
    })
      .then(() => {
        // O alerta em tempo real já foi disparado no backend, ANTES de qualquer aprovação;
        // a concessão nasce pendente de aprovação por pares distintos (fila da T-008).
        message.success(i18next.t("general:Break-glass requested — pending approval"));
        this.setState({submitting: false, justification: "", incidentRef: "", targetId: "", targetScope: ""});
      })
      .catch((err) => {
        // 401 de step-up é transparente (T-005); aqui caem 503 (sem canal), 422 (inválida),
        // 403 (negada) e falhas.
        this.setState({submitting: false});
        const unavailable = err && err.status === 503;
        message.error(unavailable
          ? i18next.t("general:No notification channel — request denied (fail-closed)")
          : (err && err.message) || i18next.t("general:Could not request break-glass"));
      });
  }

  render() {
    return (
      <div style={{padding: "16px", maxWidth: 720}}>
        <Card title={i18next.t("general:Request break-glass access")}>
          <Alert
            type="warning"
            showIcon
            style={{marginBottom: 16}}
            message={i18next.t("general:Emergency access")}
            description={i18next.t("general:This is audited emergency access. A real-time alert is sent immediately and the grant requires approval by distinct peers before it becomes active. Justification and incident are mandatory.")}
          />
          <Space direction="vertical" size="middle" style={{width: "100%"}}>
            <div>
              <div style={{marginBottom: 4}}>{i18next.t("general:Target type")} *</div>
              <Input
                placeholder="database / host / application"
                value={this.state.targetType}
                onChange={(e) => this.set("targetType", e.target.value)}
              />
            </div>
            <div>
              <div style={{marginBottom: 4}}>{i18next.t("general:Target id")} *</div>
              <Input
                placeholder="prod-oracle-01"
                value={this.state.targetId}
                onChange={(e) => this.set("targetId", e.target.value)}
              />
            </div>
            <div>
              <div style={{marginBottom: 4}}>{i18next.t("general:Target scope")}</div>
              <Input
                placeholder="read / admin (opcional)"
                value={this.state.targetScope}
                onChange={(e) => this.set("targetScope", e.target.value)}
              />
            </div>
            <div>
              <div style={{marginBottom: 4}}>{i18next.t("general:Justification")} *</div>
              <TextArea
                rows={3}
                value={this.state.justification}
                onChange={(e) => this.set("justification", e.target.value)}
              />
            </div>
            <div>
              <div style={{marginBottom: 4}}>{i18next.t("general:Incident reference")} *</div>
              <Input
                placeholder="INC-42"
                value={this.state.incidentRef}
                onChange={(e) => this.set("incidentRef", e.target.value)}
              />
            </div>
            <div>
              <div style={{marginBottom: 4}}>{i18next.t("general:Access window")}</div>
              <Select
                style={{width: 200}}
                value={this.state.durationHours}
                onChange={(v) => this.set("durationHours", v)}
                options={[
                  {value: 1, label: "1h"},
                  {value: 4, label: "4h"},
                  {value: 8, label: "8h"},
                ]}
              />
            </div>
            <Button
              danger
              type="primary"
              loading={this.state.submitting}
              disabled={!this.canSubmit()}
              onClick={() => this.submit()}
            >
              {i18next.t("general:Request break-glass")}
            </Button>
          </Space>
        </Card>
      </div>
    );
  }
}

export default BreakglassRequestPage;
