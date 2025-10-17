#!/usr/bin/env python3

import json
import time
from datetime import datetime
from camoufox.sync_api import Camoufox

def search_tshirts_with_size_info():
    """Search for t-shirts and filter for those with size info"""
    
    # Alternative search strategies that might work better
    search_queries = [
        "https://www.amazon.de/s?k=herren+t-shirt+groessentabelle&i=fashion",
        "https://www.amazon.de/s?k=t-shirt+groesse+laenge&i=fashion", 
        "https://www.amazon.de/s?k=t-shirt+size+chart&i=fashion",
        "https://www.amazon.de/s?k=t-shirt+masse&i=fashion"
    ]
    
    all_products = []
    
    with Camoufox(headless=False, humanize=True) as browser:
        page = browser.new_page()
        
        for query_url in search_queries:
            print(f"\n🔍 Trying search: {query_url}")
            
            try:
                page.goto(query_url, wait_until='domcontentloaded', timeout=30000)
                time.sleep(3)
                
                title = page.title()
                print(f"Page title: {title}")
                
                if "Tut uns Leid" in title:
                    print("❌ Error page, trying next query...")
                    continue
                
                # If we get here, the search worked!
                print("✅ Search successful!")
                
                # Extract products
                products = page.query_selector_all('[data-component-type="s-search-result"]')
                print(f"Found {len(products)} products")
                
                # Look for products with size-related keywords
                size_keywords = ['größe', 'groesse', 'länge', 'laenge', 'breite', 'size', 'chart', 'maße', 'masse', 'tabelle']
                
                for product in products[:20]:  # Check first 20 products
                    try:
                        asin = product.get_attribute('data-asin')
                        if not asin:
                            continue
                        
                        title_elem = product.query_selector('h2 a span')
                        title = title_elem.text_content().strip() if title_elem else ''
                        
                        # Check title and description for size keywords
                        text_content = product.text_content().lower()
                        has_size_info = any(keyword in text_content for keyword in size_keywords)
                        
                        if has_size_info or "größ" in text_content or "läng" in text_content:
                            price_elem = product.query_selector('.a-price-whole')
                            price = price_elem.text_content().strip() if price_elem else ''
                            
                            link_elem = product.query_selector('h2 a')
                            href = link_elem.get_attribute('href') if link_elem else ''
                            product_url = f"https://www.amazon.de{href}" if href.startswith('/') else href
                            
                            product_data = {
                                'asin': asin,
                                'title': title,
                                'price': price,
                                'url': product_url,
                                'search_query': query_url,
                                'scraped_at': datetime.now().isoformat()
                            }
                            
                            all_products.append(product_data)
                            print(f"✓ {asin}: {title[:50]}... (HAS SIZE INFO)")
                    
                    except Exception as e:
                        continue
                
                # If we found products, don't try other searches
                if len(all_products) > 10:
                    print(f"\n✅ Found enough products with size info!")
                    break
                    
            except Exception as e:
                print(f"❌ Error: {e}")
                continue
        
        # Try browsing to a specific category if searches fail
        if len(all_products) < 5:
            print("\n🔍 Trying category browse approach...")
            category_url = "https://www.amazon.de/s?i=fashion&rh=n%3A77028031%2Cp_n_size_browse-vebin%3A22636052031&dc&qid=1635789012&rnid=22636040031&ref=sr_nr_p_n_size_browse-vebin_1"
            
            try:
                page.goto(category_url, wait_until='domcontentloaded', timeout=30000)
                time.sleep(3)
                
                title = page.title()
                if "Tut uns Leid" not in title:
                    print("✅ Category page loaded!")
                    # Extract products from category page
                    # ... (similar extraction logic)
                    
            except:
                pass
    
    # Save results
    if all_products:
        with open('tshirts-with-sizes.json', 'w', encoding='utf-8') as f:
            json.dump(all_products, f, ensure_ascii=False, indent=2)
        
        print(f"\n✅ Found {len(all_products)} products with size information!")
        print("Results saved to: tshirts-with-sizes.json")
    else:
        print("\n❌ No products found. Amazon might be blocking all automated searches.")
        
        # Provide manual alternative
        print("\n💡 Alternative approach:")
        print("1. Open Amazon.de in your regular browser")
        print("2. Search for: t-shirt größentabelle")
        print("3. Use a browser extension to extract all product links")
        print("4. Save the ASINs to a file")
        print("5. Use the scraper to process individual product pages")

if __name__ == '__main__':
    search_tshirts_with_size_info()