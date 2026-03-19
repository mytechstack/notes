⏺ System Components                                                                                                                                                                                                                                  
                                         
    Browser ──► POST /hydrate ──► Hydrator ──► Backend Services ──► Redis                                                                                                                                                                              
    │              │                │                                │                                                                                                                                                                               
    │         (pre-auth)       validates JWT              GET /data (post-auth)                                                                                                                                                                      
    │                                                                                                                                                                                                                                                
  persistent JWT cookie (hyd_token)                         
  session token (separate, post-auth)                                                      
                                                            
  ---                                                                                                                                                                                                                                                
  Threat Model (STRIDE)                                     
                       
  1. Spoofing — can an attacker impersonate a legitimate user?
                                                                                                                                                                                                                                                     
  ┌──────────────────────────────────────────┬────────────┬────────┬───────────────────────────────────────────────┐                                                                                                                                 
  │                  Threat                  │ Likelihood │ Impact │                  Mitigation                   │                                                                                                                                 
  ├──────────────────────────────────────────┼────────────┼────────┼───────────────────────────────────────────────┤                                                                                                                                 
  │ Forge a hydration JWT                    │ Low        │ Medium │ HS256 signature — requires secret             │
  ├──────────────────────────────────────────┼────────────┼────────┼───────────────────────────────────────────────┤
  │ Forge hyd_token value inside a valid JWT │ None       │ —      │ Opaque token — backend rejects unknown tokens │                                                                                                                                 
  ├──────────────────────────────────────────┼────────────┼────────┼───────────────────────────────────────────────┤                                                                                                                                 
  │ Steal JWT via XSS                        │ Medium     │ Medium │ HttpOnly blocks JS access                     │                                                                                                                                 
  ├──────────────────────────────────────────┼────────────┼────────┼───────────────────────────────────────────────┤                                                                                                                                 
  │ Steal JWT via network                    │ Low        │ Medium │ Secure flag — HTTPS only                      │
  ├──────────────────────────────────────────┼────────────┼────────┼───────────────────────────────────────────────┤                                                                                                                                 
  │ Steal JWT via physical device access     │ Low        │ Low    │ Short Path=/hydrate scope limits blast radius │
  └──────────────────────────────────────────┴────────────┴────────┴───────────────────────────────────────────────┘                                                                                                                                 
   
  Residual risk: stolen JWT lets attacker warm cache for victim — but they still can't read the data without a session token.                                                                                                                        
                                                            
  ---                                                                                                                                                                                                                                                
  2. Tampering — can an attacker modify data in transit or at rest?
                                                                                                                                                                                                                                                     
  ┌───────────────────────────────────┬────────────┬────────┬─────────────────────────────────────────────────────────┐
  │              Threat               │ Likelihood │ Impact │                       Mitigation                        │                                                                                                                              
  ├───────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────┤
  │ Tamper with JWT payload           │ None       │ —      │ Signature invalidates tampered tokens                   │                                                                                                                              
  ├───────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────┤
  │ Tamper with Redis cache directly  │ Low        │ High   │ Redis should not be publicly accessible; use AUTH + TLS │                                                                                                                              
  ├───────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────┤                                                                                                                              
  │ MITM between hydrator and backend │ Low        │ High   │ Internal service communication over TLS                 │                                                                                                                              
  ├───────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────┤                                                                                                                              
  │ MITM between hydrator and Redis   │ Low        │ High   │ Redis TLS (rediss://)                                   │
  └───────────────────────────────────┴────────────┴────────┴─────────────────────────────────────────────────────────┘                                                                                                                              
                                                            
  ---                                                                                                                                                                                                                                                
  3. Repudiation — can actions be denied?                   
                                                                                                                                                                                                                                                     
  ┌────────────────────────────────────────────┬────────────┬────────┬────────────────────────────────────────────────────────────┐
  │                   Threat                   │ Likelihood │ Impact │                         Mitigation                         │                                                                                                                  
  ├────────────────────────────────────────────┼────────────┼────────┼────────────────────────────────────────────────────────────┤                                                                                                                  
  │ No audit trail for who triggered hydration │ Medium     │ Low    │ Log hyd_token hash (not value) + IP on every /hydrate call │
  ├────────────────────────────────────────────┼────────────┼────────┼────────────────────────────────────────────────────────────┤                                                                                                                  
  │ Backend resolves token with no record      │ Medium     │ Medium │ Backend should log token resolution with timestamp         │                                                                                                                  
  └────────────────────────────────────────────┴────────────┴────────┴────────────────────────────────────────────────────────────┘                                                                                                                  
                                                                                                                                                                                                                                                     
  ---                                                                                                                                                                                                                                                
  4. Information Disclosure — can an attacker read user data?
                                                             
  ┌────────────────────────────────────────────┬────────────┬────────┬─────────────────────────────────────────────┐
  │                   Threat                   │ Likelihood │ Impact │                 Mitigation                  │                                                                                                                                 
  ├────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────┤
  │ Read cached data via /data without session │ None       │ —      │ /data requires separate session token       │                                                                                                                                 
  ├────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────┤
  │ JWT payload reveals user identity          │ None       │ —      │ Only hyd_token (opaque) in payload — no PII │                                                                                                                                 
  ├────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────┤                                                                                                                                 
  │ hyd_token leaked in logs                   │ Medium     │ Medium │ Never log raw token — log a hash if needed  │                                                                                                                                 
  ├────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────┤                                                                                                                                 
  │ Response leaks user_id                     │ Fixed      │ High   │ Handler returns 202 with no body            │
  ├────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────┤                                                                                                                                 
  │ Redis key reveals user_id                  │ Low        │ Medium │ Use hashed keys e.g. ctx:sha256(user_id)    │
  ├────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────┤                                                                                                                                 
  │ Backend error response leaks user details  │ Low        │ Medium │ Backend returns generic errors to hydrator  │
  └────────────────────────────────────────────┴────────────┴────────┴─────────────────────────────────────────────┘                                                                                                                                 
                                                            
  ---                                                                                                                                                                                                                                                
  5. Denial of Service — can an attacker degrade availability?
                                                                                                                                                                                                                                                     
  ┌──────────────────────────────────────────────┬────────────┬────────┬─────────────────────────────────────────────────────────────────────────────────┐
  │                    Threat                    │ Likelihood │ Impact │                                   Mitigation                                    │                                                                                           
  ├──────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────────┤
  │ Flood /hydrate with valid-looking JWTs       │ High       │ High   │ Rate limit per IP — 5–10 req/min                                                │                                                                                           
  ├──────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────────┤
  │ Flood with expired/invalid JWTs              │ High       │ Low    │ Rejected at hydrator before hitting backend                                     │                                                                                           
  ├──────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────────┤                                                                                           
  │ Exhaust Redis memory with fake cache entries │ Low        │ High   │ Only hydrate after backend confirms token — no entry written for unknown tokens │                                                                                           
  ├──────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────────┤                                                                                           
  │ Slow backend causes goroutine accumulation   │ Medium     │ Medium │ BACKEND_TIMEOUT_SECS already caps this — also cap concurrent hydrations         │
  └──────────────────────────────────────────────┴────────────┴────────┴─────────────────────────────────────────────────────────────────────────────────┘                                                                                           
                                                            
  ---                                                                                                                                                                                                                                                
  6. Elevation of Privilege — can an attacker gain more access than intended?
                                                                                                                                                                                                                                                     
  ┌───────────────────────────────────────────────┬────────────┬────────┬─────────────────────────────────────────────────────────────────────────────┐
  │                    Threat                     │ Likelihood │ Impact │                                 Mitigation                                  │                                                                                              
  ├───────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────┤
  │ Use hydration token to access /data           │ None       │ —      │ Two separate tokens, separate auth                                          │                                                                                              
  ├───────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────┤
  │ Replay old hydration JWT after user logout    │ Medium     │ Low    │ Acceptable by design — hydration only warms cache, data still needs session │                                                                                              
  ├───────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────┤                                                                                              
  │ Hydrate stale permissions after role change   │ Medium     │ High   │ Short Redis TTL on permissions (60–120s) vs longer TTL for profile (hours)  │                                                                                              
  ├───────────────────────────────────────────────┼────────────┼────────┼─────────────────────────────────────────────────────────────────────────────┤                                                                                              
  │ Compromise hydrator to reach backend services │ Low        │ High   │ Hydrator has no write access to backends — read-only fetch                  │
  └───────────────────────────────────────────────┴────────────┴────────┴─────────────────────────────────────────────────────────────────────────────┘                                                                                              
                                                            
  ---                                                                                                                                                                                                                                                
  Risk Summary                                              
                                                                                                                                                                                                                                                     
  Critical    Redis exposed publicly         → never expose Redis port
              Backend services exposed       → internal network only, no public route                                                                                                                                                                
                                                                                                                                                                                                                                                     
  High        Stale permissions in cache     → short TTL on permissions specifically                                                                                                                                                                 
              No rate limiting on /hydrate   → implement before production                                                                                                                                                                           
                                                                                                                                                                                                                                                     
  Medium      JWT stolen via XSS             → HttpOnly mitigates                                                                                                                                                                                    
              hyd_token logged in plaintext  → hash before logging                                                                                                                                                                                   
              Goroutine leak under flood     → cap concurrent hydrations                                                                                                                                                                             
                                                                                                                                                                                                                                                     
  Low/Accept  Attacker warms victim's cache  → no data readable without session                                                                                                                                                                      
              Replay after logout            → by design, low impact                                                                                                                                                                                 
                                                                                                                                                                                                                                                     
  ---                                                                                                                                                                                                                                                
  TTL Strategy by resource sensitivity
                                                                                                                                                                                                                                                     
  Since stale permissions is your highest-impact risk:      
                                                                                                                                                                                                                                                     
  ┌─────────────┬───────────────┬─────────────────────────────────────┐                                                                                                                                                                              
  │  Resource   │ Suggested TTL │               Reason                │                                                                                                                                                                              
  ├─────────────┼───────────────┼─────────────────────────────────────┤                                                                                                                                                                              
  │ permissions │ 60–120s       │ Role changes must propagate quickly │
  ├─────────────┼───────────────┼─────────────────────────────────────┤
  │ preferences │ 1–4 hours     │ Low sensitivity, rarely changes     │                                                                                                                                                                              
  ├─────────────┼───────────────┼─────────────────────────────────────┤                                                                                                                                                                              
  │ profile     │ 4–24 hours    │ Stable, low sensitivity             │                                                                                                                                                                              
  ├─────────────┼───────────────┼─────────────────────────────────────┤                                                                                                                                                                              
  │ resources   │ 5–15 minutes  │ Depends on how dynamic              │
  └─────────────┴───────────────┴─────────────────────────────────────┘                                                                                                                                                                              
                                                            
  ---                                                                                                                                                                                                                                                
  What your architecture gets right                         
                                                                                                                                                                                                                                                     
  - Hydration token and session token are fully decoupled — stealing one gives nothing without the other
  - No PII in the JWT payload                                                                                                                                                                                                                        
  - Backend is the trust anchor, not the cookie                                                                                                                                                                                                      
  - Unauthenticated surface (/hydrate) has no read capability                                                                                                                                                                                        
                                                                                                                                                                                                                                                     
  The two things worth fixing before production: rate limiting on /hydrate and short TTL on permissions.  