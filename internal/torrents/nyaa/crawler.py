#!/usr/bin/env python3

# Standard library imports
import argparse
import json
import logging
import os
import sys
import time
import re
from typing import List, Dict, Optional
from urllib.parse import urljoin, quote_plus

# Third-party imports
import requests
from bs4 import BeautifulSoup
from rich.logging import RichHandler
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn
from rich.console import Console
from rich.text import Text

# Configure rich console for better output
console = Console()

# Configure logging with rich handler for better appearance
logging.basicConfig(
    level=logging.DEBUG,
    format="%(message)s",
    handlers=[
        RichHandler(
            rich_tracebacks=True,
            console=console,
            show_time=True,
            show_path=False
        ),
        logging.FileHandler('nyaa_crawler.log')
    ]
)
logger = logging.getLogger(__name__)

#######################
# Main Crawler Class #
#######################

Search = ""
class NyaaCrawler:
    """
    A crawler for Nyaa.si that downloads torrents and adds them to qBittorrent.
    
    This class handles:
    - Configuration management
    - Authentication with qBittorrent
    - Crawling Nyaa.si pages
    - Parsing torrent information
    - Adding torrents to qBittorrent
    """

    def __init__(self, config_path='config.json'):
        """
        Initialize the crawler with configuration.
        """
        self.session = requests.Session()
        self.config_path = config_path  # Define config_path

        # Load configuration
        if os.path.exists(self.config_path):
            with open(self.config_path, 'r') as f:
                self.config = json.load(f)
            # Backfill default anime base path if missing in existing configs
            if 'anime_download_base' not in self.config:
                self.config['anime_download_base'] = "/aeternae/theater/anime/completed"
        else:
            self.config = {
                "qbittorrent": {
                    "url": "http://localhost:8081/api/v2",
                    "username": "admin",
                    "password": "lolmao"
                }, #fix filesystem
                "nyaa": {
                    "base_url": "https://nyaa.si",
                    "search_query": "[Judas] batch",
                    "category": "0_0",  # All categories
                    "filter": "0",      # No filter
                    "start_page": 1,
                    "end_page": 10000,
                    "delay": 0.5          # Seconds between requests
                },
                # Preferred anime base path where each torrent will be placed under {TITLE}
                "anime_download_base": "/aeternae/theater/anime/completed"
            }


        # Authentication logic
        try:
            auth_response = self.session.post(
                f"{self.config['qbittorrent']['url']}/auth/login",
                data={
                    "username": self.config['qbittorrent']['username'],
                    "password": self.config['qbittorrent']['password']
                }
            )
            if auth_response.status_code != 200:
                raise Exception(f"Authentication failed with status {auth_response.status_code}")
            
            logger.info("[green]Successfully authenticated with qBittorrent[/green]")
        except Exception as e:
            logger.error(f"[red]Authentication failed:[/red] {str(e)}")

    def get_search_url(self, page: int) -> str:
        """
        Generate search URL for given page.

        Args:
            page (int): Page number

        Returns:
            str: Search URL
        """
        params = {
            "f": self.config['nyaa']['filter'],
            "c": self.config['nyaa']['category'],
            "q": self.config['nyaa']['search_query'],
            "p": str(page)
        }
        query = "&".join(f"{k}={quote_plus(str(v))}" for k, v in params.items())
        return f"{self.config['nyaa']['base_url']}/?{query}"

    def parse_torrent_data(self, row) -> Optional[Dict]:
        """
        Parse a single torrent row.

        Args:
            row: Torrent row

        Returns:
            Optional[Dict]: Parsed torrent data or None if parsing fails
        """
        try:
            cols = row.find_all('td')
            if len(cols) < 5:
                return None

            # Look for the main torrent title link in the second column
            title_links = cols[1].find_all('a')
            title_col = None
            
            # Find the main title link (not comments, not category)
            for link in title_links:
                href = link.get('href', '')
                # Skip comment links and category links
                if 'comments' not in href.lower() and 'category' not in href.lower() and '/view/' in href:
                    title_col = link
                    break
            
            if not title_col:
                # Fallback: try to get any link with a title attribute
                title_col = cols[1].find('a', {'title': True})
                if not title_col:
                    return None

            # Find the magnet link
            magnet = row.find('a', href=lambda href: href and href.startswith('magnet:?'))
            if not magnet:
                return None

            # Get the actual title text, prefer title attribute, fallback to link text
            title_text = title_col.get('title') or title_col.get_text(strip=True)
            
            seeders = int(cols[5].text.strip())
            if seeders <= 0:
                logger.debug(f"Skipping torrent with 0 seeders: {title_text}")
                return None
            
            return {
                'title': title_text,
                'magnet': magnet['href'],
                'size': cols[3].text.strip(),
                'date': cols[4].text.strip(),
                'seeders': seeders,
                'leechers': int(cols[6].text.strip())
            }
        except Exception as e:
            logger.error(f"[red]Error parsing torrent row:[/red] {str(e)}")
            return None

    def process_page(self, page: int) -> List[Dict]:
        """
        Process a single page of results.

        Args:
            page (int): Page number

        Returns:
            List[Dict]: List of parsed torrent data
        """
        url = self.get_search_url(page)
        logger.info(f"[blue]Processing page {page}:[/blue] {url}")
        
        try:
            response = self.session.get(url)
            response.raise_for_status()
            
            soup = BeautifulSoup(response.content, 'html.parser')
            # Verify the correct class names for rows
            rows = soup.find_all('tr', class_=['success', 'default', 'danger'])
            
            if not rows:
                logger.warning(f"[yellow]No rows found on page {page}. Check if the HTML structure has changed.[/yellow]")
                # Debugging: Log the HTML content
                logger.debug(soup.prettify())
            
            results = []
            for row in rows:
                logger.debug(f"Processing row: {row.prettify()}")
                data = self.parse_torrent_data(row)
                if data:
                    results.append(data)
            
            logger.info(f"[green]Found {len(results)} torrents on page {page}[/green]")
            return results
            
        except Exception as e:
            logger.error(f"[red]Error processing page {page}:[/red] {str(e)}")
            return []

    def _sanitize_title(self, title: str) -> str:
        """
        Make a title safe for filesystem paths.
        """
        unsafe = ['\\', '/', ':', '*', '?', '"', '<', '>', '|']
        safe = title.strip()
        for ch in unsafe:
            safe = safe.replace(ch, ' ')
        # Collapse multiple spaces
        safe = ' '.join(safe.split())
        return safe



    def _clean_torrent_title(self, title: str) -> str:
        """
        Clean a torrent title for use as a display name (preserves proper case).
        Similar to _normalize_for_match but keeps original capitalization.
        """
        try:
            if not title:
                return ''
            
            s = title
            
            # Remove all bracketed segments: [Judas], [UNCENSORED BD 1080p], etc.
            s = re.sub(r"\[[^\]]*\]", " ", s)
            
            # Remove parenthetical segments but be more selective
            # 1) Remove quality/format parentheticals like (BD 1080p), (WEB-DL), etc.
            s = re.sub(r"\([^)]*(?:BD|DVD|WEB|RIP|DL)[^)]*\)", " ", s, flags=re.I)
            # 2) Remove duplicate English title in parentheses, e.g., "Title (English Title)"
            #    Preserve any parentheses that contain season information or digits.
            s = re.sub(r"\((?![^)]*(?:season|s\s*\d))[^)\d]*\)", " ", s, flags=re.I)
            
            # Remove curly braces segments
            s = re.sub(r"\{[^}]*\}", " ", s)
            
            # Remove common quality/format tokens
            quality_tokens = [
                r"\b(bluray|bd|uncensored|censored|hevc|x264|x265|10bit|8bit)\b",
                r"\b(dual[- ]?audio|eng[- ]?subs?|multi[- ]?subs?)\b", 
                # Keep season indicators; only remove 'batch'
                r"\b(batch)\b",
                r"\b(\d{3,4}p|720p|1080p|2160p|4k)\b",
                r"\b(web[- ]?dl|web[- ]?rip|bdrip|dvdrip)\b"
            ]
            for pattern in quality_tokens:
                s = re.sub(pattern, " ", s, flags=re.I)

            # Remove version suffix tokens like v1/v2/v3 that are often used for revisions
            s = re.sub(r"\bv\d+\b", " ", s, flags=re.I)
            
            # Clean up punctuation but preserve apostrophes and basic punctuation
            s = re.sub(r"[`''""]+", "'", s)  # Normalize quotes/apostrophes
            s = re.sub(r"[^\w\s'.,!?:-]+", " ", s)  # Keep basic punctuation
            
            # Collapse multiple spaces and strip
            s = " ".join(s.split())
            
            return s
        except Exception:
            return title if title else ""


    def add_to_qbittorrent(self, title: str, magnet: str) -> bool:
        """
        Add magnet link to qBittorrent.

        Args:
            magnet (str): Magnet link

        Returns:
            bool: True if addition is successful, False otherwise
        """
        try:
            base_path = self.config.get('anime_download_base')
            safe_title = self._sanitize_title(title)
            # Create the specific directory for this anime
            target_path = os.path.join(base_path, safe_title)
            os.makedirs(target_path, exist_ok=True)
            # Tag to identify the newly added torrent so we can rename post-add
            unique_tag = f"animechanica:{int(time.time()*1000)}"
            response = self.session.post(
                f"{self.config['qbittorrent']['url']}/torrents/add",
                data={
                    "urls": magnet,
                    # Save to a specific subfolder with the cleaned name
                    "savepath": os.path.join(base_path, safe_title),
                    # Do not let qBittorrent move it elsewhere
                    "autoTMM": "false",
                    # Don't create additional root folder since we're specifying the path
                    "root_folder": "false",
                    "rename": safe_title,
                    # Use original layout to avoid extra nesting
                    "contentLayout": "Original",
                    # Attach a tag to find this torrent after add
                    "tags": unique_tag,
                }
            )
            if response.status_code != 200:
                logger.error(f"Failed to add torrent: HTTP {response.status_code} -> {response.text}")
                return False
            # Post-add: ensure the torrent/root folder name matches the DB Title
            self._finalize_torrent_name(unique_tag, safe_title)
            return True
        except Exception as e:
            logger.error(f"Error adding torrent to qBittorrent: {e}")
            logger.error(f"[red]Error adding torrent:[/red] {str(e)}")
            return False

    def _finalize_torrent_name(self, tag: str, desired_name: str) -> None:
        """
        Find the just-added torrent by temporary tag and rename it to desired_name.
        Cleans up the tag afterwards. Best-effort; logs errors but does not raise.
        """
        try:
            base_url = self.config['qbittorrent']['url']
            # Poll for up to ~30s
            for _ in range(60):
                r = self.session.get(f"{base_url}/torrents/info", params={"tags": tag})
                if r.status_code == 200:
                    items = r.json() if r.text else []
                    if isinstance(items, list) and items:
                        t = items[0]
                        torrent_hash = t.get('hash') or t.get('infohash_v1') or t.get('infohash')
                        if torrent_hash:
                            # 1) Rename torrent (updates displayed name and usually the root folder)
                            rn = self.session.post(f"{base_url}/torrents/rename", data={
                                "hash": torrent_hash,
                                "name": desired_name,
                            })
                            if rn.status_code != 200:
                                logger.debug(f"Rename API failed: HTTP {rn.status_code} -> {rn.text}")

                            # 2) Ensure content root folder on filesystem matches desired_name
                            try:
                                files_resp = self.session.get(f"{base_url}/torrents/files", params={"hash": torrent_hash})
                                if files_resp.status_code == 200:
                                    files = files_resp.json() if files_resp.text else []
                                    roots = []
                                    for f in files:
                                        rel = f.get('name') or ''
                                        if '/' in rel:
                                            roots.append(rel.split('/', 1)[0])
                                        else:
                                            # Single file torrents - use the filename as root
                                            roots.append(rel)
                                    
                                    if roots:
                                        # Get the most common root folder
                                        old_root = max(set(roots), key=roots.count)
                                        logger.debug(f"Current root folder: '{old_root}', desired: '{desired_name}'")
                                        
                                        if old_root and old_root != desired_name:
                                            logger.debug(f"Renaming folder from '{old_root}' to '{desired_name}'")
                                            rf = self.session.post(f"{base_url}/torrents/renameFolder", data={
                                                "hash": torrent_hash,
                                                "oldPath": old_root,
                                                "newPath": desired_name,
                                            })
                                            if rf.status_code == 200:
                                                logger.debug(f"Successfully renamed folder to '{desired_name}'")
                                            else:
                                                logger.debug(f"renameFolder failed: HTTP {rf.status_code} -> {rf.text}")
                                        else:
                                            logger.debug(f"Folder already has correct name: '{desired_name}'")
                                    else:
                                        logger.debug("No files found in torrent for folder rename")
                            except Exception as fe:
                                logger.debug(f"Folder rename check failed: {fe}")

                            # 3) Remove temporary tag
                            self.session.post(f"{base_url}/torrents/removeTags", data={
                                "hashes": torrent_hash,
                                "tags": tag,
                            })
                            return
                time.sleep(0.5)
            logger.debug(f"Timed out waiting for torrent with tag '{tag}' to appear for rename")
        except Exception as e:
            logger.debug(f"Finalize rename failed: {e}")

    def crawl(self) -> None:
        """
        Main crawling function that processes pages and adds torrents to qBittorrent.
        
        This function:
        - Iterates through pages based on configuration
        - Processes each page to extract torrent information
        - Adds valid torrents to qBittorrent with cleaned titles
        - Provides real-time progress feedback
        - Stops early if multiple empty pages are encountered
        """
        start_page = self.config['nyaa']['start_page']
        end_page = self.config['nyaa']['end_page']
        delay = self.config['nyaa']['delay']
        
        total_torrents = 0
        added_torrents = 0
        empty_pages_threshold = 5  # Stop after this many consecutive empty pages
        empty_pages_count = 0
        
        magnet_links = []  # List to store magnet links

        try:
            with Progress(
                SpinnerColumn(),
                TextColumn("[progress.description]{task.description}"),
                BarColumn(),
                TaskProgressColumn(),
                console=console
            ) as progress:
                page_task = progress.add_task(
                    "[cyan]Processing pages...",
                    total=end_page - start_page + 1
                )
                
                for page in range(start_page, end_page + 1):
                    torrents = self.process_page(page)
                    
                    if not torrents:
                        empty_pages_count += 1
                        if empty_pages_count >= empty_pages_threshold:
                            logger.warning(f"[yellow]Stopping early: Found {empty_pages_threshold} empty pages in a row[/yellow]")
                            break
                    else:
                        empty_pages_count = 0  # Reset counter when we find torrents
                        
                    total_torrents += len(torrents)
                    
                    for torrent in torrents:
                        magnet_links.append(torrent['magnet'])  # Save magnet link
                        # Use cleaned torrent title
                        torrent_title = torrent.get('title', '').strip()
                        if torrent_title and torrent_title not in ['Comments 1', 'Comments', '']:
                            # Use cleaned version of torrent title
                            use_title = self._clean_torrent_title(torrent_title)
                            if not use_title or not use_title.strip():
                                use_title = torrent_title
                                logger.debug("Using raw torrent title (cleaning failed)")
                            else:
                                logger.debug(f"Using cleaned torrent title: {use_title}")
                        else:
                            # Skip this torrent if we can't get a valid title
                            logger.warning(f"Skipping torrent with invalid title: '{torrent.get('title', 'N/A')}'")
                            continue
                        if self.add_to_qbittorrent(use_title, torrent['magnet']):
                            added_torrents += 1
                            log_torrent_added(use_title, torrent['size'], torrent['seeders'])
                        
                        # Be nice to the servers
                        time.sleep(delay)
                    
                    progress.update(page_task, advance=1)
                    logger.info(f"[blue]Progress:[/blue] {page}/{end_page} pages, {added_torrents}/{total_torrents} torrents added")
                
        except KeyboardInterrupt:
            logger.warning("\n[yellow]Crawling interrupted by user[/yellow]")
        except Exception as e:
            logger.error(f"[red]Crawling error:[/red] {str(e)}")
        finally:
            # Save all magnet links to a file
            with open('magnet_links.txt', 'w') as f:
                for link in magnet_links:
                    f.write(link + '\n')
            logger.info(f"[green]Crawling finished.[/green] Added {added_torrents} out of {total_torrents} torrents")

def log_torrent_added(title, size, seeders):
    # Create a formatted text object
    text = Text()
    text.append("Added: ", style="bold green")
    text.append(title, style="bold yellow")
    text.append(f" ({size}, ", style="yellow")
    text.append(f"{seeders} seeders", style="cyan")
    text.append(")", style="yellow")
    
    # Print the formatted text
    console.print(text)

def main():
    Search = [  "[Judas] batch", "[DB] batch", "[EMBER] batch", "[smol] monogatari", "[Erai-raws] batch", "[SubsPlease] batch", "[HorribleSubs] batch", "[Trix] batch",
                "[Judas] complete", "[DB] complete", "[EMBER] complete", "[Erai-raws] complete", "[SubsPlease] complete", "[HorribleSubs] complete", "[Trix] batcompletech"]
    
    for query in Search:
        logger.info(f"[cyan]Starting crawl with search query:[/cyan] {query}")
        crawler = NyaaCrawler()
        crawler.config["nyaa"]["search_query"] = query
        crawler.crawl()
        logger.info(f"[cyan]Finished crawl for:[/cyan] {query}")

if __name__ == "__main__":
    main()
