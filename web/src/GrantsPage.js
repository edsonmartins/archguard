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

// GrantsPage: concessões privilegiadas VIGENTES do tenant ativo (pacote 008, T-006).
// Consome GET /api/v1/grants (o backend lê a org da sessão). Somente leitura por ora,
// com contagem regressiva ao vivo até `expires_at` — a revogação (POST) é a Parte B,
// que exige expor o endpoint no /api/v1 (I-7.6: capacidade do pacote 004 existe,
// falta a rota HTTP).
import React from "react";
import {Button, Card, Popconfirm, Table, Tag, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "./backend/ControlPlaneBackend";

function formatRemaining(expiresAt, nowSec) {
  const secs = expiresAt - nowSec;
  if (secs <= 0) {
    return {text: i18next.t("general:Expired"), color: "red"};
  }
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = secs % 60;
  const parts = [];
  if (h) {parts.push(`${h}h`);}
  if (h || m) {parts.push(`${m}m`);}
  parts.push(`${s}s`);
  // < 5 min = laranja (atenção), senão verde.
  return {text: parts.join(" "), color: secs < 300 ? "orange" : "green"};
}

class GrantsPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {grants: [], loading: true, now: Math.floor(Date.now() / 1000), revoking: null};
  }

  componentDidMount() {
    this.fetch();
    // Tique de 1s para a contagem regressiva (Date.now não é usado em setState direto
    // do render; só aqui, num efeito temporal — o build proíbe Date.now no domínio, não
    // na UI). Limpo no unmount.
    this.timer = setInterval(() => this.setState({now: Math.floor(Date.now() / 1000)}), 1000);
  }

  componentWillUnmount() {
    if (this.timer) {clearInterval(this.timer);}
  }

  fetch() {
    ControlPlane.getGrants()
      .then((res) => {
        const grants = (res && res.grants) ? res.grants : [];
        this.setState({grants, loading: false});
      })
      .catch((err) => {
        // Fail-closed: em dev/perfil sem projeção o PDP/lister nega — mostra vazio, não
        // uma lista parcial como se fosse completa. 401 = sem contexto /api/v1.
        this.setState({grants: [], loading: false, denied: err && err.status});
      });
  }

  revoke(grantId) {
    this.setState({revoking: grantId});
    ControlPlane.revokeGrant(grantId)
      .then(() => {
        message.success(i18next.t("general:Grant revoked"));
        this.setState({revoking: null});
        this.fetch();
      })
      .catch((err) => {
        // O 401 de step-up é tratado de forma transparente pelo interceptor (T-005);
        // aqui só caem negação (403), conflito (409 = já não ativa) e falhas (5xx).
        this.setState({revoking: null});
        const denied = err && err.status === 403;
        message.error(denied
          ? i18next.t("general:You are not allowed to revoke this grant")
          : (err && err.message) || i18next.t("general:Could not revoke the grant"));
      });
  }

  renderTable() {
    const {grants, now} = this.state;
    const columns = [
      {
        title: i18next.t("general:Target"),
        dataIndex: "target_id",
        key: "target",
        render: (_, r) => (
          <span>
            <Tag>{r.target_type}</Tag>
            <span style={{fontFamily: "monospace"}}>{r.target_id}</span>
            {r.target_scope ? <span style={{color: "#737373"}}> · {r.target_scope}</span> : null}
          </span>
        ),
      },
      {title: i18next.t("general:Origin"), dataIndex: "origin", key: "origin", width: 140},
      {
        title: i18next.t("general:Status"),
        dataIndex: "status",
        key: "status",
        width: 120,
        render: (v) => <Tag color={v === "active" ? "green" : "default"}>{v}</Tag>,
      },
      {
        title: i18next.t("general:Expires in"),
        key: "expires",
        width: 160,
        render: (_, r) => {
          const c = formatRemaining(r.expires_at, now);
          return <Tag color={c.color}>{c.text}</Tag>;
        },
      },
      {
        title: i18next.t("general:Action"),
        key: "action",
        width: 120,
        // Só concessões ativas são revogáveis; o Popconfirm explicita a consequência
        // destrutiva (revoga + encerra sessões derivadas). A autorização é do backend
        // (L3 + RequireAdmin) — esconder o botão não é controle de acesso.
        render: (_, r) => (
          r.status === "active" ? (
            <Popconfirm
              title={i18next.t("general:Revoke this grant?")}
              description={i18next.t("general:This revokes the grant and ends its derived sessions. Irreversible.")}
              okText={i18next.t("general:Revoke")}
              cancelText={i18next.t("general:Cancel")}
              okButtonProps={{danger: true}}
              onConfirm={() => this.revoke(r.grant_id)}
            >
              <Button danger size="small" loading={this.state.revoking === r.grant_id}>
                {i18next.t("general:Revoke")}
              </Button>
            </Popconfirm>
          ) : null
        ),
      },
    ];
    return (
      <Table
        rowKey="grant_id"
        size="middle"
        columns={columns}
        dataSource={grants}
        loading={this.state.loading}
        pagination={{pageSize: 20, hideOnSinglePage: true}}
        locale={{emptyText: i18next.t("general:No active grants")}}
      />
    );
  }

  render() {
    return (
      <div style={{padding: "16px"}}>
        <Card
          title={i18next.t("general:Active grants")}
          styles={{body: {padding: 0}}}
        >
          {this.renderTable()}
        </Card>
      </div>
    );
  }
}

export default GrantsPage;
