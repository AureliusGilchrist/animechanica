'use client'

import { useGetCharacterDetails } from '@/api/hooks/anilist.hooks'
import { PageWrapper } from '@/components/shared/page-wrapper'
import { LoadingSpinner } from '@/components/ui/loading-spinner'
import { useSearchParams } from 'next/navigation'
import React from 'react'
import { CharacterDetailsView } from '../_components/character-details-view'

export function CharacterEntryPage() {
    const searchParams = useSearchParams()
    const characterId = searchParams.get('id')

    const { data: character, isLoading, error } = useGetCharacterDetails(characterId ? parseInt(characterId) : undefined)

    if (!characterId) {
        return (
            <PageWrapper className="p-4 sm:p-8 space-y-8">
                <div className="text-center">
                    <h1 className="text-2xl font-bold text-red-500">Invalid Character ID</h1>
                    <p className="text-muted-foreground mt-2">Please provide a valid character ID in the URL.</p>
                </div>
            </PageWrapper>
        )
    }

    if (isLoading) {
        return (
            <PageWrapper className="p-4 sm:p-8 space-y-8">
                <div className="flex justify-center items-center min-h-[50vh]">
                    <LoadingSpinner />
                </div>
            </PageWrapper>
        )
    }

    if (error || !character) {
        return (
            <PageWrapper className="p-4 sm:p-8 space-y-8">
                <div className="text-center">
                    <h1 className="text-2xl font-bold text-red-500">Character Not Found</h1>
                    <p className="text-muted-foreground mt-2">
                        The character with ID {characterId} could not be found or an error occurred while fetching the data.
                    </p>
                </div>
            </PageWrapper>
        )
    }

    return (
        <PageWrapper className="p-4 sm:p-8 space-y-8">
            <CharacterDetailsView character={character} />
        </PageWrapper>
    )
}
