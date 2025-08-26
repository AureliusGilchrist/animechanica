import { useQuery, UseQueryResult } from "@tanstack/react-query"

export type LightNovelSeries = {
  id: string
  title: string
  coverUrl?: string
  volumeCount: number
}

export type LightNovelVolume = {
  fileName: string
  path: string
  size: number
}

export function useGetLightNovelSeries(): UseQueryResult<LightNovelSeries[]> {
  return useQuery<LightNovelSeries[]>({
    queryKey: ["light-novel-series"],
    queryFn: async () => {
      const res = await fetch("/api/v1/light-novels/series")
      if (!res.ok) throw new Error("Failed to fetch light novel series")
      return (await res.json()) as LightNovelSeries[]
    },
    refetchOnWindowFocus: false,
    staleTime: 5 * 60 * 1000,
  })
}

export function useGetLightNovelSeriesDetails(id: string | undefined): UseQueryResult<LightNovelVolume[]> {
  return useQuery<LightNovelVolume[]>({
    queryKey: ["light-novel-series-details", id],
    enabled: !!id,
    queryFn: async () => {
      const res = await fetch(`/api/v1/light-novels/series/${encodeURIComponent(id!)}`)
      if (!res.ok) throw new Error("Failed to fetch light novel series details")
      return (await res.json()) as LightNovelVolume[]
    },
    refetchOnWindowFocus: false,
    staleTime: 5 * 60 * 1000,
  })
}

export function buildLightNovelEpubUrl(path: string) {
  const params = new URLSearchParams()
  params.set("path", path)
  return `/api/v1/light-novels/epub?${params.toString()}`
}
