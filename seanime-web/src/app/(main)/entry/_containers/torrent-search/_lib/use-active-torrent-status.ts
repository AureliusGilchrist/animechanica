import { TorrentClient_Torrent } from "@/api/generated/types"
import { useGetActiveTorrentList } from "@/api/hooks/torrent_client.hooks"
import { TorrentDownloadStatus } from "@/app/(main)/entry/_containers/torrent-search/_components/torrent-item-badges"
import { useMemo } from "react"

/**
 * Hook to get the download status of a torrent by its info hash.
 * Returns the status and progress if the torrent is currently in the torrent client.
 */
export function useActiveTorrentStatus(enabled: boolean = true) {
    const { data: activeTorrents } = useGetActiveTorrentList(enabled)

    // Create a map of info hash -> torrent for quick lookup
    const torrentMap = useMemo(() => {
        const map = new Map<string, TorrentClient_Torrent>()
        if (activeTorrents) {
            for (const torrent of activeTorrents) {
                if (torrent.hash) {
                    // Store with lowercase hash for case-insensitive matching
                    map.set(torrent.hash.toLowerCase(), torrent)
                }
            }
        }
        return map
    }, [activeTorrents])
    
    /**
     * Get the download status for a torrent by its info hash
     */
    const getStatus = (infoHash: string | undefined): { status: TorrentDownloadStatus; progress?: number } => {
        if (!infoHash) return { status: null }
        
        const torrent = torrentMap.get(infoHash.toLowerCase())
        if (!torrent) return { status: null }

        return {
            status: torrent.status as TorrentDownloadStatus,
            progress: torrent.progress,
        }
    }

    /**
     * Check if a torrent is currently active (downloading, seeding, or paused)
     */
    const isActive = (infoHash: string | undefined): boolean => {
        if (!infoHash) return false
        return torrentMap.has(infoHash.toLowerCase())
    }

    return {
        activeTorrents,
        torrentMap,
        getStatus,
        isActive,
    }
}
