#!/usr/bin/env python3

print("1. Script started")

try:
    from camoufox.sync_api import Camoufox
    print("2. Camoufox imported")
    
    with Camoufox(headless=True, humanize=True) as browser:
        print("3. Browser created")
        
        page = browser.new_page()
        print("4. Page created")
        
        print("5. Navigating to Amazon...")
        page.goto("https://www.amazon.de", timeout=15000)
        print("6. Navigation complete")
        
        title = page.title()
        print(f"7. Title: {title}")
        
        # Save screenshot
        page.screenshot(path="quick-test-result.png")
        print("8. Screenshot saved")
        
        # Check page content
        body = page.query_selector("body")
        if body:
            text = body.text_content()
            if "robots" in text.lower() or "captcha" in text.lower():
                print("⚠️ CAPTCHA or robot check detected!")
            elif "tut uns leid" in text.lower():
                print("⚠️ Error page detected!")
            else:
                print("✅ Page seems OK")
                
except Exception as e:
    print(f"❌ Error at some point: {e}")
    import traceback
    traceback.print_exc()

print("9. Script finished")