// Case-, accent- and final-sigma-insensitive form for search, so "μαρια" finds "Μαρία Σάββα".
export function fold(text: string): string {
  return text.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase().replace(/ς/g, "σ");
}
