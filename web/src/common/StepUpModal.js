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

// StepUpModal: interceptor global de step-up do console (pacote 008, T-005). Registra
// no ControlPlaneBackend um handler que, quando uma operação responde 401 RFC 9470
// (garantia insuficiente), conduz o desafio TOTP e resolve `true` no sucesso — o
// cpRequest então REPETE a operação original, preservando o estado (o formulário do
// chamador nunca é perdido: ele só aguarda a promessa). Cancelar resolve `false`.
// Fator: TOTP (o exposto no /api/v1/stepup/totp); WebAuthn é refinamento posterior.
import React, {useEffect, useRef, useState} from "react";
import {Input, Modal, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "../backend/ControlPlaneBackend";

function StepUpModal() {
  const [open, setOpen] = useState(false);
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const resolverRef = useRef(null);

  useEffect(() => {
    ControlPlane.setStepUpHandler(() => new Promise((resolve) => {
      resolverRef.current = resolve;
      setCode("");
      setLoading(false);
      setOpen(true);
    }));
    return () => ControlPlane.setStepUpHandler(null);
  }, []);

  const settle = (ok) => {
    setOpen(false);
    setLoading(false);
    const resolve = resolverRef.current;
    resolverRef.current = null;
    if (resolve) {resolve(ok);}
  };

  const submit = () => {
    if (!code) {return;}
    setLoading(true);
    ControlPlane.stepupTotp({code})
      .then(() => {
        message.success(i18next.t("general:Step-up completed"));
        settle(true);
      })
      .catch(() => {
        setLoading(false);
        message.error(i18next.t("general:Invalid code"));
      });
  };

  return (
    <Modal
      open={open}
      title={i18next.t("general:Additional confirmation required")}
      okText={i18next.t("general:Confirm")}
      cancelText={i18next.t("general:Cancel")}
      confirmLoading={loading}
      onOk={submit}
      onCancel={() => settle(false)}
      maskClosable={false}
      destroyOnClose={true}
    >
      <p style={{marginBottom: 12}}>{i18next.t("general:This operation requires a stronger factor")}</p>
      <Input
        value={code}
        onChange={(e) => setCode(e.target.value.trim())}
        onPressEnter={submit}
        placeholder={i18next.t("general:Authenticator code")}
        maxLength={8}
        autoFocus
      />
    </Modal>
  );
}

export default StepUpModal;
