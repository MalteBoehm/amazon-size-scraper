#!/usr/bin/env python3

from camoufox.sync_api import Camoufox
import sys

def quick_test():
    url = sys.argv[1] if len(sys.argv) > 1 else "https://www.amazon.de/dp/B08N5WRWNW"
    
    print(f"Quick Camoufox test with: {url}")
    
    with Camoufox(headless=True, humanize=True) as browser:
        page = browser.new_page()
        
        print("Navigating...")
        page.goto(url)
        page.wait_for_timeout(3000)
        
        # Screenshot
        page.screenshot(path='quick-test.png')
        
        # Title
        title = page.title()
        print(f"Title: {title}")
        
        # Check for captcha
        if page.query_selector('#captchacharacters'):
            print("CAPTCHA DETECTED!")
        elif "Tut uns Leid" in title:
            print("ERROR PAGE!")
        else:
            print("Page loaded successfully")
            
            # Count products
            products = page.query_selector_all('[data-asin]')
            print(f"Products found: {len(products)}")
            
            # Get some text
            body_text = page.query_selector('body').text_content()
            if "Roboter" in body_text:
                print("Robot check detected in text")

if __name__ == '__main__':
    quick_test()