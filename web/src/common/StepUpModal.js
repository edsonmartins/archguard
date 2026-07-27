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

// StepUpModal: interceptor global de step-up do console (pacote 008, T-005/T-005b).
// Registra no ControlPlaneBackend um handler que, quando uma operação responde 401 RFC 9470
// (garantia insuficiente), conduz o desafio e resolve `true` no sucesso — o cpRequest então
// REPETE a operação original, preservando o estado (o formulário do chamador nunca é perdido:
// ele só aguarda a promessa). Cancelar resolve `false`.
//
// DOIS fatores: se o desafio exige phishing-resistant (`needsPhishingResistant` — operação
// L3), conduz WebAuthn (`navigator.credentials.get`), o ÚNICO que satisfaz L3; senão, TOTP
// (AAL2). A escolha vem do próprio desafio, não de configuração.
import React, {useEffect, useRef, useState} from "react";
import {Input, Modal, message} from "antd";
import i18next from "i18next";
import * as ControlPlane from "../backend/ControlPlaneBackend";
import {webAuthnBufferDecode, webAuthnBufferEncode} from "../backend/UserWebauthnBackend";

function StepUpModal() {
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState("totp"); // "totp" | "webauthn"
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const resolverRef = useRef(null);

  useEffect(() => {
    ControlPlane.setStepUpHandler((challenge) => new Promise((resolve) => {
      resolverRef.current = resolve;
      setMode(challenge && challenge.needsPhishingResistant ? "webauthn" : "totp");
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

  const submitTotp = () => {
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

  const submitWebauthn = async() => {
    if (!navigator.credentials || !navigator.credentials.get) {
      message.error(i18next.t("general:WebAuthn is not available in this browser"));
      return;
    }
    setLoading(true);
    try {
      const options = await ControlPlane.stepupWebauthnBegin();
      const pk = options.publicKey;
      pk.challenge = webAuthnBufferDecode(pk.challenge);
      if (pk.allowCredentials) {
        pk.allowCredentials = pk.allowCredentials.map((c) => ({...c, id: webAuthnBufferDecode(c.id)}));
      }
      const cred = await navigator.credentials.get({publicKey: pk});
      const assertion = {
        id: cred.id,
        rawId: webAuthnBufferEncode(cred.rawId),
        type: cred.type,
        response: {
          authenticatorData: webAuthnBufferEncode(cred.response.authenticatorData),
          clientDataJSON: webAuthnBufferEncode(cred.response.clientDataJSON),
          signature: webAuthnBufferEncode(cred.response.signature),
          userHandle: cred.response.userHandle ? webAuthnBufferEncode(cred.response.userHandle) : "",
        },
      };
      await ControlPlane.stepupWebauthnFinish(assertion);
      message.success(i18next.t("general:Step-up completed"));
      settle(true);
    } catch (e) {
      // Inclui o usuário cancelar o prompt do autenticador ou a asserção ser recusada.
      setLoading(false);
      message.error(i18next.t("general:Authentication failed or cancelled"));
    }
  };

  const isWebauthn = mode === "webauthn";
  return (
    <Modal
      open={open}
      title={isWebauthn ? i18next.t("general:Security key required") : i18next.t("general:Additional confirmation required")}
      okText={isWebauthn ? i18next.t("general:Authenticate") : i18next.t("general:Confirm")}
      cancelText={i18next.t("general:Cancel")}
      confirmLoading={loading}
      onOk={isWebauthn ? submitWebauthn : submitTotp}
      onCancel={() => settle(false)}
      maskClosable={false}
      destroyOnClose={true}
    >
      {isWebauthn ? (
        <p style={{marginBottom: 12}}>{i18next.t("general:This operation requires a phishing-resistant factor (security key). TOTP is not sufficient.")}</p>
      ) : (
        <>
          <p style={{marginBottom: 12}}>{i18next.t("general:This operation requires a stronger factor")}</p>
          <Input
            value={code}
            onChange={(e) => setCode(e.target.value.trim())}
            onPressEnter={submitTotp}
            placeholder={i18next.t("general:Authenticator code")}
            maxLength={8}
            autoFocus
          />
        </>
      )}
    </Modal>
  );
}

export default StepUpModal;
