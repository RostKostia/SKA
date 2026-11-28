import type { Metadata } from "next";
import { cookies } from "next/headers";
import { Suspense, type CSSProperties } from "react";
import { inviteMember, removeMember, revokeInvite } from "@/app/(app)/actions";
import { Field } from "@/components/AuthCard";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { TBody, THead, Table, Td, Th, Tr } from "@/components/ui/Table";
import { getApi } from "@/lib/api";
import type { Invite } from "@/lib/api/types";
import { roleLabel } from "@/lib/account";
import { formatDate } from "@/lib/format";
import type { Tone } from "@/lib/status";
import { INVITE_LINK_COOKIE } from "@/lib/session";

export const metadata: Metadata = { title: "Team · Kerdos" };

const step = (i: number) => ({ "--i": i }) as CSSProperties;
const day = (iso: string | null) => (iso ? formatDate(iso.slice(0, 10)) : "Never");

function emailState(invite: Invite): { label: string; tone: Tone } {
  switch (invite.email_status) {
    case "sent":
      return { label: "Email sent", tone: "success" };
    case "failed":
      return { label: `Email failed: ${invite.email_error}`, tone: "danger" };
    case "cancelled":
      return { label: "Email not sent", tone: "neutral" };
    case "queued":
      return { label: "Email on its way", tone: "info" };
    default:
      return { label: "No email", tone: "neutral" };
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
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader
        title="Team"
        description="Everyone here works on the same business. Invite people by email: they create their account from the link in the invitation."
      />

      <Card className="reveal max-w-3xl" style={step(1)} aria-labelledby="invite-title">
        <CardHeader>
          <CardTitle id="invite-title">Invite someone</CardTitle>
        </CardHeader>
        <CardContent>
          <form action={inviteMember} className="grid items-end gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
            <input type="hidden" name="returnTo" value="/team" />
            <Field label="Email">
              <Input name="email" type="email" required />
            </Field>
            <Field label="Name (optional)">
              <Input name="name" maxLength={100} />
            </Field>
            <Button type="submit" variant="primary">Send invitation</Button>
          </form>
          {fresh ? (
            <div className="grid gap-1.5 rounded-md border border-border bg-background p-3">
              <Field label={`Invitation link for ${fresh.email}`} hint={`Works once, for 7 days. Anyone with it can join as ${fresh.email}, so only send it to them.`}>
                <Input readOnly value={fresh.link} className="font-mono text-xs" />
              </Field>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <section className="reveal grid gap-3" style={step(2)} aria-labelledby="members-title">
        <h2 id="members-title" className="text-base font-semibold tracking-tight">Members</h2>
        <Table caption="Members of the business">
          <THead>
            <Tr>
              <Th>Name</Th>
              <Th>Email</Th>
              <Th>Role</Th>
              <Th>Signs in with</Th>
              <Th>Last sign-in</Th>
              <Th className="text-right"><span className="sr-only">Actions</span></Th>
            </Tr>
          </THead>
          <TBody>
            {team.members.map((m) => (
              <Tr key={m.id}>
                <Td className="font-medium">
                  {m.name}
                  {m.you ? <span className="text-muted"> (you)</span> : null}
                </Td>
                <Td className="whitespace-nowrap">{m.email}</Td>
                <Td>
                  <Badge tone={m.role === "admin" ? "info" : "neutral"}>{roleLabel(m.role)}</Badge>
                </Td>
                <Td>{[m.has_password && "Password", m.google_linked && "Google"].filter(Boolean).join(", ")}</Td>
                <Td className="whitespace-nowrap">{day(m.last_login_at)}</Td>
                <Td className="text-right">
                  {m.you ? null : (
                    <form action={removeMember}>
                      <input type="hidden" name="returnTo" value="/team" />
                      <input type="hidden" name="id" value={m.id} />
                      <input type="hidden" name="name" value={m.name} />
                      <Button type="submit" size="sm" variant="danger" aria-label={`Remove ${m.name}`}>Remove</Button>
                    </form>
                  )}
                </Td>
              </Tr>
            ))}
          </TBody>
        </Table>
      </section>

      <section className="reveal grid gap-3" style={step(3)} aria-labelledby="invites-title">
        <h2 id="invites-title" className="text-base font-semibold tracking-tight">Open invitations</h2>
        {team.invites.length === 0 ? (
          <p className="text-sm text-muted">None.</p>
        ) : (
          <Table caption="Open invitations">
            <THead>
              <Tr>
                <Th>Invited</Th>
                <Th>Invited by</Th>
                <Th>Expires</Th>
                <Th>Invitation email</Th>
                <Th className="text-right"><span className="sr-only">Actions</span></Th>
              </Tr>
            </THead>
            <TBody>
              {team.invites.map((inv) => {
                const email = emailState(inv);
                return (
                  <Tr key={inv.id}>
                    <Td className="font-medium">
                      {inv.name ? <>{inv.name} <span className="text-muted">· {inv.email}</span></> : inv.email}
                    </Td>
                    <Td>{inv.invited_by ?? ""}</Td>
                    <Td className="whitespace-nowrap">{inv.expired ? <Badge tone="warning">Expired</Badge> : day(inv.expires_at)}</Td>
                    <Td>
                      <Badge tone={email.tone} className="max-w-64 truncate">{email.label}</Badge>
                    </Td>
                    <Td>
                      <div className="flex justify-end gap-2">
                        <form action={inviteMember}>
                          <input type="hidden" name="returnTo" value="/team" />
                          <input type="hidden" name="email" value={inv.email} />
                          <input type="hidden" name="name" value={inv.name ?? ""} />
                          <Button type="submit" size="sm" aria-label={`Invite ${inv.email} again`}>Invite again</Button>
                        </form>
                        <form action={revokeInvite}>
                          <input type="hidden" name="returnTo" value="/team" />
                          <input type="hidden" name="id" value={inv.id} />
                          <input type="hidden" name="email" value={inv.email} />
                          <Button type="submit" size="sm" aria-label={`Cancel the invitation for ${inv.email}`}>Cancel</Button>
                        </form>
                      </div>
                    </Td>
                  </Tr>
                );
              })}
            </TBody>
          </Table>
        )}
      </section>
    </div>
  );
}

export default function TeamPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <TeamPageContent />
    </Suspense>
  );
}
