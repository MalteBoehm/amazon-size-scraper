#!/usr/bin/env python3

import time
from camoufox.sync_api import Camoufox

def bypass_amazon_check():
    print("🔓 Amazon Bot-Check Bypass Test")
    
    with Camoufox(headless=False, humanize=True) as browser:
        page = browser.new_page()
        
        # First, go to Amazon
        print("1. Navigating to Amazon.de...")
        page.goto("https://www.amazon.de", timeout=20000)
        time.sleep(2)
        
        # Check if we get the bot check
        title = page.title()
        print(f"2. Page title: {title}")
        
        # Look for the "Weiter shoppen" button
        button_selectors = [
            'button:has-text("Weiter shoppen")',
            'input[type="submit"][value*="Weiter"]',
            '.a-button-primary',
            'button.a-button-text'
        ]
        
        button_found = False
        for selector in button_selectors:
            button = page.query_selector(selector)
            if button:
                print(f"3. Found button with selector: {selector}")
                print("4. Clicking button...")
                button.click()
                button_found = True
                break
        
        if button_found:
            # Wait for navigation
            time.sleep(3)
            
            # Check new page
            new_title = page.title()
            print(f"5. New page title: {new_title}")
            
            if "Amazon.de" in new_title and "Klicke" not in page.content():
                print("✅ Successfully bypassed bot check!")
                
                # Now try our search
                print("\n6. Trying search...")
                search_url = "https://www.amazon.de/s?k=t-shirt+groesse+laenge&i=fashion"
                page.goto(search_url, timeout=20000)
                time.sleep(3)
                
                search_title = page.title()
                print(f"7. Search page title: {search_title}")
                
                if "Tut uns Leid" not in search_title:
                    products = page.query_selector_all('[data-component-type="s-search-result"]')
                    print(f"8. Products found: {len(products)}")
                    
                    if len(products) > 0:
                        print("\n✅ SUCCESS! We can now scrape products!")
                        
                        # Save some ASINs as proof
                        asins = []
                        for i, product in enumerate(products[:5]):
                            asin = product.get_attribute('data-asin')
                            if asin:
                                asins.append(asin)
                                title_elem = product.query_selector('h2 a span')
                                title = title_elem.text_content()[:50] if title_elem else ''
                                print(f"   {asin}: {title}...")
                        
                        return True
                else:
                    print("❌ Still getting error page on search")
            else:
                print("❌ Bot check bypass didn't work")
        else:
            print("❌ Couldn't find button to click")
        
        # Keep browser open to investigate
        print("\n💡 Browser is open for manual inspection")
        print("Press Ctrl+C to close")
        
        try:
            while True:
                time.sleep(1)
        except KeyboardInterrupt:
            print("\nClosing...")
    
    return False

if __name__ == '__main__':
    bypass_amazon_check()