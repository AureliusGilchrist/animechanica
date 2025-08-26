import { useServerQuery, useServerMutation } from "@/api/client/requests"
import { Anime_LocalFile } from "@/api/generated/types"

export type LinkedAnimeListParams = {
  source?: "any" | "auto" | "manual"
  blocked?: "any" | "true" | "false"
  mediaId?: number
  page?: number
  pageSize?: number
  hidden?: "include" | "exclude" | "only"
  includeUnmatched?: boolean
}

export type LinkedLocalFile = Anime_LocalFile & {
  linkSource?: "auto" | "manual"
  autoMatchBlocked?: boolean
}

export type LinkedAnimeListResponse = {
  items: LinkedLocalFile[]
  total: number
  page: number
  pageSize: number
}

const buildQueryString = (params?: LinkedAnimeListParams) => {
  const p = new URLSearchParams()
  if (!params) return ""
  if (params.source) p.set("source", params.source)
  if (params.blocked) p.set("blocked", params.blocked)
  if (params.mediaId) p.set("mediaId", String(params.mediaId))
  if (params.page) p.set("page", String(params.page))
  if (params.pageSize) p.set("pageSize", String(params.pageSize))
  if (params.hidden) p.set("hidden", params.hidden)
  if (params.includeUnmatched) p.set("includeUnmatched", "true")
  const s = p.toString()
  return s ? `?${s}` : ""
}

export function useListLinkedAnimeFiles(params?: LinkedAnimeListParams) {
  const endpoint = "/api/v1/library/anime-linked" + buildQueryString(params)
  return useServerQuery<LinkedAnimeListResponse>({
    endpoint,
    method: "GET",
    // Key must include params to cache per filter/page
    queryKey: ["LIBRARY-linked-anime", params ?? {}],
    enabled: true,
  })
}

// Convenience mutation for per-path unlink with optional block flag.
export type UnlinkLocalFilesVars = {
  paths: string[]
  blockAutoRematch?: boolean
}

export function useUnlinkLocalFiles() {
  // Uses anime-entry/unmatch which accepts { mediaId? or paths, blockAutoRematch? }
  return useServerMutation<boolean, UnlinkLocalFilesVars>({
    endpoint: "/api/v1/library/anime-entry/unmatch",
    method: "POST",
    mutationKey: ["LOCALFILES-unlink"],
  })
}

export type ToggleBlockVars = { path: string; value: boolean }

export function useToggleAutoMatchBlocked() {
  // Sends minimal patch with path + autoMatchBlocked
  return useServerMutation<boolean, { path: string; autoMatchBlocked: boolean }>({
    endpoint: "/api/v1/library/local-file",
    method: "PATCH",
    mutationKey: ["LOCALFILES-toggle-auto-block"],
  })
}

// Move & Rename a linked anime series' main episode files
export type MoveRenameSeriesVars = {
  mediaId: number
  confirmDelete?: boolean
  dryRun?: boolean
}

export type MoveRenameSeriesResponse = {
  moved: number
  skipped: number
  deletedFolders: string[]
  errors: string[]
}

export function useMoveRenameAnimeSeries() {
  return useServerMutation<MoveRenameSeriesResponse, MoveRenameSeriesVars>({
    endpoint: "/api/v1/library/anime-entry/move-rename",
    method: "POST",
    mutationKey: ["LIBRARY-move-rename-series"],
  })
}

// Hide / Unhide a series (per mediaId)
export function useHideAnimeSeries() {
  return useServerMutation<boolean, { mediaId: number }>({
    endpoint: "/api/v1/library/anime-entry/hide",
    method: "POST",
    mutationKey: ["LIBRARY-hide-series"],
  })
}

export function useUnhideAnimeSeries() {
  return useServerMutation<boolean, { mediaId: number }>({
    endpoint: "/api/v1/library/anime-entry/unhide",
    method: "POST",
    mutationKey: ["LIBRARY-unhide-series"],
  })
}
