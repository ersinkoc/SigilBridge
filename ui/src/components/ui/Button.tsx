import type { ButtonHTMLAttributes, ReactNode } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  icon?: ReactNode;
  variant?: "primary" | "secondary" | "ghost";
};

export function Button({ icon, children, variant = "secondary", className = "", ...props }: Props) {
  return (
    <button className={`btn btn-${variant} ${className}`} {...props}>
      {icon}
      {children ? <span>{children}</span> : null}
    </button>
  );
}
