// Up to two upper-case initials for an avatar; "?" when the name is empty.
export function initials(name: string): string {
  const letters = name
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word[0].toUpperCase());
  return letters.length ? letters.join("") : "?";
}

// The backend's roles in the words the UI uses: the owner is "admin", everyone invited is "user".
export function roleLabel(role: string): string {
  return role === "admin" ? "Owner" : "Member";
}
