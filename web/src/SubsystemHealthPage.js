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

// SubsystemHealthPage: saúde dos subsistemas do plano de controle (pacote 008, T-013).
// Consome GET /api/v1/health (PDP/cofre/auditoria/banco/perfil), que já devolve um
// AGREGADO HONESTO (o pior status vence — RFC-0005 §9). A tela reforça o invariante no
// cliente: o selo de topo é o PIOR entre o agregado do backend e o pior subsistema, então
// verde no topo NUNCA coexiste com pendência no detalhe. Erro de leitura é fail-closed
// (agregado "unavailable" + alerta), nunca um falso "ok".
import React from "react";
import {Alert, Button, Card, Space, Table, Tag, message} from "antd";
import {ReloadOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as ControlPlane from "./backend/ControlPlaneBackend";

// Severidade dos status, igual ao backend (aggregateStatus): quanto maior, pior.
const SEVERITY = {ok: 0, degraded: 1, unavailable: 2};

function severity(status) {
  return SEVERITY[status] ?? SEVERITY.unavailable; // status desconhecido é tratado como o pior
}

// worstOf: o status mais severo entre o agregado do backend e todos os subsistemas — a
// garantia cliente-side de que o topo nunca é mais otimista que o detalhe.
function worstOf(aggregate, subsystems) {
  let worst = aggregate || "unavailable";
  for (const s of subsystems) {
    if (severity(s.status) > severity(worst)) {
      worst = s.status;
    }
  }
  return worst;
}

function statusColor(status) {
  switch (status) {
  case "ok": return "green";
  case "degraded": return "orange";
  default: return "red"; // unavailable e desconhecido
  }
}

function statusLabel(status) {
  switch (status) {
  case "ok": return i18next.t("general:Operational");
  case "degraded": return i18next.t("general:Degraded");
  default: return i18next.t("general:Unavailable");
  }
}

class SubsystemHealthPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      subsystems: [],
      aggregate: "unknown",
      loading: true,
      error: false,
    };
  }

  componentDidMount() {
    this.fetch();
  }

  fetch() {
    this.setState({loading: true, error: false});
    ControlPlane.getHealth()
      .then((res) => {
        const subsystems = (res && Array.isArray(res.subsystems)) ? res.subsystems : [];
        this.setState({
          subsystems,
          aggregate: (res && res.status) ? res.status : "unavailable",
          loading: false,
        });
      })
      .catch(() => {
        // Fail-closed: não conseguimos ler a saúde → agregado indisponível, nunca "ok".
        this.setState({subsystems: [], aggregate: "unavailable", loading: false, error: true});
        message.error(i18next.t("general:Could not read subsystem health"));
      });
  }

  // O selo de topo é o PIOR status observado (agregado do backend reforçado pelo detalhe).
  overallStatus() {
    return worstOf(this.state.aggregate, this.state.subsystems);
  }

  renderBanner() {
    const overall = this.overallStatus();
    if (overall === "ok") {
      return <Alert type="success" showIcon message={i18next.t("general:All subsystems operational")} />;
    }
    if (overall === "degraded") {
      return <Alert type="warning" showIcon message={i18next.t("general:A subsystem needs attention")} />;
    }
    return <Alert type="error" showIcon message={i18next.t("general:A critical subsystem is unavailable")} />;
  }

  columns() {
    return [
      {
        title: i18next.t("general:Subsystem"),
        dataIndex: "name",
        key: "name",
        render: (name) => <strong>{name}</strong>,
      },
      {
        title: i18next.t("general:Status"),
        dataIndex: "status",
        key: "status",
        render: (status) => <Tag color={statusColor(status)}>{statusLabel(status)}</Tag>,
      },
      {
        title: i18next.t("general:Detail"),
        dataIndex: "detail",
        key: "detail",
        render: (detail) => detail || "—",
      },
    ];
  }

  render() {
    return (
      <div style={{padding: "16px", maxWidth: "960px", margin: "0 auto"}}>
        <Card
          title={i18next.t("general:Subsystem health")}
          extra={
            <Button icon={<ReloadOutlined />} onClick={() => this.fetch()} loading={this.state.loading}>
              {i18next.t("general:Refresh")}
            </Button>
          }
        >
          <Space direction="vertical" size="middle" style={{width: "100%"}}>
            {this.renderBanner()}
            {this.state.error ? (
              <Alert type="error" showIcon message={i18next.t("general:Could not read subsystem health")} />
            ) : null}
            <Table
              rowKey="name"
              loading={this.state.loading}
              columns={this.columns()}
              dataSource={this.state.subsystems}
              pagination={false}
            />
          </Space>
        </Card>
      </div>
    );
  }
}

export default SubsystemHealthPage;
