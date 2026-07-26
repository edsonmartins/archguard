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

// Ponte (shim): reexporta os ícones que o console declara com os MESMOS nomes do
// @ant-design/icons, mas renderizando a webfont Tabler (`<i class="ti ti-...">`).
// Assim os arquivos só trocam o import ("@ant-design/icons" -> "TablerIcons") e o
// JSX (<HomeOutlined/>) permanece igual. Os ícones INTERNOS do antd (setas de
// ordenação, carets, X) continuam vindo do antd — só os declarados por nós mudam.
import React from "react";

// Constrói um componente de ícone Tabler compatível com a API dos ícones antd
// (aceita className, style, spin, rotate, onClick, etc.).
const mk = (tablerName) => {
  const Icon = React.forwardRef(function TablerIcon(
    {className = "", style, spin, rotate, twoToneColor, ...rest}, ref) {
    const cls = `ti ti-${tablerName}${spin ? " ti-spin" : ""}${className ? " " + className : ""}`;
    const st = rotate ? {transform: `rotate(${rotate}deg)`, ...style} : style;
    return <i ref={ref} aria-hidden="true" className={cls} style={st} {...rest} />;
  });
  Icon.displayName = tablerName;
  return Icon;
};

// setTwoToneColor é usado pelo antd para tingir ícones "TwoTone"; a webfont é
// monocromática, então é um no-op.
export const setTwoToneColor = () => {};

export const ApartmentOutlined = mk("sitemap");
export const AppstoreOutlined = mk("layout-grid");
export const ArrowLeftOutlined = mk("arrow-left");
export const ArrowUpOutlined = mk("arrow-up");
export const BarsOutlined = mk("menu-2");
export const CameraOutlined = mk("camera");
export const CheckCircleOutlined = mk("circle-check");
export const CheckCircleTwoTone = mk("circle-check");
export const CheckOutlined = mk("check");
export const ClockCircleOutlined = mk("clock");
export const CloseCircleOutlined = mk("circle-x");
export const CloseCircleTwoTone = mk("circle-x");
export const CopyOutlined = mk("copy");
export const DeleteOutlined = mk("trash");
export const DeploymentUnitOutlined = mk("hierarchy-2");
export const DownOutlined = mk("chevron-down");
export const DownloadOutlined = mk("download");
export const EditOutlined = mk("edit");
export const ExclamationCircleFilled = mk("alert-circle");
export const ExclamationCircleOutlined = mk("alert-circle");
export const EyeInvisibleOutlined = mk("eye-off");
export const EyeTwoTone = mk("eye");
export const FileTextOutlined = mk("file-text");
export const FullscreenExitOutlined = mk("arrows-minimize");
export const FullscreenOutlined = mk("arrows-maximize");
export const GithubOutlined = mk("brand-github");
export const GlobalOutlined = mk("world");
export const HolderOutlined = mk("grip-vertical");
export const HomeOutlined = mk("home");
export const InfoCircleFilled = mk("info-circle");
export const InfoCircleTwoTone = mk("info-circle");
export const KeyOutlined = mk("key");
export const LinkOutlined = mk("link");
export const LockOutlined = mk("lock");
export const LogoutOutlined = mk("logout");
export const MailOutlined = mk("mail");
export const MenuFoldOutlined = mk("layout-sidebar-left-collapse");
export const MenuUnfoldOutlined = mk("layout-sidebar-left-expand");
export const MinusCircleOutlined = mk("circle-minus");
export const MinusOutlined = mk("minus");
export const PhoneOutlined = mk("phone");
export const PlusOutlined = mk("plus");
export const QuestionCircleOutlined = mk("help-circle");
export const SafetyCertificateOutlined = mk("shield-check");
export const SafetyOutlined = mk("shield");
export const SearchOutlined = mk("search");
export const SendOutlined = mk("send");
export const SettingOutlined = mk("settings");
export const ShareAltOutlined = mk("share");
export const ShoppingCartOutlined = mk("shopping-cart");
export const SolutionOutlined = mk("user-check");
export const SyncOutlined = mk("refresh");
export const TeamOutlined = mk("users");
export const UpOutlined = mk("chevron-up");
export const UploadOutlined = mk("upload");
export const UserOutlined = mk("user");
export const UsergroupAddOutlined = mk("users-plus");
export const WalletOutlined = mk("wallet");
