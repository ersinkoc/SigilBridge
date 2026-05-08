import { useQuery } from "@tanstack/react-query";

import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { BudgetResponse, UsageResponse } from "../types/api";

export function BudgetsRoute() {
  const budgets = useQuery({ queryKey: ["budgets"], queryFn: () => api<BudgetResponse>("/admin/v1/budgets") });
  const usage = useQuery({ queryKey: ["usage"], queryFn: () => api<UsageResponse>("/admin/v1/usage") });
  const usageRows = usage.data?.items ?? [];
  const dailyUsedCents = budgets.data?.daily_used_cents ?? 0;
  const monthlyUsedCents = budgets.data?.monthly_used_cents ?? 0;
  const dailyCents = budgets.data?.daily_cents ?? 0;
  const monthlyCents = budgets.data?.monthly_cents ?? 0;

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Budgets</h2>
        <p>Track spend guardrails before traffic fans out across providers.</p>
      </div>
      <div className="page-grid">
        <Card className="budget-card">
          <span className="metric-label">Daily budget</span>
          <strong className="metric-value">{formatDollars(dailyCents)}</strong>
          <div className="budget-meter">
            <span style={{ width: percent(dailyUsedCents, dailyCents) }} />
          </div>
          <em>{formatDollars(dailyUsedCents)} used today</em>
        </Card>
        <Card className="budget-card">
          <span className="metric-label">Monthly budget</span>
          <strong className="metric-value">{formatDollars(monthlyCents)}</strong>
          <div className="budget-meter">
            <span style={{ width: percent(monthlyUsedCents, monthlyCents) }} />
          </div>
          <em>{formatDollars(monthlyUsedCents)} used this month</em>
        </Card>
      </div>
      <table className="table">
        <thead>
          <tr>
            <th>Key</th>
            <th>Daily Used</th>
            <th>Daily Budget</th>
            <th>Monthly Used</th>
            <th>Monthly Budget</th>
            <th>Cap</th>
          </tr>
        </thead>
        <tbody>
          {usageRows.map((row) => (
            <tr key={String(row.key_id)}>
              <td className="mono-cell">{String(row.name || row.key_id || "")}</td>
              <td>{formatDollars(Number(row.daily_cents ?? 0))}</td>
              <td>{formatDollars(Number(row.daily_budget_cents ?? 0))}</td>
              <td>{formatDollars(Number(row.monthly_cents ?? 0))}</td>
              <td>{formatDollars(Number(row.monthly_budget_cents ?? 0))}</td>
              <td>{row.hard_cap ? "Hard" : "Soft"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function formatDollars(cents: number) {
  return `$${(cents / 100).toFixed(2)}`;
}

function percent(value: number, max: number) {
  if (max <= 0) {
    return "0%";
  }
  return `${Math.min(100, Math.round((value / max) * 100))}%`;
}
