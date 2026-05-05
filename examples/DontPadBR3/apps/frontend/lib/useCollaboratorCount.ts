"use client";

import { useEffect, useState } from "react";
import { usePresence } from "@/lib/collab/react";

export function useCollaboratorCount() {
  const presence = usePresence();
  const [count, setCount] = useState<number>(() => {
    try {
      const others = Array.from(presence?.entries?.() ?? []);
      return Math.max(1, others.length + 1);
    } catch (e) {
      return 1;
    }
  });

  useEffect(() => {
    try {
      const others = Array.from(presence.entries());
      setCount(Math.max(1, others.length + 1));
    } catch (e) {
      setCount(1);
    }
  }, [presence]);

  return count;
}
