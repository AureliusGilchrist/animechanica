import { AL_AnimeDetailsById_Media, Anime_Entry, Nullish } from "@/api/generated/types"
import { MediaCardGrid } from "@/app/(main)/_features/media/_components/media-card-grid"
import { MediaEntryCard } from "@/app/(main)/_features/media/_components/media-entry-card"
import { useServerStatus } from "@/app/(main)/_hooks/use-server-status"
import capitalize from "lodash/capitalize"
import React from "react"

type RelationsRecommendationsSectionProps = {
    entry: Nullish<Anime_Entry>
    details: Nullish<AL_AnimeDetailsById_Media>
    containerRef?: React.RefObject<HTMLElement>
}

export function RelationsRecommendationsSection(props: RelationsRecommendationsSectionProps) {

    const {
        entry,
        details,
        containerRef,
        ...rest
    } = props

    const serverStatus = useServerStatus()

    const sourceManga = React.useMemo(() => {
        return serverStatus?.settings?.library?.enableManga
            ? details?.relations?.edges?.find(edge => (edge?.relationType === "SOURCE" || edge?.relationType === "ADAPTATION") && edge?.node?.format === "MANGA")?.node
            : undefined
    }, [details?.relations?.edges, serverStatus?.settings?.library?.enableManga])

    const relations = React.useMemo(() => (details?.relations?.edges?.map(edge => edge) || [])
        .filter(Boolean)
            .filter(n => (n.node?.format === "TV" || n.node?.format === "OVA" || n.node?.format === "MOVIE" || n.node?.format === "SPECIAL") && (n.relationType === "PREQUEL" || n.relationType === "SEQUEL" || n.relationType === "PARENT" || n.relationType === "SIDE_STORY" || n.relationType === "ALTERNATIVE" || n.relationType === "ADAPTATION")),
        [details?.relations?.edges])

    const recommendations = React.useMemo(() => details?.recommendations?.edges?.map(edge => edge?.node?.mediaRecommendation)?.filter(Boolean) || [],
        [details?.recommendations?.edges])

    // Map of mediaId -> folder exists
    const [recFolderExists, setRecFolderExists] = React.useState<Record<number, boolean>>({})
    // LocalStorage cache for positive hits so we don't recheck repeatedly
    const CACHE_KEY = "recFolderExistsCacheV2"
    const loadCache = React.useCallback((): Record<number, boolean> => {
        try {
            const raw = localStorage.getItem(CACHE_KEY)
            if (!raw) return {}
            const parsed = JSON.parse(raw) as unknown
            if (parsed && typeof parsed === "object") {
                // Ensure values are booleans and keys numeric
                const out: Record<number, boolean> = {}
                const obj = parsed as Record<string, unknown>
                Object.keys(obj).forEach(k => {
                    const id = Number(k)
                    if (!Number.isNaN(id) && !!obj[k]) out[id] = true
                })
                return out
            }
        } catch (_) { /* ignore */ }
        return {}
    }, [])
    const saveCache = React.useCallback((cache: Record<number, boolean>) => {
        try { localStorage.setItem(CACHE_KEY, JSON.stringify(cache)) } catch (_) { /* ignore */ }
    }, [])

    // Fetch folder existence for each recommended anime using backend endpoint (with positive cache)
    React.useEffect(() => {
        const ids = (recommendations || []).map(m => m!.id).filter(Boolean) as number[]
        if (!ids.length) {
            setRecFolderExists({})
            return
        }
        // Prefill from cache
        const cache = loadCache()
        const initial: Record<number, boolean> = {}
        ids.forEach(id => { if (cache[id]) initial[id] = true })
        if (Object.keys(initial).length) setRecFolderExists(prev => ({ ...prev, ...initial }))

        // Only fetch for ids not in cache
        const toFetch = ids.filter(id => !cache[id])
        if (!toFetch.length) return

        let cancelled = false
        ;(async () => {
            try {
                const results = await Promise.allSettled(toFetch.map(async (id) => {
                    const resp = await fetch(`/api/v1/library/anime-entry/dir-exists/${id}`)
                    if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
                    const data = await resp.json() as { exists?: boolean }
                    return { id, exists: !!data?.exists }
                }))
                if (cancelled) return
                const map: Record<number, boolean> = {}
                let changed = false
                results.forEach(r => {
                    if (r.status === "fulfilled") {
                        map[r.value.id] = r.value.exists
                        if (r.value.exists && !cache[r.value.id]) { cache[r.value.id] = true; changed = true }
                    }
                })
                if (changed) saveCache(cache)
                if (Object.keys(map).length) setRecFolderExists(prev => ({ ...prev, ...map }))
            } catch (_) {
                // fail silently; keep map as-is
            }
        })()
        return () => { cancelled = true }
    }, [recommendations, loadCache, saveCache])

    if (!entry || !details) return null

    return (
        <>
            {/*{(!!sourceManga || relations.length > 0 || recommendations.length > 0) && <Separator />}*/}
            {(!!sourceManga || relations.length > 0) && (
                <>
                    <h2>Relations</h2>
                    <MediaCardGrid>
                        {!!sourceManga && <div className="col-span-1">
                            <MediaEntryCard
                                media={sourceManga!}
                                overlay={<p
                                    className="font-semibold text-white bg-gray-950 z-[-1] absolute right-0 w-fit px-4 py-1.5 text-center !bg-opacity-90 text-sm lg:text-base rounded-none rounded-bl-lg border border-t-0 border-r-0"
                                >Manga</p>}
                                type="manga"
                            /></div>}
                        {relations.slice(0, 4).map(edge => {
                            return <div key={edge.node?.id} className="col-span-1">
                                <MediaEntryCard
                                    media={edge.node!}
                                    overlay={<p
                                        className="font-semibold text-white bg-gray-950 z-[-1] absolute right-0 w-fit px-4 py-1.5 text-center !bg-opacity-90 text-sm lg:text-base rounded-none rounded-bl-lg border border-t-0 border-r-0"
                                    >{edge.node?.format === "MOVIE"
                                        ? capitalize(edge.relationType || "").replace("_", " ") + " (Movie)"
                                        : capitalize(edge.relationType || "").replace("_", " ")}</p>}
                                    showLibraryBadge
                                    showTrailer
                                    type="anime"
                                />
                            </div>
                        })}
                    </MediaCardGrid>
                </>
            )}
            {recommendations.length > 0 && <>
                <h2>Recommendations</h2>
                <MediaCardGrid>
                    {recommendations.map(media => {
                        return <div key={media.id} className="col-span-1">
                            <MediaEntryCard
                                media={media!}
                                showLibraryBadge
                                existingFolder={!!recFolderExists[media.id!]}
                                showTrailer
                                type="anime"
                            />
                        </div>
                    })}
                </MediaCardGrid>
            </>}
        </>
    )
}
