import { useState } from "react";

import { Disclosure } from "../molecules/Disclosure";

/** Kinds mirror internal/app/doctor.go; keep labels in sync with DoctorIssues.kindLabel. */
const issueKinds: { label: string; meaning: string }[] = [
  {
    label: "Invalid frontmatter",
    meaning:
      "SKILL.md does not meet the skill spec; the message lists each violation.",
  },
  {
    label: "Dangling symlinks",
    meaning: "A skill symlink whose target no longer exists.",
  },
  {
    label: "Broken vault links",
    meaning:
      "A symlink that points at a vault entry which has since been removed.",
  },
  {
    label: "Drift between copies",
    meaning:
      "The same skill (same name and scope) has different content across agents.",
  },
  {
    label: "Quarantine conflicts",
    meaning:
      "A disabled skill is quarantined but its original path exists again; enabling fails until that path is removed.",
  },
  {
    label: "Scan errors",
    meaning: "An agent directory or skill could not be read or inspected.",
  },
  {
    label: "Missing vault entries",
    meaning:
      "A vault entry whose directory is gone; remove it or add it again.",
  },
  {
    label: "Rejected conversions",
    meaning:
      "A source that could not be converted into a skill, with the shape and reason.",
  },
];

/** Sticky explainer shown beside the doctor report. */
export function DoctorHelp() {
  const [openSection, setOpenSection] = useState<"types" | "fix" | null>(null);
  const typesOpen = openSection === "types";
  const fixOpen = openSection === "fix";
  return (
    <aside className="doctor-help" aria-label="About doctor">
      <div className="card doctor-help-card">
        <div className="card-head">
          <h2>What is this?</h2>
        </div>
        <p className="card-hint">
          Doctor re-reads every enabled agent and registered project, compares
          every copy of each skill, and flags anything that does not parse, does
          not resolve, or does not match.
        </p>

        <Disclosure
          className="doctor-help-section"
          summary={<h3 className="doctor-help-title">Issue types</h3>}
          open={typesOpen}
          onToggle={() => setOpenSection((s) => (s === "types" ? null : "types"))}
        >
          <dl className="doctor-help-list">
            {issueKinds.map((k) => (
              <div key={k.label} className="doctor-help-item">
                <dt>{k.label}</dt>
                <dd>{k.meaning}</dd>
              </div>
            ))}
          </dl>
        </Disclosure>

        <Disclosure
          className="doctor-help-section"
          summary={<h3 className="doctor-help-title">How to fix</h3>}
          open={fixOpen}
          onToggle={() => setOpenSection((s) => (s === "fix" ? null : "fix"))}
        >
          <dl className="doctor-help-list">
            <div className="doctor-help-item">
              <dt>Repair from &lt;agent&gt;</dt>
              <dd>
                Overwrites the other copies with that agent&apos;s copy (or the
                vault entry). Copies that are already identical, symlinks into
                the vault, or read-only are skipped.
              </dd>
            </div>
            <div className="doctor-help-item">
              <dt>Adopt</dt>
              <dd>
                Stores that agent&apos;s copy in the vault as the canonical
                version, so future installs and syncs use it. Only offered when
                the vault has no entry with that name.
              </dd>
            </div>
            <div className="doctor-help-item">
              <dt>Read-only copies</dt>
              <dd>
                Skills inside the Claude plugin cache cannot be changed, so
                drift between plugin versions is informational only.
              </dd>
            </div>
            <div className="doctor-help-item">
              <dt>Invalid frontmatter</dt>
              <dd>
                Fix it by editing the skill&apos;s SKILL.md, then run doctor
                again.
              </dd>
            </div>
          </dl>
        </Disclosure>
      </div>
    </aside>
  );
}
