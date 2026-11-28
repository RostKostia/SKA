"use client";

import { Menu } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Dialog, DialogContent, DialogTrigger } from "@/components/ui/Dialog";
import { NavLinks } from "./NavLinks";

// The sidebar's replacement below 768px: a menu button that opens the same links in a drawer.
export function MobileNav() {
  const [open, setOpen] = useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm" className="md:hidden" aria-label="Open menu">
          <Menu className="size-4" aria-hidden="true" />
        </Button>
      </DialogTrigger>
      <DialogContent title="Kerdos" description="Main navigation" side="left">
        <nav aria-label="Main">
          <NavLinks onNavigate={() => setOpen(false)} />
        </nav>
      </DialogContent>
    </Dialog>
  );
}
