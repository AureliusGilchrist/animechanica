"use client"

import { useEffect, useMemo, useRef, useState } from "react"

// Simple in-memory cache for the session
const posterCache = new Map<string, string | null>()

// Persistent cache (localStorage) helpers
const LS_PREFIX = "kitsuPoster:"
const DEFAULT_TTL_MS = 1000 * 60 * 60 * 24 * 300 // 300 days

type StoredPoster = {
  url: string | null
  ts: number // epoch ms when stored
}

function lsKey(key: string) {
  return `${LS_PREFIX}${key}`
}

function readFromLocalStorage(key: string, ttlMs = DEFAULT_TTL_MS): string | null | undefined {
  if (typeof window === "undefined") return undefined
  try {
    const raw = window.localStorage.getItem(lsKey(key))
    if (!raw) return undefined
    const parsed = JSON.parse(raw) as StoredPoster
    if (!parsed || typeof parsed.ts !== "number") return undefined
    const age = Date.now() - parsed.ts
    if (age > ttlMs) {
      // expired, clean up
      window.localStorage.removeItem(lsKey(key))
      return undefined
    }
    return parsed.url ?? null
  } catch {
    return undefined
  }
}

function writeToLocalStorage(key: string, url: string | null) {
  if (typeof window === "undefined") return
  try {
    const payload: StoredPoster = { url, ts: Date.now() }
    window.localStorage.setItem(lsKey(key), JSON.stringify(payload))
  } catch {
    // ignore quota/JSON errors
  }
}

// Normalize titles for matching/caching
function normTitle(t: string) {
  return t.trim().toLowerCase()
}

// Kitsu API response typing (minimal)
interface KitsuPosterImage {
  tiny?: string
  small?: string
  medium?: string
  large?: string
  original?: string
}
interface KitsuMangaAttributes {
  canonicalTitle?: string
  titles?: Record<string, string>
  posterImage?: KitsuPosterImage
}
interface KitsuMangaItem {
  id: string
  attributes: KitsuMangaAttributes
}
interface KitsuSearchResponse {
  data: KitsuMangaItem[]
}

/**
 * useKitsuPoster
 * Fetches a Kitsu poster image URL for a given manga title. No auth required.
 * Returns best-available size; caches results in-memory to avoid repeat requests.
 */
export function useKitsuPoster(title: string | undefined | null) {
  const [url, setUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const key = useMemo(() => (title ? normTitle(title) : ""), [title])

  useEffect(() => {
    if (!key) {
      setUrl(null)
      setError(null)
      return
    }

    // Serve from cache if present
    if (posterCache.has(key)) {
      setUrl(posterCache.get(key) ?? null)
      setError(null)
      return
    }

    // Try persistent cache (localStorage) before network
    const lsCached = readFromLocalStorage(key)
    if (lsCached !== undefined) {
      posterCache.set(key, lsCached)
      setUrl(lsCached)
      setError(null)
      return
    }

    // Abort any inflight request
    if (abortRef.current) abortRef.current.abort()
    const controller = new AbortController()
    abortRef.current = controller

    async function run() {
      try {
        setLoading(true)
        setError(null)

        const qs = new URLSearchParams({ "filter[text]": title ?? "" })
        const resp = await fetch(`https://kitsu.io/api/edge/manga?${qs.toString()}`, {
          signal: controller.signal,
          headers: {
            Accept: "application/vnd.api+json",
          },
        })
        if (!resp.ok) throw new Error(`Kitsu HTTP ${resp.status}`)
        const json = (await resp.json()) as KitsuSearchResponse
        const items = json?.data ?? []

        let best: KitsuPosterImage | undefined
        if (items.length > 0) {
          // Try to pick best match by canonicalTitle or any titles containing our query
          const nQuery = normTitle(title || "")
          const scored = items.map((it) => {
            const a = it.attributes || {}
            const ct = a.canonicalTitle || ""
            const titles = [ct, ...(Object.values(a.titles || {}))]
            const hit = titles.some((t) => normTitle(t || "").includes(nQuery))
            return { it, hit }
          })
          const exact = scored.find((s) => s.hit)
          const chosen = (exact?.it ?? items[0])
          best = chosen.attributes?.posterImage
        }

        const selected = best?.original || best?.large || best?.medium || best?.small || best?.tiny || null
        posterCache.set(key, selected)
        writeToLocalStorage(key, selected)
        setUrl(selected)
      } catch (e: any) {
        if (e?.name === "AbortError") return
        setError(e?.message || "Failed to fetch Kitsu poster")
        posterCache.set(key, null)
        writeToLocalStorage(key, null)
        setUrl(null)
      } finally {
        setLoading(false)
      }
    }

    run()

    return () => {
      controller.abort()
    }
  }, [key, title])

  return { url, loading, error }
}
