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

// AuditPage: linha do tempo de auditoria do tenant ativo (pacote 008, T-009). Consome
// GET /api/v1/audit/timeline (org da sessão) e GET /api/v1/audit/verify (L3 → step-up
// WebAuthn transparente). O INDICADOR DE INTEGRIDADE DA CADEIA fica SEMPRE visível no topo
// (agregado honesto): divergência é destaque MÁXIMO (vermelho), íntegra é verde discreto,
// e "não verificada nesta sessão" é neutro com o botão de verificar.
import React from "react";
import {Alert, Button, Card, Input, Select, Space, Table, Tag, Tooltip, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "./backend/ControlPlaneBackend";

function outcomeColor(outcome) {
  const o = (outcome || "").toLowerCase();
  if (o.includes("deny") || o.includes("denied")) {return "orange";}
  if (o.includes("fail")) {return "red";}
  if (o.includes("allow")) {return "green";}
  return "default";
}

class AuditPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      events: [],
      loading: true,
      limit: 50,
      filter: "",
      integrity: {status: "unknown"}, // unknown | intact | divergence
      verifying: false,
    };
  }

  componentDidMount() {
    this.fetch();
  }

  fetch() {
    this.setState({loading: true});
    ControlPlane.getAuditTimeline({limit: this.state.limit})
      .then((res) => {
        this.setState({events: (res && res.events) ? res.events : [], loading: false});
      })
      .catch(() => {
        // Fail-closed: negação/sem contexto → vazio, nunca uma trilha parcial como completa.
        this.setState({events: [], loading: false});
      });
  }

  verify() {
    this.setState({verifying: true});
    ControlPlane.verifyAuditChain()
      .then((res) => {
        this.setState({verifying: false, integrity: {status: "intact", ...res}});
        message.success(i18next.t("general:Chain verified — intact"));
      })
      .catch((err) => {
        // 409 = DIVERGÊNCIA (o corpo traz o detalhe). Outros = falha/step-up cancelado
        // (o 401 de step-up é conduzido transparente pelo interceptor da T-005/005b).
        if (err && err.status === 409 && err.body) {
          this.setState({verifying: false, integrity: {status: "divergence", ...err.body}});
        } else {
          this.setState({verifying: false});
          message.error(i18next.t("general:Chain verification could not be completed"));
        }
      });
  }

  renderIntegrity() {
    const {integrity, verifying} = this.state;
    const verifyButton = (
      <Button danger={integrity.status === "divergence"} type="primary" loading={verifying} onClick={() => this.verify()}>
        {i18next.t("general:Verify chain")}
      </Button>
    );
    if (integrity.status === "divergence") {
      const where = i18next.t("general:Divergence at sequence {n}").replace("{n}", integrity.first_divergence_seq);
      return (
        <Alert
          type="error"
          showIcon
          banner
          style={{marginBottom: 16}}
          message={<b>{i18next.t("general:AUDIT CHAIN DIVERGENCE DETECTED")}</b>}
          description={`${where}${integrity.divergence_kind ? " · " + integrity.divergence_kind : ""}${integrity.detail ? " · " + integrity.detail : ""}`}
          action={verifyButton}
        />
      );
    }
    if (integrity.status === "intact") {
      return (
        <Alert
          type="success"
          showIcon
          style={{marginBottom: 16}}
          message={i18next.t("general:Audit chain intact")}
          description={i18next.t("general:{e} events and {s} seals checked").replace("{e}", integrity.events_checked).replace("{s}", integrity.seals_checked)}
          action={verifyButton}
        />
      );
    }
    return (
      <Alert
        type="info"
        showIcon
        style={{marginBottom: 16}}
        message={i18next.t("general:Chain integrity not verified this session")}
        description={i18next.t("general:Run the verification (requires a security key) to confirm the trail was not tampered with.")}
        action={verifyButton}
      />
    );
  }

  filteredEvents() {
    const f = this.state.filter.trim().toLowerCase();
    if (!f) {return this.state.events;}
    return this.state.events.filter((e) =>
      [e.action, e.actor_subject, e.target_id, e.target_label, e.reason, e.pcid]
        .some((v) => (v || "").toLowerCase().includes(f)));
  }

  renderTable() {
    const columns = [
      {
        title: i18next.t("general:When"),
        dataIndex: "occurred_at",
        key: "when",
        width: 170,
        render: (v) => <span style={{whiteSpace: "nowrap"}}>{new Date(v * 1000).toLocaleString()}</span>,
      },
      {title: i18next.t("general:Action"), dataIndex: "action", key: "action", width: 200},
      {
        title: i18next.t("general:Outcome"),
        dataIndex: "outcome",
        key: "outcome",
        width: 110,
        render: (v) => <Tag color={outcomeColor(v)}>{v}</Tag>,
      },
      {
        title: i18next.t("general:Actor"),
        dataIndex: "actor_subject",
        key: "actor",
        width: 150,
        render: (v) => <span style={{fontFamily: "monospace", fontSize: 12}}>{v}</span>,
      },
      {
        title: i18next.t("general:Target"),
        key: "target",
        render: (_, r) => (
          r.target_type ? (
            <span>
              <Tag>{r.target_type}</Tag>
              <span>{r.target_label || r.target_id}</span>
            </span>
          ) : null
        ),
      },
      {
        title: i18next.t("general:Reason"),
        dataIndex: "reason",
        key: "reason",
        render: (v) => v ? <Tooltip title={v}><span>{v}</span></Tooltip> : null,
      },
      {
        title: "PCID",
        dataIndex: "pcid",
        key: "pcid",
        width: 120,
        render: (v) => v ? <span style={{fontFamily: "monospace", fontSize: 11}}>{v}</span> : null,
      },
    ];
    return (
      <Table
        rowKey="seq"
        size="small"
        columns={columns}
        dataSource={this.filteredEvents()}
        loading={this.state.loading}
        pagination={{pageSize: 25, hideOnSinglePage: true}}
        locale={{emptyText: i18next.t("general:No audit events")}}
      />
    );
  }

  render() {
    return (
      <div style={{padding: "16px"}}>
        {this.renderIntegrity()}
        <Card
          title={i18next.t("general:Audit timeline")}
          extra={
            <Space>
              <Input.Search
                allowClear
                placeholder={i18next.t("general:Filter")}
                style={{width: 220}}
                onChange={(e) => this.setState({filter: e.target.value})}
              />
              <Select
                value={this.state.limit}
                style={{width: 110}}
                onChange={(v) => this.setState({limit: v}, () => this.fetch())}
                options={[{value: 50, label: "50"}, {value: 100, label: "100"}, {value: 200, label: "200"}]}
              />
              <Button onClick={() => this.fetch()}>{i18next.t("general:Refresh")}</Button>
            </Space>
          }
          styles={{body: {padding: 0}}}
        >
          {this.renderTable()}
        </Card>
      </div>
    );
  }
}

export default AuditPage;
