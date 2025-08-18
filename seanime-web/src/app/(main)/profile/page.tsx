"use client"

import { PageWrapper } from "@/components/shared/page-wrapper"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { useServerStatus } from "@/app/(main)/_hooks/use-server-status"
import React from "react"

export const dynamic = "force-static"

export default function ProfilePage() {
    // const serverStatus = useServerStatus() // not used on profile anymore

    const [stats, setStats] = React.useState<any>(null)
    const [viewer, setViewer] = React.useState<any>(null)
    const [loading, setLoading] = React.useState(true)
    const [error, setError] = React.useState<string | null>(null)

    React.useEffect(() => {
        let abort = false
        async function load() {
            try {
                setLoading(true)
                const [s, v] = await Promise.all([
                    fetch("/api/v1/anilist/stats").then(r => r.ok ? (r.json() as any) : Promise.reject(new Error("stats failed"))),
                    fetch("/api/v1/anilist/viewer").then(r => r.ok ? (r.json() as any) : Promise.reject(new Error("viewer failed"))),
                ])
                if (!abort) {
                    setStats(s?.data ?? s ?? null)
                    setViewer(v?.data?.Viewer ?? v?.Viewer ?? null)
                    setError(null)
                }
            } catch (e: any) {
                if (!abort) setError(e?.message ?? "Failed to load profile data")
            } finally {
                if (!abort) setLoading(false)
            }
        }
        load()
        return () => { abort = true }
    }, [])

    const [rematching, setRematching] = React.useState(false)
    const [rematchMsg, setRematchMsg] = React.useState<string | null>(null)

    async function handleRematchAll() {
        setRematchMsg(null)
        setRematching(true)
        try {
            const res = await fetch("/api/v1/library/rematch-anime-links", { method: "POST" })
            if (!res.ok) throw new Error(`HTTP ${res.status}`)
            // Optionally we could refresh stats or notify
            setRematchMsg("Rematch started and scan completed. Library links updated.")
        } catch (e: any) {
            setRematchMsg(`Failed to rematch: ${e?.message ?? "Unknown error"}`)
        } finally {
            setRematching(false)
        }
    }

    return (
        <>
            <PageWrapper
                className="p-4 sm:p-8 pt-4 relative"
                data-profile-page
                {...{
                    initial: { opacity: 0, y: 10 },
                    animate: { opacity: 1, y: 0 },
                    exit: { opacity: 0, y: 10 },
                    transition: { type: "spring", damping: 20, stiffness: 100 },
                }}
            >
                <ProfileHeader />

                <AppLayoutStack className="mt-6 space-y-8">
                    {/* Library Tools */}
                    <section>
                        <h2 className="text-lg font-semibold mb-3">Library Tools</h2>
                        <div className="flex items-center gap-3">
                            <button
                                onClick={handleRematchAll}
                                disabled={rematching}
                                className="px-3 py-2 rounded border bg-[--muted]/10 hover:bg-[--muted]/20 disabled:opacity-60"
                                title="Bulk unmatch all anime links and rescan to rematch using Romaji-prioritized matcher"
                            >
                                {rematching ? "Running rematch…" : "Rematch Anime Links"}
                            </button>
                            {rematchMsg && (
                                <span className="text-sm text-[--muted]">{rematchMsg}</span>
                            )}
                        </div>
                    </section>

                    {error && (
                        <div className="text-red-500 text-sm">{error}</div>
                    )}

                    {/* Statistics */}
                    <section>
                        <h2 className="text-lg font-semibold mb-3">Statistics</h2>
                        {loading ? (
                            <div className="text-[--muted] text-sm">Loading…</div>
                        ) : (
                            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
                                <StatCard label="Anime Count" value={stats?.anime?.count ?? 0} />
                                <StatCard label="Episodes Watched" value={stats?.anime?.episodesWatched ?? 0} />
                                <StatCard label="Minutes Watched" value={stats?.anime?.minutesWatched ?? 0} />
                                <StatCard label="Manga Count" value={stats?.manga?.count ?? 0} />
                                <StatCard label="Chapters Read" value={stats?.manga?.chaptersRead ?? 0} />
                                <StatCard label="Volumes Read" value={stats?.manga?.volumesRead ?? 0} />
                            </div>
                        )}
                    </section>

                    {/* Favourites */}
                    <section className="space-y-6">
                        <h2 className="text-lg font-semibold">Favourites</h2>
                        {/* Favourite Anime */}
                        {!!viewer?.favourites?.anime?.nodes?.length && (
                            <div>
                                <h3 className="font-medium mb-2">Anime</h3>
                                <FavGrid items={viewer.favourites.anime.nodes} type="anime" />
                            </div>
                        )}
                        {/* Favourite Manga */}
                        {!!viewer?.favourites?.manga?.nodes?.length && (
                            <div>
                                <h3 className="font-medium mb-2">Manga</h3>
                                <FavGrid items={viewer.favourites.manga.nodes} type="manga" />
                            </div>
                        )}
                        {/* Favourite Characters */}
                        {!!viewer?.favourites?.characters?.nodes?.length && (
                            <div>
                                <h3 className="font-medium mb-2">Characters</h3>
                                <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-3">
                                    {viewer.favourites.characters.nodes.map((c: any) => (
                                        <div key={c.id} className="text-sm">
                                            <img src={c?.image?.large || c?.image?.medium || "/logo.png"} className="w-full aspect-[3/4] object-cover rounded border" />
                                            <div className="mt-1 truncate" title={c?.name?.full || c?.name?.native || ""}>{c?.name?.full || c?.name?.native || ""}</div>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )}
                        {/* Favourite Studios */}
                        {!!viewer?.favourites?.studios?.nodes?.length && (
                            <div>
                                <h3 className="font-medium mb-2">Studios</h3>
                                <div className="flex flex-wrap gap-2">
                                    {viewer.favourites.studios.nodes.map((s: any) => (
                                        <span key={s.id} className="px-2 py-1 rounded border bg-[--muted]/20 text-sm">{s?.name}</span>
                                    ))}
                                </div>
                            </div>
                        )}
                    </section>
                </AppLayoutStack>
            </PageWrapper>
        </>
    )
}

function StatCard({ label, value }: { label: string; value: number }) {
    return (
        <div className="p-3 rounded-lg border bg-[--muted]/10">
            <div className="text-[--muted] text-xs">{label}</div>
            <div className="text-lg font-semibold">{Number(value ?? 0).toLocaleString()}</div>
        </div>
    )
}

function FavGrid({ items, type }: { items: any[]; type: "anime" | "manga" }) {
    return (
        <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-3">
            {items?.map((m: any) => (
                <div key={m?.id} className="text-sm">
                    <img src={m?.coverImage?.large || m?.coverImage?.extraLarge || m?.coverImage?.medium || "/logo.png"} className="w-full aspect-[3/4] object-cover rounded border" />
                    <div className="mt-1 truncate" title={m?.title?.english || m?.title?.romaji || m?.title?.native || ""}>{m?.title?.english || m?.title?.romaji || m?.title?.native || ""}</div>
                </div>
            ))}
        </div>
    )
}

function ProfileHeader() {
    const serverStatus = useServerStatus()
    const viewer = serverStatus?.user?.viewer
    const avatar = viewer?.avatar?.large || viewer?.avatar?.medium || "/logo.png"
    const banner = viewer?.bannerImage
    const name = viewer?.name || "Guest"

    return (
        <div className="relative overflow-hidden rounded-xl bg-[--muted]/20 border">
            {banner ? (
                <div
                    className="h-24 sm:h-32 w-full bg-cover bg-center"
                    style={{ backgroundImage: `url(${banner})` }}
                />
            ) : (
                <div className="h-24 sm:h-32 w-full bg-gradient-to-r from-indigo-600/40 via-purple-600/30 to-cyan-500/30" />
            )}
            <div className="p-4 sm:p-6 -mt-8 flex items-end gap-4">
                <img
                    src={avatar}
                    alt="avatar"
                    className="w-16 h-16 rounded-lg border bg-[--background] object-cover"
                />
                <div className="pb-1">
                    <h1 className="text-xl sm:text-2xl font-semibold">{name}</h1>
                    <p className="text-[--muted] text-sm">AniList Profile</p>
                </div>
            </div>
        </div>
    )
}
