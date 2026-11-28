import type { ReactNode } from "react";

// Sign-in and sign-up sit outside the app shell: no navigation until you're signed in.
export default function AuthLayout({ children }: { children: ReactNode }) {
  return <main className="grid min-h-screen place-items-center px-4 py-10">{children}</main>;
}
