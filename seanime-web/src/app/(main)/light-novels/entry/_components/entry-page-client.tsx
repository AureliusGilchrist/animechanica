"use client"

import React from "react"
import Link from "next/link"
import { useSearchParams, useRouter } from "next/navigation"
import { buildLightNovelEpubUrl, useGetLightNovelSeriesDetails } from "@/api/hooks/light_novels.hooks"
import { useDeleteLightNovelBookmark, useGetLightNovelBookmarks, useSaveLightNovelBookmark } from "@/api/hooks/light_novels_bookmarks.hooks"

export function LightNovelEntryPageClient() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const id = searchParams.get("id") ?? undefined
  const { data, isLoading, error } = useGetLightNovelSeriesDetails(id)

  React.useEffect(() => {
    if (!id) router.push("/light-novels")
  }, [id])

  return (
    <div className="container mx-auto px-4 py-6">
      <h1 className="text-2xl font-semibold mb-2">Series</h1>
      <p className="text-muted-foreground mb-6">Volumes available in this series.</p>

      {isLoading && <div className="text-sm text-muted-foreground">Loading volumes…</div>}
      {error && <div className="text-sm text-red-500">Failed to load volumes.</div>}

      <div className="space-y-2">
        {data?.map((v) => (
          <VolumeRow key={v.path} seriesId={id!} fileName={v.fileName} size={v.size} path={v.path} />
        ))}
      </div>

      <div className="mt-6">
        <Link href="/light-novels" className="text-sm text-muted-foreground hover:underline">
          ← Back to Light Novels
        </Link>
      </div>
    </div>
  )
}

function VolumeRow({ seriesId, fileName, size, path }: { seriesId: string; fileName: string; size: number; path: string }) {
  const { data: bookmarks } = useGetLightNovelBookmarks(seriesId, fileName)
  const del = useDeleteLightNovelBookmark()
  const save = useSaveLightNovelBookmark()
  const [showForm, setShowForm] = React.useState(false)
  const [percentStr, setPercentStr] = React.useState("")
  const [label, setLabel] = React.useState("")
  const [cfi, setCfi] = React.useState("")

  return (
    <div className="rounded-md border p-3">
      <div className="flex items-center justify-between">
        <div className="min-w-0">
          <div className="text-sm font-medium truncate">{fileName}</div>
          <div className="text-xs text-muted-foreground">{(size / (1024 * 1024)).toFixed(1)} MB</div>
        </div>
        <div className="ml-4 shrink-0">
          <a
            className="inline-flex items-center rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground hover:opacity-90"
            href={buildLightNovelEpubUrl(path)}
            target="_blank"
            rel="noreferrer"
          >
            Read
          </a>
        </div>
      </div>
      <div className="mt-3">
        <div className="text-xs font-medium text-muted-foreground mb-1">Bookmarks</div>
        {bookmarks && bookmarks.length > 0 ? (
          <ul className="space-y-1">
            {bookmarks.map((b) => (
              <li key={b.id} className="flex items-center justify-between text-sm">
                <div className="truncate">
                  <span className="text-muted-foreground">{Math.round((b.percent ?? 0) * 100)}%</span>
                  {b.label ? <span className="ml-2">{b.label}</span> : null}
                </div>
                <div className="flex items-center gap-3">
                  <button
                    className="text-xs text-blue-500 hover:underline"
                    onClick={async () => {
                      const newLabel = window.prompt("Edit label", b.label ?? "") ?? undefined
                      if (newLabel === undefined) return
                      const cfi = b.cfi ?? ""
                      save.mutate({ seriesId, volume: fileName, id: b.id, cfi, percent: b.percent ?? 0, label: newLabel || undefined })
                    }}
                  >
                    Edit label
                  </button>
                  <button
                    className="text-xs text-red-500 hover:underline"
                    onClick={() => del.mutate({ seriesId, volume: fileName, id: b.id })}
                  >
                    Delete
                  </button>
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <div className="text-xs text-muted-foreground">No bookmarks.</div>
        )}
        <div className="mt-2">
          {showForm ? (
            <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
              <div>
                <label className="block text-xs text-muted-foreground">Percent (0-100)</label>
                <input
                  value={percentStr}
                  onChange={(e) => setPercentStr(e.target.value)}
                  placeholder="e.g. 42"
                  className="h-8 rounded-md border px-2 text-sm"
                  inputMode="numeric"
                />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground">Label</label>
                <input
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                  placeholder="Optional note"
                  className="h-8 rounded-md border px-2 text-sm"
                />
              </div>
              <div className="flex-1 min-w-0">
                <label className="block text-xs text-muted-foreground">CFI (optional)</label>
                <input
                  value={cfi}
                  onChange={(e) => setCfi(e.target.value)}
                  placeholder="Reader position CFI"
                  className="w-full h-8 rounded-md border px-2 text-sm"
                />
              </div>
              <div className="flex items-center gap-2">
                <button
                  className="h-8 rounded-md bg-primary px-3 text-sm text-primary-foreground hover:opacity-90"
                  onClick={() => {
                    const p = Number(percentStr)
                    if (Number.isNaN(p) || p < 0 || p > 100) {
                      alert("Please enter a valid percent between 0 and 100")
                      return
                    }
                    save.mutate({ seriesId, volume: fileName, cfi, percent: p / 100, label: label || undefined }, {
                      onSuccess: () => {
                        setShowForm(false)
                        setPercentStr("")
                        setLabel("")
                        setCfi("")
                      }
                    })
                  }}
                >
                  Save
                </button>
                <button
                  className="h-8 rounded-md border px-3 text-sm hover:bg-accent"
                  onClick={() => {
                    setShowForm(false)
                    setPercentStr("")
                    setLabel("")
                    setCfi("")
                  }}
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <button
              className="mt-1 text-xs text-muted-foreground hover:underline"
              onClick={() => setShowForm(true)}
            >
              + Add bookmark
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
