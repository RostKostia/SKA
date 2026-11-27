// A same-site path from a form field, or "/" for anything that could send the user elsewhere.
export function safePath(value: unknown): string {
  if (typeof value !== "string" || !value.startsWith("/")) return "/";
  if (value.startsWith("//") || value.startsWith("/\\") || /[\u0000-\u001f]/.test(value)) return "/";
  return value;
}

// Actions report back through the URL: ?notice=... or ?error=..., replacing any earlier message.
export function flash(path: string, message: { notice?: string; error?: string }): string {
  const url = new URL(path, "http://local");
  url.searchParams.delete("notice");
  url.searchParams.delete("error");
  if (message.notice) url.searchParams.set("notice", message.notice);
  if (message.error) url.searchParams.set("error", message.error);
  return url.pathname + url.search;
}
