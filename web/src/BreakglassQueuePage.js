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

// BreakglassQueuePage: fila de aprovação de break-glass do pacote 008 (T-008). Consome
// GET /api/v1/breakglass/pending e POST /api/v1/breakglass/approve. A separação de deveres
// (o solicitante não aprova; aprovadores distintos) é imposta pelo BACKEND — o botão aqui
// não é o controle; a API nega (403/409). Aprovar é L3 → step-up transparente (T-005/005b).
import React from "react";
import {Button, Card, Popconfirm, Table, Tag, Tooltip, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "./backend/ControlPlaneBackend";

class BreakglassQueuePage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {pending: [], loading: true, approving: null};
  }

  componentDidMount() {
    this.fetch();
  }

  fetch() {
    ControlPlane.getBreakglassPending()
      .then((res) => {
        const pending = (res && res.pending) ? res.pending : [];
        this.setState({pending, loading: false});
      })
      .catch(() => {
        // Fail-closed: em negação/sem contexto mostra vazio, nunca uma fila parcial.
        this.setState({pending: [], loading: false});
      });
  }

  approve(grantId) {
    this.setState({approving: grantId});
    ControlPlane.approveBreakglass(grantId)
      .then(() => {
        message.success(i18next.t("general:Approval recorded"));
        this.setState({approving: null});
        this.fetch();
      })
      .catch((err) => {
        // O 401 de step-up é transparente (T-005/005b). Aqui caem 403 (auto-aprovação),
        // 409 (duplicada / não aguardando) e falhas.
        this.setState({approving: null});
        const status = err && err.status;
        let msg;
        if (status === 403) {
          msg = i18next.t("general:You cannot approve your own request");
        } else if (status === 409) {
          msg = (err && err.message) || i18next.t("general:Already approved or no longer pending");
        } else {
          msg = (err && err.message) || i18next.t("general:Could not record the approval");
        }
        message.error(msg);
      });
  }

  renderTable() {
    const {pending} = this.state;
    const columns = [
      {
        title: i18next.t("general:Requester"),
        dataIndex: "subject_membership_id",
        key: "requester",
        width: 160,
        render: (v) => <span style={{fontFamily: "monospace", fontSize: 12}}>{v}</span>,
      },
      {
        title: i18next.t("general:Target"),
        key: "target",
        render: (_, r) => (
          <span>
            <Tag>{r.target_type}</Tag>
            <span style={{fontFamily: "monospace"}}>{r.target_id}</span>
            {r.target_scope ? <span style={{color: "#737373"}}> · {r.target_scope}</span> : null}
          </span>
        ),
      },
      {
        title: i18next.t("general:Justification"),
        dataIndex: "justification",
        key: "justification",
        render: (v) => <Tooltip title={v}><span>{v}</span></Tooltip>,
      },
      {title: i18next.t("general:Incident reference"), dataIndex: "incident_ref", key: "incident", width: 120},
      {
        title: i18next.t("general:Approvals"),
        dataIndex: "required_approvals",
        key: "approvals",
        width: 110,
        render: (v) => <Tag color="orange">{i18next.t("general:Requires {n}").replace("{n}", v)}</Tag>,
      },
      {
        title: i18next.t("general:Action"),
        key: "action",
        width: 120,
        render: (_, r) => (
          <Popconfirm
            title={i18next.t("general:Approve this break-glass request?")}
            description={i18next.t("general:You must not be the requester and each approver counts once. The backend enforces this.")}
            okText={i18next.t("general:Approve")}
            cancelText={i18next.t("general:Cancel")}
            onConfirm={() => this.approve(r.grant_id)}
          >
            <Button type="primary" size="small" loading={this.state.approving === r.grant_id}>
              {i18next.t("general:Approve")}
            </Button>
          </Popconfirm>
        ),
      },
    ];
    return (
      <Table
        rowKey="grant_id"
        size="middle"
        columns={columns}
        dataSource={pending}
        loading={this.state.loading}
        pagination={{pageSize: 20, hideOnSinglePage: true}}
        locale={{emptyText: i18next.t("general:No break-glass requests awaiting approval")}}
      />
    );
  }

  render() {
    return (
      <div style={{padding: "16px"}}>
        <Card title={i18next.t("general:Break-glass approval queue")} styles={{body: {padding: 0}}}>
          {this.renderTable()}
        </Card>
      </div>
    );
  }
}

export default BreakglassQueuePage;
