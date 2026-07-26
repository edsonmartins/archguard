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

// TenantSelect: seletor do TENANT ATIVO do plano de controle (pacote 008, T-004).
// Distinto do OrganizationSelect herdado (filtro de admin em localStorage): mostra o
// contexto de membership do /api/v1 com distinção visual inequívoca — operar no tenant
// errado é a classe de erro humano mais cara em PAM. A troca reemite o token (a sessão
// por cookie passa a apontar para o novo tenant) e recarrega o contexto; um destino
// mais restritivo pede step-up (tratamento mínimo aqui; o interceptor global é T-005).
import React, {useEffect, useState} from "react";
import {Dropdown, message} from "antd";
import {ApartmentOutlined, DownOutlined} from "TablerIcons";
import i18next from "i18next";
import * as ControlPlane from "../../backend/ControlPlaneBackend";

function TenantSelect() {
  const [tenants, setTenants] = useState([]);
  const [active, setActive] = useState(null);
  const [switching, setSwitching] = useState(false);

  useEffect(() => {
    let alive = true;
    ControlPlane.getTenants()
      .then((res) => {
        if (!alive) {return;}
        const list = (res && res.tenants) ? res.tenants : [];
        setTenants(list);
        setActive(list.find((t) => t.active) || null);
      })
      .catch(() => {
        // fail-closed: sem contexto de tenant, não exibe nada enganoso.
      });
    return () => {alive = false;};
  }, []);

  // Sem tenant ativo do /api/v1 (ex.: sessão herdada sem contexto): não renderiza.
  if (!active) {
    return null;
  }

  const others = tenants.filter((t) => !t.active && t.status === "active");

  const doSwitch = (organizationId) => {
    setSwitching(true);
    ControlPlane.switchTenant(organizationId)
      .then(() => {
        message.success(i18next.t("general:Tenant switched"));
        // Recarrega para reidratar o contexto (/session) no novo tenant.
        window.location.reload();
      })
      .catch((err) => {
        setSwitching(false);
        const status = err && err.status;
        if (status === 401) {
          // Destino mais restritivo — step-up necessário (RFC 9470). Tratamento
          // mínimo; o fluxo transparente (desafio + retomada) é a T-005.
          message.warning(i18next.t("general:Step-up required to switch tenant"));
        } else if (status === 403) {
          message.error(i18next.t("general:Not a member of the target tenant"));
        } else {
          message.error(i18next.t("general:Failed to switch tenant"));
        }
      });
  };

  const pill = (
    <span
      className="tenant-pill"
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 6,
        padding: "3px 12px",
        borderRadius: 8,
        border: "1.5px solid #16a34a",
        background: "#f0fdf4",
        color: "#15803d",
        fontWeight: 600,
        lineHeight: 1.4,
        cursor: others.length > 0 && !switching ? "pointer" : "default",
        maxWidth: 280,
      }}
      title={active.display_name}
    >
      <ApartmentOutlined style={{fontSize: 16}} />
      <span style={{color: "#525252", fontWeight: 500}}>{i18next.t("general:Tenant")}:</span>
      <span style={{overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap"}}>
        {active.display_name}
      </span>
      {others.length > 0 && <DownOutlined style={{fontSize: 12, marginLeft: 2}} />}
    </span>
  );

  if (others.length === 0) {
    return pill; // membership único: selo fixo, sem troca.
  }

  const items = others.map((t) => ({key: t.organization_id, label: t.display_name}));
  return (
    <Dropdown
      menu={{items, onClick: ({key}) => doSwitch(key)}}
      disabled={switching}
      trigger={["click"]}
    >
      {pill}
    </Dropdown>
  );
}

export default TenantSelect;
