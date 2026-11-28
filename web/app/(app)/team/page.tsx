import { Suspense } from "react";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import { inviteMember, removeMember, revokeInvite } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { Button } from "@/components/ui/Button";
import { getApi } from "@/lib/api";
import type { Invite } from "@/lib/api/types";
import { formatDate } from "@/lib/format";
import { INVITE_LINK_COOKIE } from "@/lib/session";

export const metadata: Metadata = { title: "Team · Kerdos" };

const day = (iso: string | null) => (iso ? formatDate(iso.slice(0, 10)) : "Never");

function emailState(invite: Invite) {
  switch (invite.email_status) {
    case "sent":
      return "Email sent";
    case "failed":
      return `Email failed: ${invite.email_error}`;
    case "cancelled":
      return "Email not sent";
    case "queued":
      return "Email on its way";
    default:
      return "No email";
  }
}

async function readNewLink(): Promise<{ email: string; link: string } | null> {
  const raw = (await cookies()).get(INVITE_LINK_COOKIE)?.value;
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw);
    return typeof parsed?.link === "string" && typeof parsed?.email === "string" ? parsed : null;
  } catch {
    return null;
  }
}

async function TeamPageContent() {
  let team;
  try {
    team = await getApi().getTeam();
  } catch (error) {
    return apiErrorView(error);
  }
  const fresh = await readNewLink();

  return (
    <main className="shell__main">
      <h1>Team</h1>
      <p className="muted">Everyone here works on the same business. Invite people by email: they create their account from the link in the invitation.</p>

      <section className="slab stack account__section" aria-labelledby="invite-title">
        <h2 id="invite-title">Invite someone</h2>
        <form action={inviteMember} className="toolbar">
          <input type="hidden" name="returnTo" value="/team" />
          <label className="stack auth__label">
            Email
            <input className="field" name="email" type="email" required />
          </label>
          <label className="stack auth__label">
            Name (optional)
            <input className="field" name="name" maxLength={100} />
          </label>
          <Button type="submit" variant="primary">Send invitation</Button>
        </form>
        {fresh ? (
          <div className="stack">
            <label className="stack auth__label">
              Invitation link for {fresh.email} (works for 7 days, once)
              <input className="field" readOnly value={fresh.link} />
            </label>
            <p className="muted auth__hint">Anyone with this link can join as {fresh.email}, so only send it to them.</p>
          </div>
        ) : null}
      </section>

      <section className="stack" aria-labelledby="members-title">
        <h2 id="members-title">Members</h2>
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th scope="col">Name</th>
                <th scope="col">Email</th>
                <th scope="col">Role</th>
                <th scope="col">Signs in with</th>
                <th scope="col">Last sign-in</th>
                <th scope="col" className="actions"><span className="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {team.members.map((m) => (
                <tr key={m.id}>
                  <th scope="row">{m.name}{m.you ? " (you)" : ""}</th>
                  <td>{m.email}</td>
                  <td>{m.role === "admin" ? "Owner" : "Member"}</td>
                  <td>{[m.has_password && "Password", m.google_linked && "Google"].filter(Boolean).join(", ")}</td>
                  <td>{day(m.last_login_at)}</td>
                  <td className="actions">
                    {m.you ? null : (
                      <form action={removeMember}>
                        <input type="hidden" name="returnTo" value="/team" />
                        <input type="hidden" name="id" value={m.id} />
                        <input type="hidden" name="name" value={m.name} />
                        <Button type="submit" className="key--small" aria-label={`Remove ${m.name}`}>Remove</Button>
                      </form>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="stack" aria-labelledby="invites-title">
        <h2 id="invites-title">Open invitations</h2>
        {team.invites.length === 0 ? (
          <p className="muted">None.</p>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th scope="col">Invited</th>
                  <th scope="col">Invited by</th>
                  <th scope="col">Expires</th>
                  <th scope="col">Invitation email</th>
                  <th scope="col" className="actions"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody>
                {team.invites.map((inv) => (
                  <tr key={inv.id}>
                    <th scope="row">{inv.name ? `${inv.name} · ${inv.email}` : inv.email}</th>
                    <td>{inv.invited_by ?? ""}</td>
                    <td>{inv.expired ? "Expired" : day(inv.expires_at)}</td>
                    <td>{emailState(inv)}</td>
                    <td className="actions">
                      <form action={inviteMember}>
                        <input type="hidden" name="returnTo" value="/team" />
                        <input type="hidden" name="email" value={inv.email} />
                        <input type="hidden" name="name" value={inv.name ?? ""} />
                        <Button type="submit" className="key--small" aria-label={`Invite ${inv.email} again`}>Invite again</Button>
                      </form>
                      <form action={revokeInvite}>
                        <input type="hidden" name="returnTo" value="/team" />
                        <input type="hidden" name="id" value={inv.id} />
                        <input type="hidden" name="email" value={inv.email} />
                        <Button type="submit" className="key--small" aria-label={`Cancel the invitation for ${inv.email}`}>Cancel</Button>
                      </form>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </main>
  );
}

export default function TeamPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <TeamPageContent />
    </Suspense>
  );
}
