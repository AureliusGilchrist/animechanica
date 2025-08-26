import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

export type LightNovelBookmark = {
  id: string
  cfi: string
  percent: number
  label?: string
  createdAt: number
  updatedAt: number
}

export function useGetLightNovelBookmarks(seriesId: string | undefined, volume: string | undefined) {
  return useQuery<LightNovelBookmark[]>({
    queryKey: ["ln-bookmarks", seriesId, volume],
    enabled: !!seriesId && !!volume,
    queryFn: async () => {
      const params = new URLSearchParams()
      params.set("seriesId", seriesId!)
      params.set("volume", volume!)
      const res = await fetch(`/api/v1/light-novels/bookmarks?${params.toString()}`)
      if (!res.ok) throw new Error("Failed to fetch bookmarks")
      return (await res.json()) as LightNovelBookmark[]
    },
    refetchOnWindowFocus: false,
    staleTime: 60_000,
  })
}

export function useSaveLightNovelBookmark() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: { seriesId: string; volume: string; id?: string; cfi: string; percent: number; label?: string }) => {
      const res = await fetch(`/api/v1/light-novels/bookmarks`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      })
      if (!res.ok) throw new Error("Failed to save bookmark")
      return (await res.json()) as LightNovelBookmark[]
    },
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ["ln-bookmarks", vars.seriesId, vars.volume] })
    },
  })
}

export function useDeleteLightNovelBookmark() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: { seriesId: string; volume: string; id: string }) => {
      const res = await fetch(`/api/v1/light-novels/bookmarks`, {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      })
      if (!res.ok) throw new Error("Failed to delete bookmark")
      return (await res.json()) as LightNovelBookmark[]
    },
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ["ln-bookmarks", vars.seriesId, vars.volume] })
    },
  })
}
