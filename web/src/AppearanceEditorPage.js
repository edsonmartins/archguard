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

// AppearanceEditorPage: editor AMIGÁVEL da aparência da página de login de UMA aplicação
// (pacote 008, T-022). É uma tela NOSSA e ADITIVA — NÃO refatora a ApplicationEditPage
// herdada (1855 linhas; reescrever encareceria todo cherry-pick do upstream, CLAUDE.md §7/§8).
// Compõe peças que já existem: a prévia ao vivo reusa <LoginPage>/<SignupPage> (mesmo
// mecanismo do renderSignupSigninPreview herdado) e as cores reusam <ThemeEditor>. Grava os
// MESMOS campos via updateApplication (logo, themeData, formCss, formBackgroundUrl, ...). O
// botão "Edição avançada" abre a tela herdada com todos os campos.
import React from "react";
import {Button, Card, Col, ConfigProvider, Divider, Input, Row, Segmented, Space, Spin, message} from "antd";
import {SettingOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as ApplicationBackend from "./backend/ApplicationBackend";
import * as Setting from "./Setting";
import * as Conf from "./Conf";
import ThemeEditor from "./common/theme/ThemeEditor";
import LoginPage from "./auth/LoginPage";
import SignupPage from "./auth/SignupPage";

class AppearanceEditorPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      owner: props.match.params.organizationName,
      name: props.match.params.applicationName,
      application: null,
      page: "login",
      loading: true,
    };
  }

  componentDidMount() {
    this.getApplication();
  }

  getApplication() {
    ApplicationBackend.getApplication(this.state.owner, this.state.name)
      .then((res) => {
        if (res.status === "error" || !res.data) {
          message.error(res.msg ?? i18next.t("general:Failed to get"));
          this.setState({loading: false});
          return;
        }
        this.setState({application: res.data, loading: false});
      });
  }

  updateField(field, value) {
    this.setState({application: {...this.state.application, [field]: value}});
  }

  // O ThemeEditor emite (changedValues, allValues); persistimos o tema completo e o marcamos
  // habilitado, senão a página de login real ignora a cor.
  onThemeChange(_, themeData) {
    this.updateField("themeData", {...themeData, isEnabled: true});
  }

  save() {
    ApplicationBackend.updateApplication(this.state.owner, this.state.name, this.state.application)
      .then((res) => {
        if (res.status === "ok") {
          message.success(i18next.t("general:Successfully saved"));
        } else {
          message.error(`${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      });
  }

  renderPreview() {
    const application = this.state.application;
    const themeData = application.themeData ?? Conf.ThemeDefault;

    let previewComponent;
    if (this.state.page === "signup" && Setting.isPasswordEnabled(application)) {
      previewComponent = <SignupPage application={application} preview="auto" />;
    } else {
      previewComponent = <LoginPage type={"login"} mode={this.state.page === "signup" ? "signup" : "signin"} application={application} preview="auto" />;
    }

    return (
      <ConfigProvider theme={{
        token: {
          colorPrimary: themeData.colorPrimary,
          colorInfo: themeData.colorPrimary,
          borderRadius: themeData.borderRadius,
        },
      }}>
        {/* mesma moldura da prévia herdada: fundo do formulário + máscara que impede interação */}
        <div style={{position: "relative", border: "1px solid rgb(217,217,217)", boxShadow: "6px 6px 5px #888888", overflow: "auto", minHeight: "620px"}}>
          <div className="loginBackground" style={{backgroundImage: `url(${application.formBackgroundUrl})`, overflow: "auto"}}>
            {previewComponent}
          </div>
          <div style={{position: "absolute", top: 0, left: 0, height: "100%", width: "100%", background: "rgba(0,0,0,0.02)", zIndex: 10}} />
        </div>
      </ConfigProvider>
    );
  }

  renderPanel() {
    const application = this.state.application;
    return (
      <Space direction="vertical" size="middle" style={{width: "100%"}}>
        <Divider orientation="left" style={{margin: "4px 0"}}>{i18next.t("general:Brand")}</Divider>
        <div>
          <div style={{marginBottom: 4}}>{i18next.t("general:Logo URL")}</div>
          <Input value={application.logo} onChange={(e) => this.updateField("logo", e.target.value)} placeholder="https://.../logo.png" />
          {application.logo ? <img src={application.logo} alt="logo" style={{maxHeight: 48, marginTop: 8}} /> : null}
        </div>
        {/* cores/raio/tema reusam o ThemeEditor herdado */}
        <ThemeEditor themeData={application.themeData ?? Conf.ThemeDefault} onThemeChange={(changed, all) => this.onThemeChange(changed, all)} />

        <Divider orientation="left" style={{margin: "4px 0"}}>{i18next.t("general:Background")}</Divider>
        <div>
          <div style={{marginBottom: 4}}>{i18next.t("application:Background URL")}</div>
          <Input value={application.formBackgroundUrl} onChange={(e) => this.updateField("formBackgroundUrl", e.target.value)} placeholder="https://.../background.png" />
        </div>

        <Divider orientation="left" style={{margin: "4px 0"}}>{i18next.t("general:Custom CSS")}</Divider>
        <Input.TextArea rows={8} value={application.formCss} onChange={(e) => this.updateField("formCss", e.target.value)} placeholder=".login-panel { ... }" style={{fontFamily: "monospace"}} />
      </Space>
    );
  }

  render() {
    if (this.state.loading || this.state.application === null) {
      return <div style={{padding: "48px", textAlign: "center"}}><Spin size="large" /></div>;
    }
    const {owner, name, application} = this.state;
    return (
      <div style={{padding: "16px"}}>
        <Row justify="space-between" align="middle" style={{marginBottom: "12px"}}>
          <Col>
            <h2 style={{margin: 0}}>{i18next.t("general:Appearance")} — {application.displayName || name}</h2>
            <div style={{color: "rgba(0,0,0,0.45)"}}>{i18next.t("general:Customize the hosted login page for this application")}</div>
          </Col>
          <Col>
            <Space>
              <Button icon={<SettingOutlined />} onClick={() => this.props.history.push(`/applications/${owner}/${name}`)}>
                {i18next.t("general:Advanced edit")}
              </Button>
              <Button type="primary" onClick={() => this.save()}>{i18next.t("general:Save")}</Button>
            </Space>
          </Col>
        </Row>

        <Segmented
          style={{marginBottom: "12px"}}
          value={this.state.page}
          onChange={(page) => this.setState({page})}
          options={[
            {label: i18next.t("general:Login page"), value: "login"},
            {label: i18next.t("general:Signup page"), value: "signup"},
          ]}
        />

        <Row gutter={16}>
          <Col span={14}>{this.renderPreview()}</Col>
          <Col span={10}><Card size="small">{this.renderPanel()}</Card></Col>
        </Row>
      </div>
    );
  }
}

export default AppearanceEditorPage;
