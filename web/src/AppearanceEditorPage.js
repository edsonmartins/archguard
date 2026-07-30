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
// (pacote 008, T-022). Tela NOSSA e ADITIVA — NÃO refatora a ApplicationEditPage herdada
// (1855 linhas; reescrever encareceria cherry-pick do upstream, CLAUDE.md §7/§8). A prévia
// ao vivo reusa <LoginPage>/<SignupPage> (mesmo mecanismo do renderSignupSigninPreview) sob
// um <ConfigProvider> alimentado pelo themeData. O painel usa controles COMPACTOS (cor/raio/
// tema/logo/fundo/CSS) com fiação direta — evita o <ThemeEditor> herdado, que é 800px fixos e
// estourava o layout. Grava os MESMOS campos via updateApplication. "Edição avançada" abre a
// tela herdada.
import React from "react";
import {Button, Card, Col, ConfigProvider, Divider, Input, InputNumber, Row, Segmented, Space, Spin, message} from "antd";
import {StyleProvider, legacyLogicalPropertiesTransformer} from "@ant-design/cssinjs";
import {SettingOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as ApplicationBackend from "./backend/ApplicationBackend";
import * as Setting from "./Setting";
import * as Conf from "./Conf";
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

  // Fiação direta do tema: mescla no themeData atual e marca habilitado (senão a página de
  // login real ignora a cor). O preview reage na hora porque lê application.themeData.
  updateTheme(key, value) {
    const base = this.state.application.themeData ?? {...Conf.ThemeDefault};
    this.updateField("themeData", {...base, [key]: value, isEnabled: true});
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

    // O antd não re-tematiza um <LoginPage> já montado quando só o token muda (estilo em
    // cache). A key força REMONTAR o preview a cada mudança de tema, então o botão/links
    // pegam a cor nova na hora.
    const previewKey = `${this.state.page}-${themeData.colorPrimary}-${themeData.borderRadius}-${themeData.themeType}`;

    let previewComponent;
    if (this.state.page === "signup" && Setting.isPasswordEnabled(application)) {
      previewComponent = <SignupPage key={previewKey} application={application} preview="auto" />;
    } else {
      previewComponent = <LoginPage key={previewKey} type={"login"} mode={this.state.page === "signup" ? "signup" : "signin"} application={application} preview="auto" />;
    }

    return (
      // O root do app usa StyleProvider hashPriority="high", então o CSS do botão vem com
      // especificidade alta e o token do meu ConfigProvider aninhado (só) perdia. Envolver o
      // preview no PRÓPRIO StyleProvider high dá a mesma especificidade — e, injetado depois,
      // a cor escolhida vence. Junto com a key (remonta), o preview reflete a cor na hora.
      <StyleProvider hashPriority="high" transformers={[legacyLogicalPropertiesTransformer]}>
        <ConfigProvider theme={{
          token: {
            colorPrimary: themeData.colorPrimary,
            colorInfo: themeData.colorPrimary,
            colorLink: themeData.colorPrimary,
            borderRadius: themeData.borderRadius,
          },
        }}>
          {/* moldura da prévia: rola por dentro (não empurra a página) + máscara sem interação */}
          <div style={{position: "relative", border: "1px solid rgb(217,217,217)", boxShadow: "4px 4px 8px rgba(0,0,0,0.15)", overflow: "auto", height: "640px", maxWidth: "100%"}}>
            <div className="loginBackground" style={{backgroundImage: `url(${application.formBackgroundUrl})`}}>
              {previewComponent}
            </div>
            <div style={{position: "absolute", top: 0, left: 0, height: "100%", width: "100%", background: "rgba(0,0,0,0.01)", zIndex: 10}} />
          </div>
        </ConfigProvider>
      </StyleProvider>
    );
  }

  renderPanel() {
    const application = this.state.application;
    const themeData = application.themeData ?? Conf.ThemeDefault;
    const color = themeData.colorPrimary || "#1677FF";
    return (
      <Space direction="vertical" size="middle" style={{width: "100%"}}>
        <Divider orientation="left" style={{margin: "4px 0"}}>{i18next.t("general:Brand")}</Divider>
        <div>
          <div style={{marginBottom: 4}}>{i18next.t("general:Logo URL")}</div>
          <Input value={application.logo} onChange={(e) => this.updateField("logo", e.target.value)} placeholder="https://.../logo.png" />
          {application.logo ? <img src={application.logo} alt="logo" style={{maxHeight: 44, marginTop: 8, maxWidth: "100%"}} /> : null}
        </div>
        <div>
          <div style={{marginBottom: 4}}>{i18next.t("theme:Primary color")}</div>
          <Space>
            <input type="color" value={/^#[0-9a-fA-F]{6}$/.test(color) ? color : "#1677FF"} onChange={(e) => this.updateTheme("colorPrimary", e.target.value)} style={{width: 44, height: 32, padding: 0, border: "1px solid #d9d9d9", borderRadius: 4, cursor: "pointer"}} />
            <Input value={color} onChange={(e) => this.updateTheme("colorPrimary", e.target.value)} style={{width: 120}} />
          </Space>
        </div>
        <Row gutter={16}>
          <Col span={12}>
            <div style={{marginBottom: 4}}>{i18next.t("theme:Border radius")}</div>
            <InputNumber min={0} max={24} value={themeData.borderRadius} onChange={(v) => this.updateTheme("borderRadius", v ?? 0)} style={{width: "100%"}} />
          </Col>
          <Col span={12}>
            <div style={{marginBottom: 4}}>{i18next.t("theme:Theme")}</div>
            <Segmented
              block
              value={themeData.themeType === "dark" ? "dark" : "default"}
              onChange={(v) => this.updateTheme("themeType", v)}
              options={[{label: i18next.t("theme:Light"), value: "default"}, {label: i18next.t("theme:Dark"), value: "dark"}]}
            />
          </Col>
        </Row>

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
      <div style={{padding: "16px", maxWidth: "100%", overflowX: "hidden"}}>
        <Row justify="space-between" align="middle" gutter={[8, 8]} style={{marginBottom: "12px"}}>
          <Col>
            <h2 style={{margin: 0}}>{i18next.t("general:Appearance")} — {application.displayName || name}</h2>
            <div style={{color: "rgba(0,0,0,0.45)"}}>{i18next.t("general:Customize the hosted login page for this application")}</div>
          </Col>
          <Col>
            <Space>
              {/* #ui-customization: a tela herdada lê a aba do hash (window.location.hash), então
                  abre direto na aba de personalização, não na "Básico". */}
              <Button icon={<SettingOutlined />} onClick={() => this.props.history.push(`/applications/${owner}/${name}#ui-customization`)}>
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

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={14}>{this.renderPreview()}</Col>
          <Col xs={24} lg={10}><Card size="small">{this.renderPanel()}</Card></Col>
        </Row>
      </div>
    );
  }
}

export default AppearanceEditorPage;
