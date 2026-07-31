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

// AccessReviewPage: campanha de revisão de acesso (pacote 008, T-012). Escolhido um
// ativo, consome GET /api/v1/access/review?asset=<id> — quem alcança o ativo e a ORIGEM
// (direto/herdado/concessão), decidido pelo PDP (pacote 007). Fail-closed: 503 do PDP vira
// aviso "não pôde concluir", nunca uma lista vazia apresentada como "ninguém tem acesso".
import React from "react";
import {Alert, Card, Empty, Select, Space, Table, Tag, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "./backend/ControlPlaneBackend";

// mapeia a origem (o backend devolve os rótulos pt do domínio) para cor + i18n.
const ORIGIN_META = {
  "direto": {color: "blue", key: "general:Direct"},
  "herdado": {color: "geekblue", key: "general:Inherited"},
  "concessão": {color: "gold", key: "general:Grant"},
};

function renderOrigin(o) {
  const meta = ORIGIN_META[o] || {color: "default", key: null};
  return <Tag key={o} color={meta.color}>{meta.key ? i18next.t(meta.key) : o}</Tag>;
}

class AccessReviewPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      assets: [],
      assetId: null,
      entries: [],
      loading: false,
      pdpError: false,
      loadedFor: null,
    };
  }

  componentDidMount() {
    ControlPlane.getAssets()
      .then((res) => this.setState({assets: (res && res.assets) ? res.assets : []}))
      .catch(() => this.setState({assets: []}));
  }

  review(assetId) {
    this.setState({assetId, loading: true, pdpError: false});
    ControlPlane.getAccessReview(assetId)
      .then((res) => {
        this.setState({entries: (res && res.entries) ? res.entries : [], loading: false, loadedFor: assetId});
      })
      .catch((err) => {
        // 503 do PDP → fail-closed: sinaliza que não pôde concluir, nunca "ninguém tem acesso".
        const pdpDown = err && err.status === 503;
        this.setState({entries: [], loading: false, pdpError: pdpDown, loadedFor: assetId});
        if (pdpDown) {
          message.error(i18next.t("general:Review could not be completed (PDP)"));
        }
      });
  }

  columns() {
    return [
      {
        title: i18next.t("general:Membership"),
        dataIndex: "membership_ref",
        key: "membership_ref",
        render: (ref) => <span style={{fontFamily: "monospace"}}>{ref}</span>,
      },
      {
        title: i18next.t("general:Origin"),
        dataIndex: "origins",
        key: "origins",
        render: (origins) => (origins || []).map(renderOrigin),
      },
    ];
  }

  render() {
    const {assets, entries, loading, pdpError, loadedFor} = this.state;
    return (
      <div style={{padding: "16px", maxWidth: "960px", margin: "0 auto"}}>
        <Card title={i18next.t("general:Access review")}>
          <Space direction="vertical" size="middle" style={{width: "100%"}}>
            <Select
              showSearch
              style={{width: "100%", maxWidth: 480}}
              placeholder={i18next.t("general:Select an asset")}
              optionFilterProp="label"
              value={this.state.assetId}
              onChange={(v) => this.review(v)}
              options={assets.map((a) => ({value: a.id, label: `${a.name} (${a.kind})`}))}
            />

            {pdpError ? (
              <Alert type="error" showIcon message={i18next.t("general:Review could not be completed (PDP)")} />
            ) : null}

            {loadedFor && !pdpError && entries.length === 0 && !loading ? (
              <Empty description={i18next.t("general:No effective access to this asset")} />
            ) : (
              <Table
                rowKey="membership_ref"
                loading={loading}
                columns={this.columns()}
                dataSource={entries}
                pagination={false}
                locale={{emptyText: loadedFor ? undefined : i18next.t("general:Select an asset")}}
              />
            )}
          </Space>
        </Card>
      </div>
    );
  }
}

export default AccessReviewPage;
