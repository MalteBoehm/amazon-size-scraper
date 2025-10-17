# Amazon Size Scraper - Project Plan

## Current Status
- Project Phase: Architecture Design
- Last Updated: 2025-07-03
- Status: Planning

## Sprint Tasks

### Sprint 1: Foundation (Current)
- [x] Evaluate Camoufox browser (Completed - Not suitable for production)
- [x] Document system architecture
- [ ] Set up Go project structure
- [ ] Implement basic scraper prototype with Playwright-go
- [ ] Create data models for product dimensions

### Sprint 2: Core Implementation
- [ ] Build request queue system
- [ ] Implement proxy rotation
- [ ] Create HTML parser for size extraction
- [ ] Add rate limiting logic
- [ ] Set up database schema

### Sprint 3: Anti-Detection
- [ ] Implement browser fingerprint randomization
- [ ] Add human-like behavior patterns
- [ ] Integrate captcha handling
- [ ] Test against Amazon WAF
- [ ] Monitor detection rates

### Sprint 4: Scalability
- [ ] Containerize application
- [ ] Add horizontal scaling support
- [ ] Implement distributed queue
- [ ] Set up monitoring and alerting
- [ ] Performance optimization

## Technical Decisions Log

| Date | Decision | Rationale | Status |
|------|----------|-----------|---------|
| 2025-07-03 | Reject Camoufox | Maintenance issues, not production-ready | ✓ Decided |
| 2025-07-03 | Use Go + Playwright | Better support, production stability | ✓ Decided |
| TBD | Database choice | PostgreSQL vs MongoDB | ⏳ Pending |
| TBD | Queue system | RabbitMQ vs Redis | ⏳ Pending |
| TBD | Proxy provider | Residential vs datacenter | ⏳ Pending |

## Risk Assessment

### High Priority Risks
1. **Legal Compliance**
   - Risk: ToS violation
   - Mitigation: Consider official API first, limit scraping rate

2. **Detection & Blocking**
   - Risk: IP bans, captchas
   - Mitigation: Proxy rotation, human-like patterns

3. **Data Quality**
   - Risk: Incomplete or incorrect size data
   - Mitigation: Validation rules, manual verification

### Medium Priority Risks
1. **Scalability Limits**
   - Risk: Performance bottlenecks
   - Mitigation: Distributed architecture, caching

2. **Maintenance Burden**
   - Risk: Amazon HTML changes
   - Mitigation: Flexible parsers, monitoring

## Milestones

- **M1**: Basic scraper working (Week 2)
- **M2**: Anti-detection implemented (Week 4)
- **M3**: Production-ready system (Week 6)
- **M4**: Scaled deployment (Week 8)

## Research Findings

### Browser Automation Options
1. **Camoufox**: Superior anti-detection but maintenance issues
2. **Playwright**: Good balance of features and stability
3. **Chromedp**: Lightweight but limited stealth features
4. **Commercial APIs**: Most reliable but higher cost

### Amazon.de Specific Challenges
- Dynamic content loading
- A/B testing variations
- Region-specific layouts
- Frequent HTML structure changes

## Next Actions
1. Initialize Go project with proper structure
2. Create Playwright-go proof of concept
3. Test basic size extraction on sample products
4. Evaluate proxy providers
5. Design database schema for dimension data

## Notes
- Keep scraping rate conservative (< 100 req/hour)
- Monitor Amazon robots.txt changes
- Consider hybrid approach with official API
- Document all HTML selectors for maintenance