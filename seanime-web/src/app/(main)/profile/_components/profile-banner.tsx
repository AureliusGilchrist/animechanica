"use client"

import React from "react"

export function ProfileBanner({ userId, onMetaChange }: { userId?: number; onMetaChange?: (meta: { hasBanner: boolean; hasBio: boolean }) => void }) {
  const userKey = userId ? `u${userId}` : "local"
  const BANNER_KEY = `seanime.profile.${userKey}.banner`
  const BIO_KEY = `seanime.profile.${userKey}.bio`

  const [banner, setBanner] = React.useState<string | null>(null)
  const [bio, setBio] = React.useState("")
  const fileRef = React.useRef<HTMLInputElement | null>(null)

  React.useEffect(() => {
    try {
      const b64 = localStorage.getItem(BANNER_KEY)
      if (b64) setBanner(b64)
      const bioSaved = localStorage.getItem(BIO_KEY)
      if (bioSaved) setBio(bioSaved)
    } catch {}
  }, [BANNER_KEY, BIO_KEY])

  React.useEffect(() => {
    const id = setTimeout(() => {
      try { localStorage.setItem(BIO_KEY, bio) } catch {}
    }, 300)
    return () => clearTimeout(id)
  }, [bio])

  React.useEffect(() => {
    onMetaChange?.({ hasBanner: !!banner, hasBio: !!bio?.trim() })
  }, [banner, bio, onMetaChange])

  function onPickFile() {
    fileRef.current?.click()
  }

  function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    if (!f) return
    const reader = new FileReader()
    reader.onload = () => {
      const result = reader.result as string
      setBanner(result)
      try { localStorage.setItem(BANNER_KEY, result) } catch {}
    }
    reader.readAsDataURL(f)
  }

  function onRemove() {
    setBanner(null)
    try { localStorage.removeItem(BANNER_KEY) } catch {}
  }

  return (
    <div className="relative overflow-hidden rounded-2xl border bg-card/60 backdrop-blur supports-[backdrop-filter]:backdrop-blur-lg">
      <div className="relative h-40 sm:h-56 md:h-64">
        {banner ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={banner} alt="Profile banner" className="absolute inset-0 h-full w-full object-cover" />
        ) : (
          <div className="absolute inset-0 grid place-items-center bg-gradient-to-br from-[--card]/60 to-transparent">
            <div className="text-center text-sm text-muted-foreground">
              Add a personal banner to customize your profile
            </div>
          </div>
        )}
        <div className="absolute inset-0 bg-gradient-to-t from-background/70 to-transparent" />
        <div className="absolute right-3 top-3 flex gap-2">
          <button onClick={onPickFile} className="rounded-full border bg-background/70 px-3 py-1 text-xs hover:bg-background">Upload</button>
          {banner && (
            <button onClick={onRemove} className="rounded-full border bg-background/70 px-3 py-1 text-xs hover:bg-background">Remove</button>
          )}
          <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={onFileChange} />
        </div>
      </div>

      <div className="p-4 sm:p-6">
        <h2 className="text-lg font-semibold">About You</h2>
        <p className="text-xs text-muted-foreground mb-2">Write a short description for your profile.</p>
        <textarea
          value={bio}
          onChange={(e) => setBio(e.target.value)}
          placeholder="Tell the world about your anime and manga journey..."
          className="mt-1 w-full min-h-[88px] rounded-xl border bg-background/60 p-3 text-sm outline-none focus:ring-2 focus:ring-[--ring]"
        />
      </div>
    </div>
  )
}
