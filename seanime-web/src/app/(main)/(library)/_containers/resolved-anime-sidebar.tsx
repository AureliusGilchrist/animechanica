"use client"
import React from "react"
import { useListLinkedAnimeFiles, useMoveRenameAnimeSeries } from "@/api/hooks/anime_linked.hooks"
import { Anime_LibraryCollectionList } from "@/api/generated/types"
import { Badge } from "@/components/ui/badge"
import { TextInput } from "@/components/ui/text-input"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { cn } from "@/components/ui/core/styling"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"

export type ResolvedAnimeSidebarProps = {
  collectionList: Anime_LibraryCollectionList[]
}

// Small helper to map mediaId -> title from library collection
function useMediaTitleMap(collectionList: Anime_LibraryCollectionList[]) {
  return React.useMemo(() => {
    const map = new Map<number, string>()
    for (const col of collectionList ?? []) {
      // collection may contain multiple entries; ensure mapping for each mediaId
      for (const entry of col.entries ?? []) {
        if (entry?.media?.id) {
          const title = entry.media.title?.romaji || entry.media.title?.english || String(entry.media.id)
          map.set(entry.media.id, title)
        }
      }
    }
    return map
  }, [collectionList])
}

export function ResolvedAnimeSidebar(props: ResolvedAnimeSidebarProps) {
  const { collectionList } = props
  const titleMap = useMediaTitleMap(collectionList)

  // Fetch a large page of linked files; backend supports pagination, but for sidebar summary pulling many is acceptable
  const { data, isLoading, refetch } = useListLinkedAnimeFiles({ page: 1, pageSize: 5000 })

  const { mutate: moveRename, isPending } = useMoveRenameAnimeSeries()

  const [query, setQuery] = React.useState("")
  const [expanded, setExpanded] = React.useState<Record<number, boolean>>({})

  const grouped = React.useMemo(() => {
    const m = new Map<number, { mediaId: number; files: { path: string }[] }>()
    for (const it of data?.items ?? []) {
      const g = m.get(it.mediaId) || { mediaId: it.mediaId, files: [] as { path: string }[] }
      g.files.push({ path: it.path })
      m.set(it.mediaId, g)
    }
    let arr = Array.from(m.values())
    if (query.trim()) {
      const q = query.toLowerCase()
      arr = arr.filter(g => (titleMap.get(g.mediaId)?.toLowerCase().includes(q)) || g.mediaId.toString().includes(q))
    }
    // sort by title asc then by mediaId
    arr.sort((a, b) => {
      const ta = titleMap.get(a.mediaId) || String(a.mediaId)
      const tb = titleMap.get(b.mediaId) || String(b.mediaId)
      return ta.localeCompare(tb)
    })
    return arr
  }, [data?.items, titleMap, query])

  return (
    <aside className={cn(
      "w-80 shrink-0 border-r border-[--border] bg-[--background]",
      "max-h-[calc(100vh-6rem)] overflow-y-auto sticky top-[4.5rem] p-3",
    )} data-resolved-anime-sidebar>
      <div className="flex items-center justify-between mb-2">
        <div className="font-semibold">Resolved Anime</div>
        <Badge intent="gray">{(data?.total ?? 0).toLocaleString()}</Badge>
      </div>
      <TextInput
        placeholder="Filter by title or ID…"
        value={query}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setQuery(e.target.value)}
        className="mb-3"
      />
      {isLoading && (
        <div className="py-8 flex items-center gap-2"><LoadingSpinner /> Loading…</div>
      )}
      {!isLoading && grouped.length === 0 && (
        <div className="py-8 text-sm opacity-70">No resolved anime found.</div>
      )}
      <div className="flex flex-col gap-2">
        {grouped.map(g => {
          const isOpen = expanded[g.mediaId]
          const title = titleMap.get(g.mediaId) || `Media ${g.mediaId}`
          return (
            <div key={g.mediaId} className="rounded-[--radius-md] border border-[--border]">
              <div className="w-full px-3 py-2 flex items-center gap-2">
                <button
                  className="flex-1 text-left hover:underline"
                  onClick={() => setExpanded(s => ({ ...s, [g.mediaId]: !s[g.mediaId] }))}
                  title={title}
                >
                  <span className="truncate" title={title}>{title}</span>
                </button>
                <Badge intent="gray">{g.files.length} ep</Badge>
                <Button
                  size="xs"
                  intent="gray-subtle"
                  disabled={isPending}
                  onClick={() => {
                    const proceed = window.confirm(
                      `Move & rename all main episodes for "${title}" into the series folder with standard naming?`
                    )
                    if (!proceed) return
                    const doDelete = window.confirm(
                      "Also delete the original release folder(s) after successful moves?"
                    )
                    moveRename(
                      { mediaId: g.mediaId, confirmDelete: doDelete },
                      {
                        onSuccess: (res) => {
                          const moved = res?.moved ?? 0
                          const skipped = res?.skipped ?? 0
                          const delCount = res?.deletedFolders?.length ?? 0
                          const errCount = res?.errors?.length ?? 0
                          toast.success(
                            `Moved: ${moved}, skipped: ${skipped}${delCount ? ", deleted folders: " + delCount : ""}`
                          )
                          if (errCount) {
                            toast.warning(`Some errors occurred: ${errCount}`)
                          }
                          refetch()
                        },
                        onError: (err: any) => {
                          toast.error(err?.message || "Move & rename failed")
                        },
                      }
                    )
                  }}
                >
                  Move & Rename
                </Button>
              </div>
              {isOpen && (
                <div className="px-3 pb-2">
                  <ul className="space-y-1 text-xs">
                    {g.files.slice().sort((a,b)=>a.path.localeCompare(b.path)).map((f, idx) => (
                      <li key={idx} className="break-all opacity-80">{f.path.split(/[\\\/]/).pop()}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </aside>
  )
}
