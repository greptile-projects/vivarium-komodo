import { ActivityItem, Avatar, Badge, Button, EmptyState, RepositoryCard } from "@/components/ui";
import { ArrowRight, Branch, GitPullRequest, Plus, Sparkles } from "@/components/icons";

const repositories = [
  { name: "wayfinder", description: "Tools for making collaborative work legible.", language: "TypeScript", color: "#2dd4bf", updated: "12 min ago" },
  { name: "field-notes", description: "A shared record of decisions, experiments, and outcomes.", language: "Go", color: "#38bdf8", updated: "Yesterday" },
];

export default function Home() {
  return (
    <div className="page-grid">
      <section className="min-w-0" aria-labelledby="welcome-heading">
        <div className="eyebrow"><Sparkles size={14} /> Your workspace</div>
        <div className="page-heading">
          <div>
            <h1 id="welcome-heading">Good morning, Rowan.</h1>
            <p>Pick up where your collaborators left off.</p>
          </div>
          <Button><Plus size={16} /> New repository</Button>
        </div>

        <div className="section-heading">
          <div><h2>Repositories</h2><p>Your most recently active projects</p></div>
          <a className="text-link" href="#repositories">View all <ArrowRight size={15} /></a>
        </div>
        <div className="repo-grid" id="repositories">
          {repositories.map((repository) => <RepositoryCard key={repository.name} {...repository} />)}
        </div>

        <div className="section-heading activity-heading">
          <div><h2>Recent activity</h2><p>Work moving across your projects</p></div>
        </div>
        <div className="panel activity-list">
          <ActivityItem icon={<GitPullRequest size={16} />} actor="Mina Okafor" action="opened a pull request" subject="Clarify repository access states" repository="wayfinder" time="18 minutes ago" accent="purple" />
          <ActivityItem icon={<Branch size={16} />} actor="Alex Chen" action="pushed 3 commits to" subject="feat/review-summary" repository="field-notes" time="2 hours ago" accent="blue" />
          <ActivityItem avatar={<Avatar initials="RO" size="sm" />} actor="You" action="commented on" subject="Design the contributor handoff" repository="wayfinder" time="Yesterday" />
        </div>
      </section>

      <aside className="sidebar" aria-label="Workspace summary">
        <div className="panel attention-panel">
          <div className="panel-title"><span>Needs attention</span><Badge tone="accent">2</Badge></div>
          <a className="attention-item" href="#activity">
            <span className="attention-icon purple"><GitPullRequest size={16} /></span>
            <span><strong>Review requested</strong><small>wayfinder · #24</small></span>
            <span className="attention-time">18m</span>
          </a>
          <a className="attention-item" href="#activity">
            <span className="attention-icon green">✓</span>
            <span><strong>Ready to merge</strong><small>field-notes · #17</small></span>
            <span className="attention-time">3h</span>
          </a>
        </div>

        <div className="panel quick-panel">
          <div className="panel-title"><span>Quick start</span></div>
          <EmptyState icon={<Branch size={20} />} title="Start something together" description="Create a repository and invite collaborators when you’re ready." action={<Button variant="secondary" size="sm">Create repository</Button>} />
        </div>
      </aside>
    </div>
  );
}
