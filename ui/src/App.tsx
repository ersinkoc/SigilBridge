import { lazy, Suspense } from "react";
import { Navigate, Route, Routes } from "react-router-dom";

import { AppShell } from "./components/layout/AppShell";
import { Skeleton } from "./components/common/State";

const AuditRoute = lazy(() => import("./routes/audit").then((module) => ({ default: module.AuditRoute })));
const BudgetsRoute = lazy(() => import("./routes/budgets").then((module) => ({ default: module.BudgetsRoute })));
const CredentialsRoute = lazy(() => import("./routes/credentials").then((module) => ({ default: module.CredentialsRoute })));
const CredentialsApiKeyNewRoute = lazy(() => import("./routes/credentials-api-key-new").then((module) => ({ default: module.CredentialsApiKeyNewRoute })));
const CredentialsCliRoute = lazy(() => import("./routes/credentials-cli").then((module) => ({ default: module.CredentialsCliRoute })));
const CredentialsOAuthNewRoute = lazy(() => import("./routes/credentials-oauth-new").then((module) => ({ default: module.CredentialsOAuthNewRoute })));
const CredentialsSessionsNewRoute = lazy(() => import("./routes/credentials-sessions-new").then((module) => ({ default: module.CredentialsSessionsNewRoute })));
const EventsRoute = lazy(() => import("./routes/events").then((module) => ({ default: module.EventsRoute })));
const HealthRoute = lazy(() => import("./routes/health").then((module) => ({ default: module.HealthRoute })));
const HomeRoute = lazy(() => import("./routes/index").then((module) => ({ default: module.HomeRoute })));
const KeysRoute = lazy(() => import("./routes/keys").then((module) => ({ default: module.KeysRoute })));
const KeysDetailRoute = lazy(() => import("./routes/keys-detail").then((module) => ({ default: module.KeysDetailRoute })));
const KeysNewRoute = lazy(() => import("./routes/keys-new").then((module) => ({ default: module.KeysNewRoute })));
const LoginRoute = lazy(() => import("./routes/login").then((module) => ({ default: module.LoginRoute })));
const ModelsRoute = lazy(() => import("./routes/models").then((module) => ({ default: module.ModelsRoute })));
const PoolEditRoute = lazy(() => import("./routes/pool-edit").then((module) => ({ default: module.PoolEditRoute })));
const PoolsRoute = lazy(() => import("./routes/pools").then((module) => ({ default: module.PoolsRoute })));
const SettingsRoute = lazy(() => import("./routes/settings").then((module) => ({ default: module.SettingsRoute })));
const SettingsOAuthProvidersRoute = lazy(() => import("./routes/settings-oauth-providers").then((module) => ({ default: module.SettingsOAuthProvidersRoute })));
const SettingsPoolsRawRoute = lazy(() => import("./routes/settings-pools-raw").then((module) => ({ default: module.SettingsPoolsRawRoute })));
const SetupRoute = lazy(() => import("./routes/setup").then((module) => ({ default: module.SetupRoute })));

export default function App() {
  return (
    <Suspense fallback={<Skeleton />}>
      <Routes>
        <Route path="/login" element={<LoginRoute />} />
        <Route element={<AppShell />}>
          <Route index element={<HomeRoute />} />
          <Route path="setup" element={<SetupRoute />} />
          <Route path="keys" element={<KeysRoute />} />
          <Route path="keys/new" element={<KeysNewRoute />} />
          <Route path="keys/:id" element={<KeysDetailRoute />} />
          <Route path="models" element={<ModelsRoute />} />
          <Route path="pools" element={<PoolsRoute />} />
          <Route path="pools/:id" element={<PoolEditRoute />} />
          <Route path="credentials" element={<CredentialsRoute />} />
          <Route path="credentials/api-key/new" element={<CredentialsApiKeyNewRoute />} />
          <Route path="credentials/oauth/new" element={<CredentialsOAuthNewRoute />} />
          <Route path="credentials/sessions/new" element={<CredentialsSessionsNewRoute />} />
          <Route path="credentials/cli" element={<CredentialsCliRoute />} />
          <Route path="audit" element={<AuditRoute />} />
          <Route path="budgets" element={<BudgetsRoute />} />
          <Route path="health" element={<HealthRoute />} />
          <Route path="events" element={<EventsRoute />} />
          <Route path="settings" element={<SettingsRoute />} />
          <Route path="settings/oauth-providers" element={<SettingsOAuthProvidersRoute />} />
          <Route path="settings/pools-raw" element={<SettingsPoolsRawRoute />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  );
}
