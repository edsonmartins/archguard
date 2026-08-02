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

// AccessManagementPage: gestão do acesso granular do tenant (pacote 007 M4). Três abas sobre
// os endpoints /api/v1: ATIVOS (asset catalog), ATRIBUIÇÕES (operator/auditor de um membership
// ou grupo sobre um ativo) e VÍNCULOS DE GRUPO (membership↔grupo de acesso). Toda escrita é no
// tenant ativo da sessão (o backend lê a org da sessão, nunca do request).
import React from "react";
import {Button, Card, Input, Select, Space, Table, Tabs, Tag, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "./backend/ControlPlaneBackend";

const RELATION_COLOR = {operator: "blue", auditor: "geekblue"};
const short = (id) => (id ? String(id).split(":").pop().slice(0, 8) : "");

class AccessManagementPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      assets: [], assignments: [], groups: [], memberships: [], catalog: [], loading: false,
      // formulários
      assetKind: "", assetName: "",
      asgSubjectType: "membership", asgSubjectId: "", asgRelation: "operator", asgObjectId: "",
      gmGroupId: "", gmMembershipId: "",
      grpName: "", grpDisplay: "",
    };
  }

  componentDidMount() {
    this.reload();
  }

  reload() {
    this.setState({loading: true});
    Promise.all([
      ControlPlane.getAssets().catch(() => ({})),
      ControlPlane.getAccessAssignments().catch(() => ({})),
      ControlPlane.getGroupMemberships().catch(() => ({})),
      ControlPlane.getMemberships().catch(() => ({})),
      ControlPlane.getAccessGroups().catch(() => ({})),
    ]).then(([a, asg, gm, m, cat]) => {
      this.setState({
        assets: (a && a.assets) || [],
        assignments: (asg && asg.assignments) || [],
        groups: (gm && gm.group_memberships) || [],
        memberships: (m && m.memberships) || [],
        catalog: (cat && cat.groups) || [],
        loading: false,
      });
    });
  }

  create(fn, payload, onOk) {
    fn(payload)
      .then(() => {
        message.success(i18next.t("general:Created"));
        onOk();
        this.reload();
      })
      .catch(() => message.error(i18next.t("general:Operation failed")));
  }

  membershipOptions() {
    return this.state.memberships.map((m) => ({
      value: m.membership_id,
      label: `${short(m.membership_id)} · ${m.status}`,
    }));
  }
  assetOptions() {
    return this.state.assets.map((a) => ({value: a.id, label: `${a.name} (${a.kind})`}));
  }
  groupOptions() {
    return this.state.catalog.map((g) => ({value: g.id, label: g.display_name || g.name}));
  }
  groupLabel(id) {
    const g = this.state.catalog.find((x) => x.id === id);
    return g ? (g.display_name || g.name) : short(id);
  }

  // ---- Aba ATIVOS ----
  renderAssets() {
    const columns = [
      {title: i18next.t("general:Name"), dataIndex: "name", key: "name"},
      {title: i18next.t("general:Kind"), dataIndex: "kind", key: "kind", render: (k) => <Tag>{k}</Tag>},
      {title: "ID", dataIndex: "id", key: "id", render: (id) => <span style={{fontFamily: "monospace"}}>{short(id)}</span>},
    ];
    return (
      <Space direction="vertical" size="middle" style={{width: "100%"}}>
        <Space wrap>
          <Input style={{width: 160}} placeholder={i18next.t("general:Kind")} value={this.state.assetKind}
            onChange={(e) => this.setState({assetKind: e.target.value})} />
          <Input style={{width: 240}} placeholder={i18next.t("general:Name")} value={this.state.assetName}
            onChange={(e) => this.setState({assetName: e.target.value})} />
          <Button type="primary" disabled={!this.state.assetKind || !this.state.assetName}
            onClick={() => this.create(ControlPlane.createAsset,
              {kind: this.state.assetKind, name: this.state.assetName},
              () => this.setState({assetKind: "", assetName: ""}))}>
            {i18next.t("general:Register asset")}
          </Button>
        </Space>
        <Table rowKey="id" size="small" loading={this.state.loading} columns={columns} dataSource={this.state.assets} pagination={false} />
      </Space>
    );
  }

  // ---- Aba ATRIBUIÇÕES ----
  renderAssignments() {
    const columns = [
      {title: i18next.t("general:Subject"), key: "subject",
        render: (_, r) => <span><Tag>{r.subject_type}</Tag>{r.subject_type === "group"
          ? <span>{this.groupLabel(r.subject_id)}</span>
          : <span style={{fontFamily: "monospace"}}>{short(r.subject_id)}</span>}</span>},
      {title: i18next.t("general:Relation"), dataIndex: "relation", key: "relation",
        render: (rel) => <Tag color={RELATION_COLOR[rel] || "default"}>{rel}</Tag>},
      {title: i18next.t("general:Asset"), key: "object",
        render: (_, r) => <span style={{fontFamily: "monospace"}}>{short(r.object_id)}</span>},
    ];
    const isGroup = this.state.asgSubjectType === "group";
    return (
      <Space direction="vertical" size="middle" style={{width: "100%"}}>
        <Space wrap>
          <Select style={{width: 130}} value={this.state.asgSubjectType}
            onChange={(v) => this.setState({asgSubjectType: v, asgSubjectId: ""})}
            options={[{value: "membership", label: i18next.t("general:Membership")}, {value: "group", label: i18next.t("general:Group")}]} />
          {isGroup ? (
            <Select showSearch style={{width: 300}} placeholder={i18next.t("general:Group")} optionFilterProp="label"
              value={this.state.asgSubjectId || undefined} options={this.groupOptions()}
              onChange={(v) => this.setState({asgSubjectId: v})} />
          ) : (
            <Select showSearch style={{width: 300}} placeholder={i18next.t("general:Membership")} optionFilterProp="label"
              value={this.state.asgSubjectId || undefined} options={this.membershipOptions()}
              onChange={(v) => this.setState({asgSubjectId: v})} />
          )}
          <Select style={{width: 140}} value={this.state.asgRelation}
            onChange={(v) => this.setState({asgRelation: v})}
            options={[{value: "operator", label: "operator"}, {value: "auditor", label: "auditor"}]} />
          <Select showSearch style={{width: 260}} placeholder={i18next.t("general:Asset")} optionFilterProp="label"
            value={this.state.asgObjectId || undefined} options={this.assetOptions()}
            onChange={(v) => this.setState({asgObjectId: v})} />
          <Button type="primary" disabled={!this.state.asgSubjectId || !this.state.asgObjectId}
            onClick={() => this.create(ControlPlane.createAccessAssignment,
              {subject_type: this.state.asgSubjectType, subject_id: this.state.asgSubjectId,
                relation: this.state.asgRelation, object_type: "asset", object_id: this.state.asgObjectId},
              () => this.setState({asgSubjectId: "", asgObjectId: ""}))}>
            {i18next.t("general:Grant access")}
          </Button>
        </Space>
        <Table rowKey="id" size="small" loading={this.state.loading} columns={columns} dataSource={this.state.assignments} pagination={false} />
      </Space>
    );
  }

  // ---- Aba VÍNCULOS DE GRUPO ----
  renderGroups() {
    const columns = [
      {title: i18next.t("general:Group"), dataIndex: "group_id", key: "group_id",
        render: (id) => <span>{this.groupLabel(id)}</span>},
      {title: i18next.t("general:Membership"), dataIndex: "membership_id", key: "membership_id",
        render: (id) => <span style={{fontFamily: "monospace"}}>{short(id)}</span>},
    ];
    return (
      <Space direction="vertical" size="middle" style={{width: "100%"}}>
        <Space wrap>
          <Select showSearch style={{width: 300}} placeholder={i18next.t("general:Group")} optionFilterProp="label"
            value={this.state.gmGroupId || undefined} options={this.groupOptions()}
            onChange={(v) => this.setState({gmGroupId: v})} />
          <Select showSearch style={{width: 300}} placeholder={i18next.t("general:Membership")} optionFilterProp="label"
            value={this.state.gmMembershipId || undefined} options={this.membershipOptions()}
            onChange={(v) => this.setState({gmMembershipId: v})} />
          <Button type="primary" disabled={!this.state.gmGroupId || !this.state.gmMembershipId}
            onClick={() => this.create(ControlPlane.createGroupMembership,
              {group_id: this.state.gmGroupId, membership_id: this.state.gmMembershipId},
              () => this.setState({gmGroupId: "", gmMembershipId: ""}))}>
            {i18next.t("general:Bind to group")}
          </Button>
        </Space>
        <Table rowKey="id" size="small" loading={this.state.loading} columns={columns} dataSource={this.state.groups} pagination={false} />
      </Space>
    );
  }

  // ---- Aba CATÁLOGO DE GRUPOS ----
  renderCatalog() {
    const columns = [
      {title: i18next.t("general:Name"), dataIndex: "name", key: "name"},
      {title: i18next.t("general:Display name"), dataIndex: "display_name", key: "display_name"},
      {title: "ID", dataIndex: "id", key: "id", render: (id) => <span style={{fontFamily: "monospace"}}>{short(id)}</span>},
    ];
    return (
      <Space direction="vertical" size="middle" style={{width: "100%"}}>
        <Space wrap>
          <Input style={{width: 200}} placeholder={i18next.t("general:Name")} value={this.state.grpName}
            onChange={(e) => this.setState({grpName: e.target.value})} />
          <Input style={{width: 240}} placeholder={i18next.t("general:Display name")} value={this.state.grpDisplay}
            onChange={(e) => this.setState({grpDisplay: e.target.value})} />
          <Button type="primary" disabled={!this.state.grpName}
            onClick={() => this.create(ControlPlane.createAccessGroup,
              {name: this.state.grpName, display_name: this.state.grpDisplay},
              () => this.setState({grpName: "", grpDisplay: ""}))}>
            {i18next.t("general:Create group")}
          </Button>
        </Space>
        <Table rowKey="id" size="small" loading={this.state.loading} columns={columns} dataSource={this.state.catalog} pagination={false} />
      </Space>
    );
  }

  render() {
    const items = [
      {key: "assets", label: i18next.t("general:Assets"), children: this.renderAssets()},
      {key: "catalog", label: i18next.t("general:Groups"), children: this.renderCatalog()},
      {key: "assignments", label: i18next.t("general:Access assignments"), children: this.renderAssignments()},
      {key: "groups", label: i18next.t("general:Group bindings"), children: this.renderGroups()},
    ];
    return (
      <div style={{padding: "16px", maxWidth: "1080px", margin: "0 auto"}}>
        <Card title={i18next.t("general:Access management")}>
          <Tabs items={items} />
        </Card>
      </div>
    );
  }
}

export default AccessManagementPage;
