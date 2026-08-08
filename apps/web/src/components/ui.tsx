import type { ButtonHTMLAttributes, ReactNode } from "react";
import { Book, Branch } from "./icons";

export function Button({ variant = "primary", size = "md", className = "", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "danger"; size?: "sm" | "md" }) {
  return <button className={`button ${variant} ${size} ${className}`} {...props} />;
}
export function Avatar({ initials, size = "md" }: { initials: string; size?: "sm" | "md" }) { return <span className={`avatar ${size}`} aria-hidden="true">{initials}</span>; }
export function Badge({ children, tone = "neutral" }: { children: ReactNode; tone?: "neutral" | "accent" }) { return <span className={`badge ${tone}`}>{children}</span>; }
export function RepositoryCard({ name, description, language, color, updated }: { name: string; description: string; language: string; color: string; updated: string }) {
  return <article className="repo-card panel"><div className="repo-card-top"><span className="repo-icon"><Book size={18}/></span><Badge>Private</Badge></div><a href="#repository"><h3>{name}</h3></a><p>{description}</p><div className="repo-meta"><span><i style={{ background: color }} />{language}</span><span><Branch size={13}/> main</span><span className="updated">Updated {updated}</span></div></article>;
}
export function ActivityItem({ icon, avatar, actor, action, subject, repository, time, accent = "neutral" }: { icon?: ReactNode; avatar?: ReactNode; actor: string; action: string; subject: string; repository: string; time: string; accent?: string }) {
  return <article className="activity-item" id="activity"><div className={`activity-symbol ${accent}`}>{avatar ?? icon}</div><div className="activity-copy"><p><strong>{actor}</strong> {action} <a href="#work">{subject}</a></p><small>{repository} <span>·</span> {time}</small></div></article>;
}
export function EmptyState({ icon, title, description, action }: { icon: ReactNode; title: string; description: string; action?: ReactNode }) { return <div className="empty-state"><span className="empty-icon">{icon}</span><strong>{title}</strong><p>{description}</p>{action}</div>; }
