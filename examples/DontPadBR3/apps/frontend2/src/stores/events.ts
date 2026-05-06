import { create } from "zustand";

type FrontendEvent =
  | { type: "route:navigated"; payload: { path: string }; at: number }
  | { type: "panel:opened"; payload: { panel: string }; at: number }
  | { type: "sync:status"; payload: { status: string }; at: number };

interface FrontendEventState {
  events: FrontendEvent[];
  emit: <T extends FrontendEvent["type"]>(
    type: T,
    payload: Extract<FrontendEvent, { type: T }>["payload"],
  ) => void;
  clear: () => void;
}

export const useFrontendEvents = create<FrontendEventState>()((set) => ({
  events: [],
  emit: (type, payload) =>
    set((state) => ({
      events: [
        ...state.events.slice(-49),
        { type, payload, at: Date.now() } as FrontendEvent,
      ],
    })),
  clear: () => set({ events: [] }),
}));
