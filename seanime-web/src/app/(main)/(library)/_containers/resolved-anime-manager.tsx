"use client"
import { useListLinkedAnimeFiles, useUnlinkLocalFiles, useToggleAutoMatchBlocked, useMoveRenameAnimeSeries, useHideAnimeSeries, useUnhideAnimeSeries } from "@/api/hooks/anime_linked.hooks"
import { useDeleteLocalFiles } from "@/api/hooks/localfiles.hooks"
import { upath } from "@/lib/helpers/upath"
import { atom } from "jotai"
import { useAtom } from "jotai/react"
import React from "react"
import { Drawer } from "@/components/ui/drawer"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { Button, IconButton } from "@/components/ui/button"
import { Select } from "@/components/ui/select"
import { Badge } from "@/components/ui/badge"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { TbRefresh, TbUnlink } from "react-icons/tb"
import { Switch } from "@/components/ui/switch"
import { NumberInput } from "@/components/ui/number-input"
import { toast } from "sonner"
import { useHandleLibraryCollection } from "@/app/(main)/(library)/_lib/handle-library-collection"

export const __resolvedAnimeManagerIsOpen = atom(false)

export function ResolvedAnimeManager() {
  const [isOpen, setIsOpen] = useAtom(__resolvedAnimeManagerIsOpen)
  const [source, setSource] = React.useState<"any" | "auto" | "manual">("any")
  const [blocked, setBlocked] = React.useState<"any" | "true" | "false">("any")
  const [mediaId, setMediaId] = React.useState<number | undefined>(undefined)
  const [page, setPage] = React.useState(1)
  const [hiddenMode, setHiddenMode] = React.useState<"include" | "exclude" | "only">("exclude")
  const pageSize = 5000

  // Fetch a large page to ensure all episodes for a series are grouped together
  const { data, isLoading, refetch } = useListLinkedAnimeFiles({
    source,
    blocked,
    mediaId,
    page: 1,
    pageSize: 5000,
    hidden: hiddenMode,
  })

  const { mutate: unlinkFiles, isPending: isUnlinking } = useUnlinkLocalFiles()
  const { mutate: toggleBlock, isPending: isToggling } = useToggleAutoMatchBlocked()
  const { mutate: moveRename, isPending: isMoving } = useMoveRenameAnimeSeries()
  const { mutate: hideSeries, isPending: isHiding } = useHideAnimeSeries()
  const { mutate: unhideSeries, isPending: isUnhiding } = useUnhideAnimeSeries()
  const { mutate: deleteFiles, isPending: isDeleting } = useDeleteLocalFiles(undefined)

  const isBusy = isLoading || isUnlinking || isToggling || isMoving || isHiding || isUnhiding || isDeleting

  // Build a mediaId -> title map (prefer romaji, fallback english, then ID)
  const { libraryCollectionList } = useHandleLibraryCollection()
  const titleMap = React.useMemo(() => {
    const map = new Map<number, string>()
    for (const col of libraryCollectionList ?? []) {
      for (const entry of col.entries ?? []) {
        const id = entry?.media?.id
        if (!id) continue
        const title = entry?.media?.title?.romaji || entry?.media?.title?.english || String(id)
        map.set(id, title)
      }
    }
    return map
  }, [libraryCollectionList])

  React.useEffect(() => {
    if (!isOpen) {
      // Reset filters when closed
      setSource("any")
      setBlocked("any")
      setMediaId(undefined)
      setPage(1)
    }
  }, [isOpen])

  const [localItems, setLocalItems] = React.useState<any[]>([])
  React.useEffect(() => { setLocalItems(data?.items ?? []) }, [data?.items])
  const groups = React.useMemo(() => {
    const map: Record<number, any[]> = {}
    const items = (localItems ?? []) as any[]
    for (const it of items) {
      const key = it.mediaId ?? 0
      if (!map[key]) map[key] = [] as any
      map[key]!.push(it as any)
    }
    return map
  }, [localItems])

  return (
    <Drawer
      open={isOpen}
      onOpenChange={() => setIsOpen(false)}
      size="xl"
      title="Resolved anime"
    >
      <AppLayoutStack className="mt-4 gap-3">
        <div className="flex flex-wrap gap-2 items-end">
          <div className="flex flex-col">
            <label className="text-xs opacity-80">Source</label>
            <Select
              value={source}
              onValueChange={(v: string) => { setPage(1); setSource(v as any) }}
              options={[
                { label: "Any", value: "any" },
                { label: "Automatic", value: "auto" },
                { label: "Manual", value: "manual" },
              ]}
            />
          </div>
          <div className="flex flex-col">
            <label className="text-xs opacity-80">Blocked</label>
            <Select
              value={blocked}
              onValueChange={(v: string) => { setPage(1); setBlocked(v as any) }}
              options={[
                { label: "Any", value: "any" },
                { label: "Blocked", value: "true" },
                { label: "Not blocked", value: "false" },
              ]}
            />
          </div>
          <div className="flex flex-col">
            <label className="text-xs opacity-80">Show</label>
            <Select
              value={hiddenMode}
              onValueChange={(v: string) => { setPage(1); setHiddenMode(v as any); refetch() }}
              options={[
                { label: "Visible", value: "exclude" },
                { label: "Hidden", value: "only" },
                { label: "All", value: "include" },
              ]}
            />
          </div>
          <div className="flex flex-col">
            <label className="text-xs opacity-80">Media ID</label>
            <NumberInput value={mediaId ?? 0} onValueChange={(v) => { setPage(1); setMediaId(v > 0 ? v : undefined) }} formatOptions={{ useGrouping: false }} />
          </div>
          <div className="flex-1" />
          <IconButton intent="gray-subtle" icon={<TbRefresh />} onClick={() => refetch()} disabled={isBusy} />
        </div>

        <div className="text-sm opacity-70">
          {(data?.total ?? 0).toLocaleString()} linked files
        </div>

        <div className="bg-gray-950 border rounded-[--radius-md] overflow-y-auto max-h-[60vh]">
          {isLoading && <div className="p-6 flex items-center gap-3"><LoadingSpinner /> Loading linked files…</div>}
          {!isLoading && (data?.items?.length ?? 0) === 0 && (
            <div className="p-6">No linked files match the current filters.</div>
          )}
          {!isLoading && Object.entries(groups)
            .sort(([a], [b]) => {
              const ia = Number(a)
              const ib = Number(b)
              const ta = titleMap.get(ia) || `Media ${ia}`
              const tb = titleMap.get(ib) || `Media ${ib}`
              return ta.localeCompare(tb)
            })
            .map(([mid, files]) => {
            const mediaIdNum = Number(mid)
            const title = titleMap.get(mediaIdNum) || `Media ${mediaIdNum}`
            const anyHidden = files.some((f: any) => !!f.hidden)
            const paths = files.map(f => f.path)
            return (
              <div key={`group-${mid}`} className="border-b border-[--border]">
                {/* Group header */}
                <div className="p-3 bg-black/30 flex flex-wrap items-center gap-2">
                  <div className="font-semibold">{title}</div>
                  <Badge intent="gray">{files.length} file{files.length === 1 ? "" : "s"}</Badge>
                  {anyHidden && <Badge intent="warning">Hidden</Badge>}
                  <div className="ml-auto flex items-center gap-2 w-[680px] max-w-full justify-end">
                    <Button
                      size="sm"
                      intent={anyHidden ? "primary" : "gray-subtle"}
                      disabled={isBusy || files.length === 0}
                      onClick={() => {
                        if (anyHidden) {
                          unhideSeries(
                            { mediaId: mediaIdNum },
                            { onSuccess: () => { toast.success("Series unhidden"); refetch() }, onError: (e:any) => toast.error(e?.message || "Failed to unhide") }
                          )
                        } else {
                          if (!window.confirm(`Hide series \"${title}\"? It will be excluded unless you choose to show hidden.`)) return
                          hideSeries(
                            { mediaId: mediaIdNum },
                            { onSuccess: () => { toast.success("Series hidden"); refetch() }, onError: (e:any) => toast.error(e?.message || "Failed to hide") }
                          )
                        }
                      }}
                    >{anyHidden ? "Unhide" : "Hide series"}</Button>
                    <Button
                      size="sm"
                      intent="gray-subtle"
                      disabled={isBusy || files.length === 0}
                      onClick={() => {
                        if (!window.confirm(`Move & rename all main episodes for media ${mediaIdNum}?`)) return
                        const doDelete = window.confirm("Also delete the original release folder(s) after successful moves?")
                        moveRename(
                          { mediaId: mediaIdNum, confirmDelete: doDelete },
                          {
                            onSuccess: (res) => {
                              const moved = res?.moved ?? 0
                              const skipped = res?.skipped ?? 0
                              const delCount = res?.deletedFolders?.length ?? 0
                              const errCount = res?.errors?.length ?? 0
                              toast.success(`Moved: ${moved}, skipped: ${skipped}${delCount ? ", deleted folders: " + delCount : ""}`)
                              if (errCount) toast.warning(`Some errors occurred: ${errCount}`)
                              refetch()
                            },
                            onError: (err: any) => toast.error(err?.message || "Move & rename failed"),
                          }
                        )
                      }}
                    >
                      Move & Rename
                    </Button>
                    <Button
                      size="sm"
                      intent="alert"
                      disabled={isBusy || files.length === 0}
                      onClick={() => {
                        if (!window.confirm(`Delete all ${files.length} file${files.length === 1 ? "" : "s"} in this series? This cannot be undone.`)) return
                        const paths = files.map((f: any) => f.path)
                        // Optimistic UI: remove this group's items immediately
                        setLocalItems(prev => prev.filter((it: any) => it.mediaId !== mediaIdNum))
                        deleteFiles(
                          { paths },
                          {
                            onSuccess: () => refetch(),
                            onError: () => refetch(),
                          }
                        )
                      }}
                    >Delete series</Button>
                    <Button
                      size="sm"
                      intent="white"
                      leftIcon={<TbUnlink />}
                      disabled={isBusy || files.length === 0}
                      onClick={() => {
                        if (!window.confirm(`Unlink all ${files.length} file${files.length === 1 ? "" : "s"} in this series?`)) return
                        // Optimistic UI: remove this group's items immediately
                        setLocalItems(prev => prev.filter((it: any) => it.mediaId !== mediaIdNum))
                        unlinkFiles(
                          { paths, blockAutoRematch: true },
                          {
                            onSuccess: () => refetch(),
                            onError: () => refetch(),
                          }
                        )
                      }}
                     >Unlink series</Button>
                  </div>
                </div>
                {/* Group items */}
                <div className="divide-y divide-[--border]">
                  {files
                    .slice()
                    .sort((a, b) => String(a.path).localeCompare(String(b.path)))
                    .map((lf, idx) => (
                    <div key={`${lf.path}-${idx}`} className="p-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <div className="flex-1 min-w-[280px]">
                          <div className="font-mono break-all">{upath.basename(lf.path)}</div>
                          <div className="opacity-70 text-xs break-all">{lf.path}</div>
                        </div>
                        <Badge intent={lf.linkSource === "manual" ? "success" : "primary"}>{lf.linkSource || "unknown"}</Badge>
                        {lf.autoMatchBlocked && <Badge intent="warning">Blocked</Badge>}

                        <div className="flex items-center gap-2 ml-auto w-[680px] max-w-full justify-end">
                          <Button
                            size="sm"
                            intent="white"
                            leftIcon={<TbUnlink />}
                            disabled={isBusy}
                            onClick={() => {
                              if (!window.confirm("Unlink this file? You can block automatic rematching for auto-links.")) return
                              // Optimistic UI: remove this single item immediately
                              setLocalItems(prev => prev.filter((it: any) => it.path !== lf.path))
                              unlinkFiles(
                                { paths: [lf.path], blockAutoRematch: true },
                                {
                                  onSuccess: () => refetch(),
                                  onError: () => refetch(),
                                }
                              )
                            }}
                          >Unlink</Button>
                          <Button
                            size="sm"
                            intent="alert"
                            disabled={isBusy}
                            onClick={() => {
                              if (!window.confirm("Delete this file? This cannot be undone.")) return
                              // Optimistic UI: remove this single item immediately
                              setLocalItems(prev => prev.filter((it: any) => it.path !== lf.path))
                              deleteFiles(
                                { paths: [lf.path] },
                                {
                                  onSuccess: () => refetch(),
                                  onError: () => refetch(),
                                }
                              )
                            }}
                          >Delete</Button>
                          <div className="flex items-center gap-2">
                            <span className="text-xs opacity-80">Block auto</span>
                            <Switch
                              value={!!lf.autoMatchBlocked}
                              onValueChange={(checked: boolean) => {
                                toggleBlock({ path: lf.path, autoMatchBlocked: checked }, { onSuccess: () => refetch() })
                              }}
                            />
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>

        <div className="flex items-center justify-between">
          <div className="text-xs opacity-70">Page {data?.page ?? page} / {Math.max(1, Math.ceil((data?.total ?? 0) / (data?.pageSize ?? pageSize)))}</div>
          <div className="flex gap-2">
            <Button intent="gray-subtle" disabled={page <= 1 || isBusy} onClick={() => setPage(p => Math.max(1, p - 1))}>Previous</Button>
            <Button intent="gray-subtle" disabled={((data?.page ?? page) * (data?.pageSize ?? pageSize)) >= (data?.total ?? 0) || isBusy} onClick={() => setPage(p => p + 1)}>Next</Button>
          </div>
        </div>
      </AppLayoutStack>
    </Drawer>
  )
}
