import { AL_AnimeDetailsById_Media, AL_MangaDetailsById_Media, AL_Staff, AL_BaseCharacter } from "@/api/generated/types"
import { imageShimmer } from "@/components/shared/image-helpers"
import { SeaLink } from "@/components/shared/sea-link"
import { cn } from "@/components/ui/core/styling"
import { useThemeSettings } from "@/lib/theme/hooks"
import Image from "next/image"
import React from "react"
import { BiSolidHeart } from "react-icons/bi"

type MediaEntryCastSectionProps = {
    details: AL_AnimeDetailsById_Media | AL_MangaDetailsById_Media | undefined
    isMangaPage?: boolean
}

export function MediaEntryCastSection(props: MediaEntryCastSectionProps) {

    const {
        details,
        isMangaPage,
        ...rest
    } = props

    const ts = useThemeSettings()

    // Get all voice actors from characters (only available for anime)
    const voiceActors = React.useMemo(() => {
        const actors: Array<{
            staff: AL_Staff,
            character: AL_BaseCharacter | undefined,
            role?: string
        }> = []
        
        // Only process voice actors for anime (not manga)
        if (!isMangaPage) {
            details?.characters?.edges?.forEach(edge => {
                // Type guard to ensure we're working with anime character edges
                const animeEdge = edge as any
                if (animeEdge?.voiceActors && animeEdge.voiceActors.length > 0) {
                    animeEdge.voiceActors.forEach((actor: AL_Staff) => {
                        // Check if this actor is already in our list
                        const existingActor = actors.find(a => a.staff.id === actor.id)
                        if (!existingActor) {
                            actors.push({
                                staff: actor,
                                character: edge.node,
                                role: edge.role
                            })
                        }
                    })
                }
            })
        }
        
        return actors
    }, [details?.characters?.edges, isMangaPage])

    if (voiceActors.length === 0) return null

    return (
        <>
            <h2 data-media-entry-cast-section-title>Cast</h2>

            <div
                data-media-entry-cast-section-grid
                className={cn(
                    "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-4",
                    isMangaPage && "grid-cols-1 md:grid-col-2 lg:grid-cols-3 xl:grid-cols-2 2xl:grid-cols-2",
                )}
            >
                {voiceActors?.slice(0, 15).map((actorData, index) => {
                    const { staff, character } = actorData
                    return <div key={`${staff?.id}-${index}`} className="col-span-1" data-media-entry-cast-section-grid-item>
                        <div
                            data-media-entry-cast-section-grid-item-container
                            className={cn(
                                "max-w-full flex gap-4",
                                "rounded-lg relative transition group/episode-list-item select-none",
                                !!ts.libraryScreenCustomBackgroundImage && ts.libraryScreenCustomBackgroundOpacity > 5
                                    ? "bg-[--background] p-3"
                                    : "py-3",
                                "pr-12",
                            )}
                            {...rest}
                        >

                            <div
                                data-media-entry-cast-section-grid-item-image-container
                                className={cn(
                                    "size-20 flex-none rounded-[--radius-md] object-cover object-center relative overflow-hidden",
                                    "group/ep-item-img-container",
                                )}
                            >
                                <div
                                    data-media-entry-cast-section-grid-item-image-overlay
                                    className="absolute z-[1] rounded-[--radius-md] w-full h-full"
                                ></div>
                                <div
                                    data-media-entry-cast-section-grid-item-image-background
                                    className="bg-[--background] absolute z-[0] rounded-[--radius-md] w-full h-full"
                                ></div>
                                {(staff?.image?.large) && <Image
                                    data-media-entry-cast-section-grid-item-image
                                    src={staff?.image?.large || ""}
                                    alt="voice actor image"
                                    fill
                                    quality={60}
                                    placeholder={imageShimmer(700, 475)}
                                    sizes="10rem"
                                    className={cn("object-cover object-center transition select-none")}
                                    data-src={staff?.image?.large}
                                />}
                            </div>

                            <div data-media-entry-cast-section-grid-item-content>
                                <SeaLink href={`/staff/entry?id=${staff?.id}`} data-media-entry-cast-section-grid-item-content-link>
                                    <p
                                        className={cn(
                                            "text-lg font-semibold transition line-clamp-2 leading-5 hover:text-brand-100",
                                        )}
                                    >
                                        {staff?.name?.full}
                                    </p>
                                </SeaLink>

                                {character && <p data-media-entry-cast-section-grid-item-content-character className="text-sm text-[--muted]">
                                    as {character?.name?.full}
                                </p>}

                                <p data-media-entry-cast-section-grid-item-content-role className="text-[--muted] text-xs">
                                    Voice Actor
                                </p>

                                {staff?.isFavourite && <div data-media-entry-cast-section-grid-item-content-favourite>
                                    <BiSolidHeart className="text-pink-600 text-lg block" />
                                </div>}
                            </div>
                        </div>
                    </div>
                })}
            </div>
        </>
    )
}
