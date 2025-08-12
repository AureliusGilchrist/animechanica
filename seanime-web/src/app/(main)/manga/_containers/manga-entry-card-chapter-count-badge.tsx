import { __mangaLibrary_latestChapterNumbersAtom as __mangaLibrary_currentMangaDataAtom } from "@/app/(main)/manga/_lib/handle-manga-collection"
import { Badge } from "@/components/ui/badge"
import { useAtom } from "jotai"
import React from "react"
import { getMangaEntryLatestChapterNumber } from "../_lib/handle-manga-selected-provider"

export type MangaEntryCardChapterCountBadgeProps = {
  mediaId: number
  progressTotal?: number
}

export function MangaEntryCardChapterCountBadge(props: MangaEntryCardChapterCountBadgeProps) {
  const { mediaId, progressTotal: _progressTotal } = props

  const [mangaData] = useAtom(__mangaLibrary_currentMangaDataAtom)
  const [progressTotal, setProgressTotal] = React.useState<number>(_progressTotal || 0)

  React.useEffect(() => {
    const latestChapterNumber = getMangaEntryLatestChapterNumber(
      mediaId,
      mangaData.latestChapterNumbers,
      mangaData.storedProviders,
      mangaData.storedFilters,
    )
    if (latestChapterNumber) {
      setProgressTotal(latestChapterNumber)
    }
  }, [mediaId, mangaData])

  if (!progressTotal || progressTotal <= 0) return null

  return (
    <div className="flex w-full z-[11]" data-manga-entry-card-chapter-count-badge-container>
      <Badge
        intent="gray-solid"
        size="lg"
        className="rounded-[--radius-md] rounded-tl-none rounded-br-none text-white bg-gray-950 !bg-opacity-90 shadow-md px-2 py-0.5 font-semibold tracking-wide"
        data-manga-entry-card-chapter-count-badge
      >
        {progressTotal} ch
      </Badge>
    </div>
  )
}
