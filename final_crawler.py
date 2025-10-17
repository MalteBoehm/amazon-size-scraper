#!/usr/bin/env python3

import json
import time
import sys
from datetime import datetime
from camoufox.sync_api import Camoufox

def crawl_amazon_tshirts(max_pages=5):
    """Crawl Amazon for t-shirts with size information"""
    
    # This search query works!
    start_url = "https://www.amazon.de/s?k=t-shirt+groesse+laenge&i=fashion"
    
    print(f"🚀 Starting Amazon T-Shirt Crawler")
    print(f"Search: t-shirt groesse laenge")
    print(f"Max pages: {max_pages}")
    print("="*80)
    
    all_products = []
    
    with Camoufox(headless=False, humanize=True) as browser:
        page = browser.new_page()
        current_url = start_url
        
        for page_num in range(1, max_pages + 1):
            print(f"\n📄 Page {page_num}")
            print(f"URL: {current_url}")
            
            try:
                # Navigate to page
                page.goto(current_url, wait_until='domcontentloaded', timeout=30000)
                time.sleep(3)
                
                # Check if page loaded successfully
                title = page.title()
                print(f"Page title: {title}")
                
                if "Tut uns Leid" in title:
                    print("❌ Error page detected. Stopping.")
                    break
                
                # Wait for products to load
                page.wait_for_selector('[data-component-type="s-search-result"]', timeout=10000)
                
                # Extract all products
                products = page.query_selector_all('[data-component-type="s-search-result"]')
                print(f"Found {len(products)} products on page")
                
                page_products = []
                for product in products:
                    try:
                        asin = product.get_attribute('data-asin')
                        if not asin:
                            continue
                        
                        # Extract title
                        title_elem = product.query_selector('h2 a span')
                        title = title_elem.text_content().strip() if title_elem else ''
                        
                        # Extract price
                        price_elem = product.query_selector('.a-price-whole')
                        price = price_elem.text_content().strip() if price_elem else ''
                        
                        # Extract URL
                        link_elem = product.query_selector('h2 a')
                        href = link_elem.get_attribute('href') if link_elem else ''
                        product_url = f"https://www.amazon.de{href}" if href.startswith('/') else href
                        
                        # Check if product likely has size table
                        text_content = product.text_content().lower()
                        size_keywords = ['größe', 'groesse', 'länge', 'laenge', 'breite', 'size', 'chart', 'maße', 'masse', 'tabelle', 'cm']
                        has_size_indicators = any(keyword in text_content for keyword in size_keywords)
                        
                        product_data = {
                            'asin': asin,
                            'title': title,
                            'price': price,
                            'url': product_url,
                            'page': page_num,
                            'likely_has_size_table': has_size_indicators,
                            'scraped_at': datetime.now().isoformat()
                        }
                        
                        page_products.append(product_data)
                        
                        # Print progress
                        marker = "✓" if has_size_indicators else " "
                        print(f"{marker} {asin}: {title[:60]}...")
                        
                    except Exception as e:
                        print(f"Error extracting product: {e}")
                        continue
                
                all_products.extend(page_products)
                print(f"\nExtracted {len(page_products)} products from page {page_num}")
                print(f"Total products so far: {len(all_products)}")
                
                # Save after each page
                with open('amazon-tshirts-complete.json', 'w', encoding='utf-8') as f:
                    json.dump(all_products, f, ensure_ascii=False, indent=2)
                
                # Look for next page
                if page_num < max_pages:
                    next_selectors = [
                        'a.s-pagination-next:not(.s-pagination-disabled)',
                        '.s-pagination-strip a:has-text("Weiter")',
                        'a:has-text("Weiter")'
                    ]
                    
                    next_url = None
                    for selector in next_selectors:
                        next_elem = page.query_selector(selector)
                        if next_elem:
                            href = next_elem.get_attribute('href')
                            if href:
                                next_url = f"https://www.amazon.de{href}" if href.startswith('/') else href
                                break
                    
                    if next_url:
                        print(f"\n➡️  Next page found: {next_url[:80]}...")
                        current_url = next_url
                        time.sleep(5)  # Rate limit between pages
                    else:
                        print("\n❌ No next page found")
                        break
                
            except Exception as e:
                print(f"❌ Error on page {page_num}: {e}")
                break
    
    # Summary
    print("\n" + "="*80)
    print("📊 SUMMARY")
    print(f"Total products collected: {len(all_products)}")
    
    products_with_size = [p for p in all_products if p['likely_has_size_table']]
    print(f"Products likely to have size tables: {len(products_with_size)}")
    
    # Save final results
    with open('amazon-tshirts-complete.json', 'w', encoding='utf-8') as f:
        json.dump(all_products, f, ensure_ascii=False, indent=2)
    
    # Save just the URLs
    with open('product-urls.txt', 'w', encoding='utf-8') as f:
        for product in all_products:
            f.write(f"{product['url']}\n")
    
    # Save ASINs for products with size info
    with open('asins-with-sizes.txt', 'w', encoding='utf-8') as f:
        for product in products_with_size:
            f.write(f"{product['asin']}\n")
    
    print(f"\n✅ Results saved to:")
    print("  - amazon-tshirts-complete.json (all data)")
    print("  - product-urls.txt (just URLs)")
    print("  - asins-with-sizes.txt (ASINs likely to have size tables)")

if __name__ == '__main__':
    max_pages = int(sys.argv[1]) if len(sys.argv) > 1 else 3
    crawl_amazon_tshirts(max_pages)