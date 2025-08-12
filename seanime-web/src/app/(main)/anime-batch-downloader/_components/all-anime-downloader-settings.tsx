"use client"

import React, { useState, useRef } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Select } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { 
    BiCog, 
    BiShow, 
    BiVolumeMute, 
    BiDisc, 
    BiDesktop,
    BiGroup,
    BiCalendar,
    BiFilter,
    BiDownload,
    BiUpload,
    BiReset,
    BiBookmark,
    BiX,
    BiPlus,
    BiTrash
} from "react-icons/bi"
import { AllAnimeDownloadSettings, settingsPresets, availableGenres } from "../_lib/use-all-anime-downloader"

interface AllAnimeDownloaderSettingsProps {
    settings: AllAnimeDownloadSettings
    onUpdateSettings: (settings: Partial<AllAnimeDownloadSettings>) => void
    onGeneratePreview: () => Promise<void>
    onApplyPreset: (presetName: keyof typeof settingsPresets) => void
    onResetSettings: () => void
    onExportSettings: () => void
    onImportSettings: (file: File) => void
    isLoading: boolean
}

export function AllAnimeDownloaderSettings({
    settings,
    onUpdateSettings,
    onGeneratePreview,
    onApplyPreset,
    onResetSettings,
    onExportSettings,
    onImportSettings,
    isLoading,
}: AllAnimeDownloaderSettingsProps) {
    const [selectedGenres, setSelectedGenres] = useState<string[]>(settings.includeGenres || [])
    const [excludedGenres, setExcludedGenres] = useState<string[]>(settings.excludeGenres || [])
    const fileInputRef = useRef<HTMLInputElement>(null)

    const handleGenreToggle = (genre: string, type: 'include' | 'exclude') => {
        if (type === 'include') {
            const newGenres = selectedGenres.includes(genre)
                ? selectedGenres.filter(g => g !== genre)
                : [...selectedGenres, genre]
            setSelectedGenres(newGenres)
            onUpdateSettings({ includeGenres: newGenres })
        } else {
            const newGenres = excludedGenres.includes(genre)
                ? excludedGenres.filter(g => g !== genre)
                : [...excludedGenres, genre]
            setExcludedGenres(newGenres)
            onUpdateSettings({ excludeGenres: newGenres })
        }
    }

    const handleFileImport = (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0]
        if (file) {
            onImportSettings(file)
            if (fileInputRef.current) fileInputRef.current.value = ''
        }
    }
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <BiCog className="w-5 h-5" />
                    Download Settings
                </CardTitle>
                <CardDescription>
                    Configure torrent selection preferences and download parameters. Settings are automatically saved.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <Tabs defaultValue="basic" className="space-y-6">
                    <TabsList className="grid w-full grid-cols-4">
                        <TabsTrigger value="basic">Basic</TabsTrigger>
                        <TabsTrigger value="genres">Genres</TabsTrigger>
                        <TabsTrigger value="presets">Presets</TabsTrigger>
                        <TabsTrigger value="manage">Manage</TabsTrigger>
                    </TabsList>

                        <TabsContent value="basic" className="space-y-6">
                            {/* Quality Preferences */}
                            <div className="space-y-4">
                                <h3 className="text-lg font-semibold flex items-center gap-2">
                                    <BiDesktop className="w-4 h-4" />
                                    Quality Preferences
                                </h3>
                                
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-2">
                                            <BiVolumeMute className="w-4 h-4" />
                                            <span>Prefer Dual Audio</span>
                                        </div>
                                        <Switch
                                            value={settings.preferDualAudio}
                                            onValueChange={(value: boolean) => onUpdateSettings({ preferDualAudio: value })}
                                        />
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-2">
                                            <BiDisc className="w-4 h-4" />
                                            <span>Prefer Bluray/BD</span>
                                        </div>
                                        <Switch
                                            value={settings.preferBluray}
                                            onValueChange={(value: boolean) => onUpdateSettings({ preferBluray: value })}
                                        />
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-2">
                                            <BiDesktop className="w-4 h-4" />
                                            <span>Prefer Highest Resolution</span>
                                        </div>
                                        <Switch
                                            value={settings.preferHighestRes}
                                            onValueChange={(value: boolean) => onUpdateSettings({ preferHighestRes: value })}
                                        />
                                    </div>
                                </div>
                            </div>

                            {/* Download Parameters */}
                            <div className="space-y-4">
                                <h3 className="text-lg font-semibold flex items-center gap-2">
                                    <BiGroup className="w-4 h-4" />
                                    Download Parameters
                                </h3>
                                
                                <div className="space-y-4">
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">Minimum Seeders</label>
                                        <p className="text-sm text-muted-foreground">
                                            Skip torrents with fewer seeders than this
                                        </p>
                                        <input
                                            type="range"
                                            min={1}
                                            max={50}
                                            value={settings.minSeeders}
                                            onChange={(e) => onUpdateSettings({ minSeeders: parseInt(e.target.value) })}
                                            className="w-full"
                                        />
                                        <div className="text-sm text-muted-foreground">
                                            Current: {settings.minSeeders} seeders
                                        </div>
                                    </div>
                                    
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">Max Concurrent Downloads</label>
                                        <p className="text-sm text-muted-foreground">
                                            Maximum number of simultaneous downloads
                                        </p>
                                        <input
                                            type="range"
                                            min={1}
                                            max={10}
                                            value={settings.maxConcurrentBatches}
                                            onChange={(e) => onUpdateSettings({ maxConcurrentBatches: parseInt(e.target.value) })}
                                            className="w-full"
                                        />
                                        <div className="text-sm text-muted-foreground">
                                            Current: {settings.maxConcurrentBatches} batches
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Year Range */}
                            <div className="space-y-4">
                                <h3 className="text-lg font-semibold flex items-center gap-2">
                                    <BiCalendar className="w-4 h-4" />
                                    Year Range & Filters
                                </h3>
                                
                                <div className="grid grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">Minimum Year</label>
                                        <input
                                            type="number"
                                            min={1900}
                                            max={2030}
                                            value={settings.minYear}
                                            onChange={(e) => onUpdateSettings({ minYear: parseInt(e.target.value) || 1990 })}
                                            className="w-full px-3 py-2 border rounded-md"
                                        />
                                    </div>
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">Maximum Year</label>
                                        <input
                                            type="number"
                                            min={1900}
                                            max={2030}
                                            value={settings.maxYear}
                                            onChange={(e) => onUpdateSettings({ maxYear: parseInt(e.target.value) || 2024 })}
                                            className="w-full px-3 py-2 border rounded-md"
                                        />
                                    </div>
                                </div>
                                
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="flex items-center justify-between">
                                        <span>Skip OVAs</span>
                                        <Switch
                                            value={settings.skipOva}
                                            onValueChange={(value: boolean) => onUpdateSettings({ skipOva: value })}
                                        />
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <span>Skip Specials</span>
                                        <Switch
                                            value={settings.skipSpecials}
                                            onValueChange={(value: boolean) => onUpdateSettings({ skipSpecials: value })}
                                        />
                                    </div>
                                </div>
                            </div>
                        </TabsContent>

                        <TabsContent value="genres" className="space-y-6">
                            <div className="space-y-4">
                                <h3 className="text-lg font-semibold flex items-center gap-2">
                                    <BiFilter className="w-4 h-4" />
                                    Genre Filtering
                                </h3>
                                <p className="text-sm text-muted-foreground">
                                    Select genres to include or exclude. If no genres are selected for inclusion, all genres will be considered.
                                </p>

                                {/* Include Genres */}
                                <div className="space-y-3">
                                    <div className="flex items-center gap-2">
                                        <BiPlus className="w-4 h-4 text-green-600" />
                                        <h4 className="font-medium text-green-600">Include Only These Genres</h4>
                                        <Badge intent="gray">{selectedGenres.length}</Badge>
                                    </div>
                                    <div className="flex flex-wrap gap-2 max-h-32 overflow-y-auto">
                                        {availableGenres.map(genre => (
                                            <Badge
                                                key={genre}
                                                intent={selectedGenres.includes(genre) ? "primary" : "gray"}
                                                className={`cursor-pointer transition-colors ${
                                                    selectedGenres.includes(genre) 
                                                        ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200" 
                                                        : "hover:bg-green-50 dark:hover:bg-green-950"
                                                }`}
                                                onClick={() => handleGenreToggle(genre, 'include')}
                                            >
                                                {genre}
                                                {selectedGenres.includes(genre) && <BiX className="w-3 h-3 ml-1" />}
                                            </Badge>
                                        ))}
                                    </div>
                                </div>

                                <Separator />

                                {/* Exclude Genres */}
                                <div className="space-y-3">
                                    <div className="flex items-center gap-2">
                                        <BiTrash className="w-4 h-4 text-red-600" />
                                        <h4 className="font-medium text-red-600">Exclude These Genres</h4>
                                        <Badge intent="gray">{excludedGenres.length}</Badge>
                                    </div>
                                    <div className="flex flex-wrap gap-2 max-h-32 overflow-y-auto">
                                        {availableGenres.map(genre => (
                                            <Badge
                                                key={genre}
                                                intent={excludedGenres.includes(genre) ? "alert" : "gray"}
                                                className={`cursor-pointer transition-colors ${
                                                    excludedGenres.includes(genre) 
                                                        ? "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200" 
                                                        : "hover:bg-red-50 dark:hover:bg-red-950"
                                                }`}
                                                onClick={() => handleGenreToggle(genre, 'exclude')}
                                            >
                                                {genre}
                                                {excludedGenres.includes(genre) && <BiX className="w-3 h-3 ml-1" />}
                                            </Badge>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </TabsContent>

                        <TabsContent value="presets" className="space-y-6">
                            <div className="space-y-4">
                                <h3 className="text-lg font-semibold flex items-center gap-2">
                                    <BiBookmark className="w-4 h-4" />
                                    Settings Presets
                                </h3>
                                <p className="text-sm text-muted-foreground">
                                    Quick configuration presets for common use cases
                                </p>

                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <Card className="cursor-pointer hover:bg-accent transition-colors" onClick={() => onApplyPreset('conservative')}>
                                        <CardContent className="p-4">
                                            <h4 className="font-medium text-blue-600">Conservative</h4>
                                            <p className="text-sm text-muted-foreground mt-1">
                                                High quality, fewer downloads. 10+ seeders, dual audio, Bluray preferred.
                                            </p>
                                            <div className="flex gap-1 mt-2">
                                                <Badge intent="gray" className="text-xs">High Quality</Badge>
                                                <Badge intent="gray" className="text-xs">2000+</Badge>
                                            </div>
                                        </CardContent>
                                    </Card>

                                    <Card className="cursor-pointer hover:bg-accent transition-colors" onClick={() => onApplyPreset('balanced')}>
                                        <CardContent className="p-4">
                                            <h4 className="font-medium text-green-600">Balanced</h4>
                                            <p className="text-sm text-muted-foreground mt-1">
                                                Good balance of quality and quantity. 5+ seeders, dual audio preferred.
                                            </p>
                                            <div className="flex gap-1 mt-2">
                                                <Badge intent="gray" className="text-xs">Balanced</Badge>
                                                <Badge intent="gray" className="text-xs">1990+</Badge>
                                            </div>
                                        </CardContent>
                                    </Card>

                                    <Card className="cursor-pointer hover:bg-accent transition-colors" onClick={() => onApplyPreset('aggressive')}>
                                        <CardContent className="p-4">
                                            <h4 className="font-medium text-orange-600">Aggressive</h4>
                                            <p className="text-sm text-muted-foreground mt-1">
                                                Maximum downloads. 1+ seeders, any quality, includes everything.
                                            </p>
                                            <div className="flex gap-1 mt-2">
                                                <Badge intent="gray" className="text-xs">Max Volume</Badge>
                                                <Badge intent="gray" className="text-xs">1960+</Badge>
                                            </div>
                                        </CardContent>
                                    </Card>

                                    <Card className="cursor-pointer hover:bg-accent transition-colors" onClick={() => onApplyPreset('qualityFocused')}>
                                        <CardContent className="p-4">
                                            <h4 className="font-medium text-purple-600">Quality Focused</h4>
                                            <p className="text-sm text-muted-foreground mt-1">
                                                Premium quality only. 15+ seeders, Bluray, highest resolution.
                                            </p>
                                            <div className="flex gap-1 mt-2">
                                                <Badge intent="gray" className="text-xs">Premium</Badge>
                                                <Badge intent="gray" className="text-xs">2010+</Badge>
                                            </div>
                                        </CardContent>
                                    </Card>
                                </div>
                            </div>
                        </TabsContent>

                        <TabsContent value="manage" className="space-y-6">
                            <div className="space-y-4">
                                <h3 className="text-lg font-semibold flex items-center gap-2">
                                    <BiCog className="w-4 h-4" />
                                    Settings Management
                                </h3>
                                
                                <div className="flex flex-wrap gap-3">
                                    <Button
                                        intent="primary-outline"
                                        onClick={onExportSettings}
                                        className="flex items-center gap-2"
                                    >
                                        <BiDownload className="w-4 h-4" />
                                        Export Settings
                                    </Button>
                                    
                                    <Button
                                        intent="primary-outline"
                                        onClick={() => fileInputRef.current?.click()}
                                        className="flex items-center gap-2"
                                    >
                                        <BiUpload className="w-4 h-4" />
                                        Import Settings
                                    </Button>
                                    
                                    <Button
                                        intent="primary-outline"
                                        onClick={onResetSettings}
                                        className="flex items-center gap-2 text-red-600 hover:text-red-700"
                                    >
                                        <BiReset className="w-4 h-4" />
                                        Reset to Defaults
                                    </Button>
                                </div>
                                
                                <input
                                    ref={fileInputRef}
                                    type="file"
                                    accept=".json"
                                    onChange={handleFileImport}
                                    className="hidden"
                                />
                            </div>

                            {/* Priority Summary */}
                            <div className="p-4 bg-gradient-to-r from-blue-50 to-purple-50 dark:from-blue-950 dark:to-purple-950 rounded-lg border">
                                <h4 className="font-medium mb-3">Torrent Selection Priority</h4>
                                <div className="space-y-2 text-sm">
                                    <div className="flex items-center gap-2">
                                        <Badge className="bg-purple-100 dark:bg-purple-900">
                                            <BiVolumeMute className="w-3 h-3 mr-1" />
                                            1st
                                        </Badge>
                                        <span>Popularity (Download Count)</span>
                                    </div>
                                    <div className="flex items-center gap-2">
                                        <Badge className="bg-indigo-100 dark:bg-indigo-900">
                                            <BiDisc className="w-3 h-3 mr-1" />
                                            2nd
                                        </Badge>
                                        <span>Dual Audio (if enabled)</span>
                                    </div>
                                    <div className="flex items-center gap-2">
                                        <Badge className="bg-cyan-100 dark:bg-cyan-900">
                                            <BiDesktop className="w-3 h-3 mr-1" />
                                            3rd
                                        </Badge>
                                        <span>Bluray/BD Quality (if enabled)</span>
                                    </div>
                                    <div className="flex items-center gap-2">
                                        <Badge>4th</Badge>
                                        <span>Most Seeders ({settings.minSeeders}+ minimum)</span>
                                    </div>
                                    <div className="flex items-center gap-2">
                                        <Badge>5th</Badge>
                                        <span>Largest File Size</span>
                                    </div>
                                </div>
                            </div>

                            {/* Generate Preview Button */}
                            <div className="pt-4 border-t">
                                <Button
                                    onClick={onGeneratePreview}
                                    disabled={isLoading}
                                    className="w-full"
                                >
                                    <BiShow className="w-4 h-4 mr-2" />
                                    Generate Preview
                                </Button>
                            </div>
                        </TabsContent>
                    </Tabs>
                </CardContent>
            </Card>
        )
}
