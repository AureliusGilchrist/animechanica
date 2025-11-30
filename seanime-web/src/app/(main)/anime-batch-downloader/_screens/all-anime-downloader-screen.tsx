"use client"

import React from "react"
import { AllAnimeDownloaderHeader } from "../_components/all-anime-downloader-header"
import { AllAnimeDownloaderDashboard } from "../_components/all-anime-downloader-dashboard"
import { AllAnimeDownloaderSettings } from "../_components/all-anime-downloader-settings"
import { AllAnimeDownloaderActivity } from "../_components/all-anime-downloader-activity"
import { AllAnimeDownloaderPreview } from "../_components/all-anime-downloader-preview"
import { useAllAnimeDownloader } from "../_lib/use-all-anime-downloader"
import { Separator } from "@/components/ui/separator"
import { Alert } from "@/components/ui/alert"
import { BiInfoCircle } from "react-icons/bi"

export function AllAnimeDownloaderScreen() {
    const {
        databaseStatus,
        activeJob,
        settings,
        preview,
        isLoading,
        databaseStats,
        loadDatabase,
        startDownload,
        cancelDownload,
        updateSettings,
        generatePreview,
        applyPreset,
        resetSettings,
        exportSettings,
        importSettings,
    } = useAllAnimeDownloader()

    return (
        <div className="space-y-6">
            {/* Header */}
            <AllAnimeDownloaderHeader 
                databaseStatus={databaseStatus}
                onLoadDatabase={loadDatabase}
                isLoading={isLoading}
            />

            {/* Warning Alert */}
            <Alert 
                intent="warning"
                icon={<BiInfoCircle className="h-4 w-4" />}
                title="Warning"
                description={
                    <span>
                        This will download ALL anime in the database (~20,000+ titles, ~160TB+). 
                        This process will take months to complete and requires significant storage space and bandwidth.
                        Make sure your qBittorrent client is properly configured and has enough disk space.
                    </span>
                }
            />

            {databaseStatus?.loaded && (
                <>
                    {/* Dashboard */}
                    <AllAnimeDownloaderDashboard 
                        databaseStats={databaseStats}
                        activeJob={activeJob}
                        onStartDownload={startDownload}
                        onCancelDownload={cancelDownload}
                        isLoading={isLoading}
                    />

                    <Separator />

                    {/* Settings */}
                    <AllAnimeDownloaderSettings 
                        settings={settings}
                        onUpdateSettings={updateSettings}
                        onGeneratePreview={generatePreview}
                        onApplyPreset={applyPreset}
                        onResetSettings={resetSettings}
                        onExportSettings={exportSettings}
                        onImportSettings={importSettings}
                        isLoading={isLoading}
                    />

                    {/* Preview */}
                    {preview && (
                        <>
                            <Separator />
                            <AllAnimeDownloaderPreview 
                                preview={preview}
                                onStartDownload={startDownload}
                                isLoading={isLoading}
                            />
                        </>
                    )}

                    {/* Activity (Progress / Stats / Logs) */}
                    {activeJob && (
                        <>
                            <Separator />
                            <AllAnimeDownloaderActivity
                                job={activeJob}
                                onCancelDownload={cancelDownload}
                            />
                        </>
                    )}
                </>
            )}
        </div>
    )
}
