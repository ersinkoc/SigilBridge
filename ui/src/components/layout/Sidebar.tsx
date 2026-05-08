import {
  Activity,
  BadgeDollarSign,
  ClipboardList,
  DatabaseZap,
  Gauge,
  HeartPulse,
  KeyRound,
  Layers3,
  PlugZap,
  Route,
  Settings,
  WandSparkles
} from "lucide-react";
import { NavLink } from "react-router-dom";

const navGroups = [
  {
    label: "Operate",
    items: [
      { to: "/", label: "Dashboard", icon: Gauge },
      { to: "/setup", label: "Setup", icon: WandSparkles },
      { to: "/keys", label: "Keys", icon: KeyRound },
      { to: "/models", label: "Models", icon: Layers3 },
      { to: "/pools", label: "Pools", icon: Route },
      { to: "/credentials", label: "Credentials", icon: PlugZap }
    ]
  },
  {
    label: "Observe",
    items: [
      { to: "/audit", label: "Audit", icon: ClipboardList },
      { to: "/budgets", label: "Budgets", icon: BadgeDollarSign },
      { to: "/health", label: "Health", icon: HeartPulse },
      { to: "/events", label: "Events", icon: Activity }
    ]
  },
  {
    label: "System",
    items: [{ to: "/settings", label: "Settings", icon: Settings }]
  }
];

export function Sidebar() {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">
          <DatabaseZap size={18} />
        </div>
        <div className="brand-copy">
          <strong>SigilBridge</strong>
          <span>Control plane</span>
        </div>
      </div>
      <nav>
        {navGroups.map((group) => (
          <div key={group.label} className="nav-group">
            <span className="nav-group-label">{group.label}</span>
            {group.items.map((item) => {
              const Icon = item.icon;
              return (
                <NavLink key={item.to} to={item.to} end={item.to === "/"} className={({ isActive }) => `nav-link ${isActive ? "active" : ""}`}>
                  <Icon size={17} />
                  <span>{item.label}</span>
                </NavLink>
              );
            })}
          </div>
        ))}
      </nav>
      <div className="sidebar-footer">
        <span className="status-dot" />
        <div>
          <strong>Local admin</strong>
          <span>Private control plane</span>
        </div>
      </div>
    </aside>
  );
}
