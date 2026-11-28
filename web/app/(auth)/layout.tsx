import type { ReactNode } from "react";

// Sign-in and sign-up sit outside the app shell: no navigation until you're signed in.
export default function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="auth">
      <div className="shell__brand auth__brand">
        <span aria-hidden="true">κ</span>
        <span>Kerdos</span>
      </div>
      {children}
    </div>
  );
}
