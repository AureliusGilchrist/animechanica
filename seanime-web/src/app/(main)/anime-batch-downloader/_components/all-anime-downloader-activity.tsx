"use client"

import React from "react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Separator } from "@/components/ui/separator"
import { AllAnimeDownloadJob } from "../_lib/use-all-anime-downloader"
import { AllAnimeDownloaderProgress } from "./all-anime-downloader-progress"
import { AllAnimeDownloaderStats } from "./all-anime-downloader-stats"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { BiListUl } from "react-icons/bi"

interface ActivityProps {
  job: AllAnimeDownloadJob
  onCancelDownload: () => Promise<void>
}

export function AllAnimeDownloaderActivity({ job, onCancelDownload }: ActivityProps) {
  return (
    <Tabs defaultValue="progress" className="space-y-4">
      <TabsList className="grid w-full grid-cols-3">
        <TabsTrigger value="progress">Progress</TabsTrigger>
        <TabsTrigger value="stats">Stats</TabsTrigger>
        <TabsTrigger value="logs">Logs</TabsTrigger>
      </TabsList>

      <TabsContent value="progress">
        <AllAnimeDownloaderProgress job={job} onCancelDownload={onCancelDownload} />
      </TabsContent>

      <TabsContent value="stats">
        {job?.statistics && <AllAnimeDownloaderStats statistics={job.statistics} />}
      </TabsContent>

      <TabsContent value="logs">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <BiListUl className="w-4 h-4" />
              Activity Log
              <Badge className="ml-2" intent="gray">{job?.logs?.length ?? 0}</Badge>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {(!job?.logs || job.logs.length === 0) ? (
              <div className="text-sm text-muted-foreground">No log entries yet.</div>
            ) : (
              <div className="space-y-6">
                {/* Failure Summary */}
                {(() => {
                  const total = (job?.totalAnime ?? 0) || (job?.logs?.length ?? 0)
                  const failedLogs = (job?.logs ?? []).filter(l => (l?.status === 'failed'))
                  const searchFailedLogs = failedLogs.filter(l => (l?.message || '').toLowerCase().startsWith('search failed'))
                  const failedCount = failedLogs.length
                  const failPct = total > 0 ? ((failedCount / total) * 100) : 0

                  // group by reason after 'search failed:'
                  const groups = new Map<string, typeof searchFailedLogs>()
                  for (const l of searchFailedLogs) {
                    const msg = (l?.message || '')
                    const idx = msg.indexOf(':')
                    const reason = idx >= 0 ? msg.substring(idx + 1).trim() : msg
                    const key = reason || 'unknown'
                    if (!groups.has(key)) groups.set(key, [])
                    groups.get(key)!.push(l)
                  }

                  return (
                    <div className="space-y-4">
                      <div>
                        <div className="text-sm font-semibold">Failures Summary</div>
                        <div className="text-sm text-muted-foreground">
                          Failed: {failedCount.toLocaleString()} / {total.toLocaleString()} ({failPct.toFixed(1)}%)
                        </div>
                      </div>

                      {groups.size > 0 && (
                        <div className="space-y-4">
                          {[...groups.entries()].map(([reason, entries]) => (
                            <div key={reason} className="p-3 border rounded-md">
                              <div className="text-sm font-medium">Search Failed: {reason} — {entries.length}</div>
                              <div className="mt-2 space-y-2">
                                {entries.slice(0, 5).map((e, i) => (
                                  <div key={i} className="text-xs text-muted-foreground">
                                    <div className="font-medium text-foreground">{e.animeTitle}</div>
                                    <div className="break-all">Query: <span className="font-mono">{e.query || '—'}</span></div>
                                    <div className="text-[10px]">{new Date(e.time).toLocaleString()}</div>
                                  </div>
                                ))}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  )
                })()}

                {job.logs.map((entry, idx) => (
                  <div key={idx} className="p-3 border rounded-md">
                    <div className="flex items-center justify-between text-sm">
                      <div className="font-medium">
                        {entry.animeTitle}
                      </div>
                      <Badge intent={entry.status === 'failed' ? 'alert' : (entry.status === 'success' ? 'success' : 'gray')}>
                        {entry.status}
                      </Badge>
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground break-all">
                      Query: <span className="font-mono">{entry.query || '—'}</span>
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground whitespace-pre-line">
                      {entry.message}
                    </div>
                    <div className="mt-1 text-[10px] text-muted-foreground">
                      {new Date(entry.time).toLocaleString()}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  )
}
