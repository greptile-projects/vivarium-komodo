import Link from "next/link";
import { Bell, Book, GitPullRequest, Home, Menu, Plus, Search, Users } from "./icons";
import { Avatar } from "./ui";

const navigation = [
  { label: "Home", href: "/", icon: Home, active: true },
  { label: "Repositories", href: "#repositories", icon: Book },
  { label: "Pull requests", href: "#activity", icon: GitPullRequest, count: 3 },
  { label: "People", href: "#people", icon: Users },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to main content</a>
    <header className="topbar">
      <div className="topbar-inner">
        <details className="mobile-menu"><summary aria-label="Open navigation"><Menu /></summary><nav>{navigation.map(({ label, href }) => <Link href={href} key={label}>{label}</Link>)}</nav></details>
        <Link className="brand" href="/" aria-label="Kanso home"><span className="brand-mark">K</span><span>Kanso</span></Link>
        <label className="search"><Search size={16}/><span className="sr-only">Search Kanso</span><input type="search" placeholder="Search projects, people, or work…"/><kbd>/</kbd></label>
        <div className="top-actions">
          <button className="icon-button create-button" aria-label="Create new"><Plus /></button>
          <button className="icon-button notification-button" aria-label="Notifications"><Bell /><span className="notification-dot" /></button>
          <button className="profile-button" aria-label="Open account menu"><Avatar initials="RO" size="sm"/><span className="profile-copy"><strong>Rowan</strong><small>@rowan</small></span></button>
        </div>
      </div>
    </header>
    <div className="shell-body">
      <nav className="side-nav" aria-label="Primary navigation">
        <p className="nav-label">Workspace</p>
        {navigation.map(({ label, href, icon: NavIcon, active, count }) => <Link key={label} href={href} className={active ? "nav-item active" : "nav-item"} aria-current={active ? "page" : undefined}><NavIcon size={17}/><span>{label}</span>{count && <span className="nav-count">{count}</span>}</Link>)}
        <div className="nav-divider" />
        <p className="nav-label">Pinned</p>
        <a className="nav-item pinned" href="#repositories"><span className="repo-dot teal" />wayfinder</a>
        <a className="nav-item pinned" href="#repositories"><span className="repo-dot blue" />field-notes</a>
      </nav>
      <main id="main-content" className="main-content">{children}</main>
    </div>
  </div>;
}
