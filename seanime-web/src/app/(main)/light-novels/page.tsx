"use client"

import React from "react"
import Link from "next/link"
import Image from "next/image"
import { useGetLightNovelSeries } from "@/api/hooks/light_novels.hooks"

export default function LightNovelsPage() {
  const { data, isLoading, error } = useGetLightNovelSeries()

  return (
    <div className="container mx-auto px-4 py-6">
      <h1 className="text-2xl font-semibold mb-2">Light Novels</h1>
      <p className="text-muted-foreground mb-6">Browse your local light novel series.</p>

      {isLoading && (
        <div className="text-sm text-muted-foreground">Loading light novels…</div>
      )}
      {error && (
        <div className="text-sm text-red-500">Failed to load light novels.</div>
      )}

      <div className="grid gap-4 grid-cols-[repeat(auto-fill,minmax(160px,1fr))]">
        {data?.map((s) => (
          <Link key={s.id} href={`/light-novels/entry?id=${encodeURIComponent(s.id)}`} className="group block">
            <div className="relative aspect-[2/3] overflow-hidden rounded-md bg-muted">
              {s.coverUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={s.coverUrl} alt={s.title} className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.03]" />
              ) : (
                <div className="h-full w-full grid place-items-center text-muted-foreground">No cover</div>
              )}
            </div>
            <div className="mt-2">
              <div className="text-sm font-medium line-clamp-2">{s.title}</div>
              <div className="text-xs text-muted-foreground">{s.volumeCount} volume{s.volumeCount === 1 ? "" : "s"}</div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
