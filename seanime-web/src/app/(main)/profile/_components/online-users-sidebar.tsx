"use client"

import React from "react"
import { useNakamaStatus } from "@/app/(main)/_features/nakama/nakama-manager"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/components/ui/core/styling"
import { Avatar } from "@/components/ui/avatar"

/**
 * OnlineUsersSidebar
 * Displays connected Nakama peers (online users) on the right side of the profile page.
 * Falls back gracefully when Nakama is not connected.
 */
export function OnlineUsersSidebar({ className }: { className?: string }) {
  const nakama = useNakamaStatus()

  const isActive = !!nakama && (nakama.isHost || nakama.isConnectedToHost)
  const peers: string[] = (nakama?.connectedPeers ?? []) as string[]

  // Attempt to enrich with usernames when available via hostConnectionStatus/current session info in other parts of app.
  // For now, peers are strings (peer IDs or usernames) from Nakama_NakamaStatus.connectedPeers

  return (
    <aside
      className={cn(
        "rounded-2xl border bg-card/60 backdrop-blur p-4 sm:p-5 sticky top-4",
        // allow inner list to scroll while header stays visible within sticky card height
        "max-h-[calc(100vh-2rem)]",
        className,
      )}
      aria-label="Online users sidebar"
    >
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm sm:text-base font-semibold">
          Online Users
          <span className="ml-2 text-xs font-medium text-muted-foreground align-middle">{peers.length}</span>
        </h3>
        <Badge intent={isActive ? "success" : "gray"} className="px-2 py-0.5">
          {isActive ? "Connected" : "Offline"}
        </Badge>
      </div>

      {!isActive && (
        <p className="text-xs text-muted-foreground" aria-live="polite">
          Connect to Nakama to see online users.
        </p>
      )}

      <div className="mt-2 space-y-2 overflow-auto pr-1" role="list">
        {peers.length === 0 && (
          <p className="text-xs text-muted-foreground text-center py-4 border rounded-lg bg-background/40">
            No users online
          </p>
        )}
        {peers.map((p, idx) => (
          <OnlineUserRow key={idx} display={p} />
        ))}
      </div>
    </aside>
  )
}

function OnlineUserRow({ display }: { display: string }) {
  // Derive simple initials for avatar fallback
  const initials = React.useMemo(() => {
    const parts = (display || "?").trim().split(/\s+/)
    const s = parts.slice(0, 2).map(w => w.charAt(0).toUpperCase()).join("")
    return s || "?"
  }, [display])

  return (
    <div
      className="flex items-center gap-3 p-2 border rounded-lg bg-background/40 hover:bg-background/60 transition-colors focus-within:ring-1 focus-within:ring-[--border]"
      role="listitem"
    >
      <Avatar className="h-8 w-8" fallback={initials} aria-label={display} />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium truncate">{display}</p>
        <p className="text-[10px] text-muted-foreground flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-full bg-emerald-500 dark:bg-emerald-400" aria-hidden />
          Online
        </p>
      </div>
    </div>
  )
}
