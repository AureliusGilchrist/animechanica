#!/usr/bin/env python3

# Standard library imports
import argparse
import json
import logging
import os
import sys
import time
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
        else:
            self.config = {
                "qbittorrent": {
                    "url": "http://localhost:8081/api/v2",
                    "username": "admin",
                    "password": "lolmao"
                },
                "nyaa": {
                    "base_url": "https://nyaa.si",
                    "search_query": "[Judas] batch",
                    "category": "0_0",  # All categories
                    "filter": "0",      # No filter
                    "start_page": 1,
                    "end_page": 1000,
                    "delay": 1          # Seconds between requests
                },
                "download_path": "/aeternae/functional/torrents/torrent_files"
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

            title_col = cols[1].find('a', {'title': True})
            if not title_col:
                return None

            # Find the magnet link
            magnet = row.find('a', href=lambda href: href and href.startswith('magnet:?'))
            if not magnet:
                return None

            seeders = int(cols[5].text.strip())
            if seeders <= 0:
                logger.debug(f"Skipping torrent with 0 seeders: {title_col['title']}")
                return None

            return {
                'title': title_col['title'],
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

    def add_to_qbittorrent(self, magnet: str) -> bool:
        """
        Add magnet link to qBittorrent.

        Args:
            magnet (str): Magnet link

        Returns:
            bool: True if addition is successful, False otherwise
        """
        try:
            response = self.session.post(
                f"{self.config['qbittorrent']['url']}/torrents/add",
                data={
                    "urls": magnet,
                    "savepath": self.config['download_path']
                }
            )
            
            if response.status_code == 200:
                logger.info("[green]Successfully added torrent to qBittorrent[/green]")
                return True
            else:
                logger.error(f"[red]Failed to add torrent:[/red] {response.text}")
                return False
                
        except Exception as e:
            logger.error(f"[red]Error adding torrent:[/red] {str(e)}")
            return False

    def crawl(self) -> None:
        """
        Main crawling function that processes pages and adds torrents to qBittorrent.
        
        This function:
        - Iterates through pages based on configuration
        - Processes each page to extract torrent information
        - Adds valid torrents to qBittorrent
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
                        if self.add_to_qbittorrent(torrent['magnet']):
                            added_torrents += 1
                            log_torrent_added(torrent['title'], torrent['size'], torrent['seeders'])
                        
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

def main(search_query=None):
    crawler = NyaaCrawler()
    if search_query is not None:
        crawler.config["nyaa"]["search_query"] = search_query
    crawler.crawl()

if __name__ == "__main__":
    Search = ["[Judas] batch", "[DB] batch", "[EMBER] batch, [smol] monogatari", "[Erai-raws] batch", "[SubsPlease] batch", "[HorribleSubs] batch", "[Trix] batch"]
    for query in Search:
        main(query)
