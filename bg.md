Microfrontend Platform System Overview
I'll provide you with a comprehensive system overview for building a microfrontend platform using esm.sh for module composition.
Core Architecture
1. Platform Components
Module Registry & Catalog

Central registry storing metadata about available microfrontends (name, version, entry points, dependencies)
Module catalog UI for browsing and discovering available modules
Version management and dependency tracking
Module metadata includes: configuration schema, required props, exposed APIs

Composition Engine

Runtime orchestrator that loads and integrates selected modules
Handles module lifecycle (bootstrap, mount, unmount)
Manages inter-module communication via event bus or shared state
Resolves and loads ES modules from esm.sh on-demand

Application Builder (Self-Service UI)

Drag-and-drop or configuration-based interface for composing applications
Real-time preview of composed application
Configuration editor for module settings
Layout designer for arranging modules on the page
Export/import composition configurations as JSON

Runtime Container

Host application shell that provides common infrastructure
Routing system for multi-page applications
Shared services layer (authentication, theming, notifications)
Error boundary and fallback handling
CSS isolation strategies (Shadow DOM, scoped styles, CSS modules)

2. Technical Flow
User selects modules → Configuration stored → 
Runtime loads config → esm.sh resolves modules → 
Modules instantiated → Inter-module communication established → 
Application renders
3. Key Integration Points with esm.sh
Module Loading Strategy

Use esm.sh CDN URLs: https://esm.sh/[package]@[version]
Import maps for dependency management
Leverage esm.sh's built-in bundling and tree-shaking
Pin specific versions or use semver ranges

Example Loading Pattern:
javascriptconst moduleUrl = `https://esm.sh/react@18.2.0`;
const Module = await import(moduleUrl);
```

### 4. **Data Model**

**Application Composition Schema:**
- Application ID and metadata
- Layout configuration (grid, slots, regions)
- Module instances with configurations
- Routing definitions
- Shared state and context

**Module Definition Schema:**
- Module identifier and version
- esm.sh package reference
- Configuration schema (JSON Schema)
- Props interface
- Exposed API surface
- Dependencies and peer dependencies

### 5. **Core Features**

**Module Isolation**
- Style encapsulation (CSS-in-JS, Shadow DOM, or scoped CSS)
- JavaScript sandboxing for untrusted modules
- Independent error handling per module

**Communication Patterns**
- Event bus for publish/subscribe
- Shared state management (optional Redux/Zustand store)
- Props drilling from container to modules
- Custom events for cross-module interaction

**Development Experience**
- Hot module replacement for development
- Module versioning and rollback
- A/B testing support for different module versions
- Analytics and performance monitoring per module

### 6. **Infrastructure Considerations**

**Backend Services**
- Composition API for saving/loading configurations
- Module registry API
- User authentication and authorization
- Analytics and telemetry collection

**Storage**
- User compositions (PostgreSQL/MongoDB)
- Module metadata cache
- CDN caching strategy for esm.sh responses

**Security**
- Content Security Policy (CSP) configuration
- Subresource Integrity (SRI) for loaded modules
- Authorization rules per module
- XSS prevention in module sandboxing

### 7. **Deployment Architecture**
```
┌─────────────────────────────────────────────┐
│           CDN (Static Assets)               │
│     Platform Shell & Builder UI             │
└─────────────────────────────────────────────┘
                    │
                    ↓
┌─────────────────────────────────────────────┐
│        Backend API Services                 │
│   - Composition API                         │
│   - Module Registry                         │
│   - Auth Service                            │
└─────────────────────────────────────────────┘
                    │
                    ↓
┌─────────────────────────────────────────────┐
│         esm.sh CDN                          │
│   (Dynamic Module Resolution)               │
└─────────────────────────────────────────────┘
8. Implementation Phases
Phase 1: Foundation

Basic module loader using esm.sh
Simple container shell application
Module registry with basic metadata

Phase 2: Composition

Self-service builder UI
Configuration persistence
Layout management system

Phase 3: Advanced Features

Inter-module communication
Advanced routing
Performance optimization
Security hardening

Phase 4: Enterprise Features

Role-based access control
Multi-tenancy support
Advanced analytics
CI/CD integration for module publishing

This architecture provides flexibility, scalability, and a genuine self-service experience while leveraging esm.sh's powerful module resolution capabilities. Would you like me to dive deeper into any specific component or create a proof-of-concept implementation?SHow to achieve dependency management and blue greenDependency Management & Blue-Green Deployment for Microfrontends
Dependency Management Strategy
1. Shared Dependencies with Import Maps
The most efficient approach is using Import Maps to deduplicate common dependencies across microfrontends.
Implementation:
javascript// Generated import map based on selected modules
{
  "imports": {
    "react": "https://esm.sh/react@18.2.0",
    "react-dom": "https://esm.sh/react-dom@18.2.0",
    "lodash": "https://esm.sh/lodash@4.17.21",
    
    // Module-specific imports
    "@my-org/module-a": "https://esm.sh/@my-org/module-a@1.2.3",
    "@my-org/module-b": "https://esm.sh/@my-org/module-b@2.0.1"
  },
  "scopes": {
    // Handle version conflicts with scopes
    "https://esm.sh/@my-org/module-legacy/": {
      "react": "https://esm.sh/react@17.0.2"
    }
  }
}
Dynamic Import Map Generation:
javascriptclass DependencyManager {
  constructor() {
    this.sharedDeps = new Map();
    this.moduleRegistry = new Map();
  }

  // Analyze all selected modules and resolve dependencies
  async analyzeDependencies(modules) {
    const depGraph = {};
    
    for (const module of modules) {
      const metadata = await this.fetchModuleMetadata(module);
      depGraph[module.id] = {
        version: module.version,
        dependencies: metadata.dependencies,
        peerDependencies: metadata.peerDependencies
      };
    }
    
    return this.resolveConflicts(depGraph);
  }

  resolveConflicts(depGraph) {
    const resolved = {};
    const conflicts = [];
    
    // Collect all dependency versions
    const allDeps = {};
    
    for (const [moduleId, info] of Object.entries(depGraph)) {
      for (const [dep, version] of Object.entries(info.dependencies)) {
        if (!allDeps[dep]) {
          allDeps[dep] = [];
        }
        allDeps[dep].push({ moduleId, version });
      }
    }
    
    // Resolve to single version or detect conflicts
    for (const [dep, versions] of Object.entries(allDeps)) {
      const uniqueVersions = [...new Set(versions.map(v => v.version))];
      
      if (uniqueVersions.length === 1) {
        // No conflict - use shared version
        resolved[dep] = uniqueVersions[0];
      } else {
        // Conflict detected - use scoped versions
        conflicts.push({
          dependency: dep,
          versions: versions
        });
      }
    }
    
    return { resolved, conflicts };
  }

  generateImportMap(resolved, conflicts) {
    const importMap = {
      imports: {},
      scopes: {}
    };
    
    // Add resolved shared dependencies
    for (const [dep, version] of Object.entries(resolved)) {
      importMap.imports[dep] = `https://esm.sh/${dep}@${version}`;
    }
    
    // Handle conflicts with scoped imports
    for (const conflict of conflicts) {
      for (const { moduleId, version } of conflict.versions) {
        const moduleScope = `https://esm.sh/@my-org/${moduleId}/`;
        if (!importMap.scopes[moduleScope]) {
          importMap.scopes[moduleScope] = {};
        }
        importMap.scopes[moduleScope][conflict.dependency] = 
          `https://esm.sh/${conflict.dependency}@${version}`;
      }
    }
    
    return importMap;
  }

  // Inject import map into document
  injectImportMap(importMap) {
    const script = document.createElement('script');
    script.type = 'importmap';
    script.textContent = JSON.stringify(importMap, null, 2);
    document.head.appendChild(script);
  }
}
2. Version Pinning & Resolution Strategies
Module Metadata Schema:
javascript// Stored in your module registry
{
  "moduleId": "@my-org/dashboard-widget",
  "version": "2.1.0",
  "entryPoint": "https://esm.sh/@my-org/dashboard-widget@2.1.0",
  
  "dependencies": {
    "react": "^18.0.0",      // Flexible
    "react-dom": "^18.0.0",
    "chart.js": "~4.2.0"     // Patch updates only
  },
  
  "peerDependencies": {
    "@my-org/design-system": "^3.0.0"
  },
  
  "bundledDependencies": [
    "some-specific-lib"      // Not shared, bundled with module
  ],
  
  "esm.sh": {
    "deps": ["react@18.2.0", "react-dom@18.2.0"],
    "bundle": false,         // Let esm.sh handle bundling
    "target": "es2020"
  }
}
Dependency Resolution Service:
javascriptclass DependencyResolver {
  async resolveModuleDependencies(modules) {
    // Fetch all module metadata
    const metadataPromises = modules.map(m => 
      this.fetchMetadata(m.id, m.version)
    );
    const allMetadata = await Promise.all(metadataPromises);
    
    // Build dependency tree
    const depTree = this.buildDependencyTree(allMetadata);
    
    // Resolve versions using semver
    const resolved = this.resolveVersions(depTree);
    
    // Generate esm.sh URLs with explicit deps
    return this.generateEsmUrls(resolved);
  }

  generateEsmUrls(resolved) {
    const urls = {};
    
    for (const [pkg, version] of Object.entries(resolved)) {
      // Build esm.sh URL with dependency hints
      const deps = this.getDepsForPackage(pkg, version);
      const depsQuery = deps.map(d => `deps=${d}`).join('&');
      
      urls[pkg] = `https://esm.sh/${pkg}@${version}${depsQuery ? '?' + depsQuery : ''}`;
    }
    
    return urls;
  }
}
3. Caching & Performance
javascriptclass ModuleCache {
  constructor() {
    this.memoryCache = new Map();
    this.cacheVersion = '1.0';
  }

  async loadModule(url, version) {
    const cacheKey = `${url}@${version}`;
    
    // Check memory cache
    if (this.memoryCache.has(cacheKey)) {
      return this.memoryCache.get(cacheKey);
    }
    
    // Check browser cache (Cache API)
    const cached = await this.getCachedModule(cacheKey);
    if (cached) {
      this.memoryCache.set(cacheKey, cached);
      return cached;
    }
    
    // Load fresh
    const module = await import(url);
    
    // Cache for future use
    await this.cacheModule(cacheKey, url);
    this.memoryCache.set(cacheKey, module);
    
    return module;
  }

  async cacheModule(key, url) {
    const cache = await caches.open(`microfrontend-v${this.cacheVersion}`);
    await cache.add(url);
  }

  async getCachedModule(key) {
    const cache = await caches.open(`microfrontend-v${this.cacheVersion}`);
    const response = await cache.match(key);
    return response ? response.json() : null;
  }
}

Blue-Green Deployment Strategy
1. Version-Based Routing Architecture
javascriptclass VersionRouter {
  constructor() {
    this.activeEnvironment = 'blue';  // or 'green'
    this.environments = {
      blue: {
        version: '1.0',
        modules: {}
      },
      green: {
        version: '2.0',
        modules: {}
      }
    };
  }

  // Load environment configuration from backend
  async loadEnvironmentConfig(envName) {
    const config = await fetch(`/api/environments/${envName}`);
    return config.json();
  }

  // Get module URL based on active environment
  getModuleUrl(moduleId, environment = null) {
    const env = environment || this.activeEnvironment;
    const moduleConfig = this.environments[env].modules[moduleId];
    
    return moduleConfig.url;
  }

  // Switch traffic to new environment
  switchEnvironment(targetEnv) {
    console.log(`Switching from ${this.activeEnvironment} to ${targetEnv}`);
    this.activeEnvironment = targetEnv;
    
    // Notify all modules to reload if necessary
    window.dispatchEvent(new CustomEvent('environment-switch', {
      detail: { environment: targetEnv }
    }));
  }
}
2. Backend Environment Configuration
Database Schema:
sql-- Environments table
CREATE TABLE environments (
  id UUID PRIMARY KEY,
  name VARCHAR(50) UNIQUE NOT NULL,  -- 'blue' or 'green'
  status VARCHAR(20) NOT NULL,        -- 'active', 'standby', 'deploying'
  version VARCHAR(50),
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

-- Module versions per environment
CREATE TABLE environment_modules (
  id UUID PRIMARY KEY,
  environment_id UUID REFERENCES environments(id),
  module_id VARCHAR(255) NOT NULL,
  version VARCHAR(50) NOT NULL,
  esm_url TEXT NOT NULL,
  config JSONB,
  health_status VARCHAR(20) DEFAULT 'healthy',
  created_at TIMESTAMP
);

-- Traffic routing rules
CREATE TABLE traffic_rules (
  id UUID PRIMARY KEY,
  rule_type VARCHAR(50),              -- 'percentage', 'user_group', 'feature_flag'
  environment_id UUID REFERENCES environments(id),
  percentage INTEGER,                 -- For canary deployments
  user_groups TEXT[],
  enabled BOOLEAN DEFAULT true,
  created_at TIMESTAMP
);
API Endpoints:
javascript// Express.js example
app.get('/api/environments/:name', async (req, res) => {
  const { name } = req.params;
  
  const environment = await db.query(
    'SELECT * FROM environments WHERE name = $1',
    [name]
  );
  
  const modules = await db.query(
    `SELECT module_id, version, esm_url, config 
     FROM environment_modules 
     WHERE environment_id = $1`,
    [environment.id]
  );
  
  res.json({
    environment: environment,
    modules: modules
  });
});

// Switch active environment
app.post('/api/environments/switch', async (req, res) => {
  const { targetEnvironment } = req.body;
  
  await db.query(
    `UPDATE environments 
     SET status = CASE 
       WHEN name = $1 THEN 'active'
       ELSE 'standby'
     END`,
    [targetEnvironment]
  );
  
  res.json({ success: true, activeEnvironment: targetEnvironment });
});
3. Frontend Environment Loader
javascriptclass EnvironmentManager {
  constructor() {
    this.currentEnv = null;
    this.moduleRegistry = new Map();
  }

  async initialize() {
    // Get active environment from backend
    const activeEnv = await this.fetchActiveEnvironment();
    await this.loadEnvironment(activeEnv);
    
    // Set up environment polling for switches
    this.startEnvironmentPolling();
  }

  async fetchActiveEnvironment() {
    const response = await fetch('/api/environments/active');
    return response.json();
  }

  async loadEnvironment(envConfig) {
    this.currentEnv = envConfig;
    
    // Generate import map for this environment
    const importMap = this.generateImportMap(envConfig.modules);
    this.injectImportMap(importMap);
    
    // Load all modules for this environment
    for (const module of envConfig.modules) {
      await this.loadModule(module);
    }
  }

  async loadModule(moduleConfig) {
    const module = await import(moduleConfig.esm_url);
    this.moduleRegistry.set(moduleConfig.module_id, {
      instance: module,
      config: moduleConfig.config,
      version: moduleConfig.version
    });
  }

  generateImportMap(modules) {
    const importMap = { imports: {} };
    
    modules.forEach(module => {
      importMap.imports[module.module_id] = module.esm_url;
    });
    
    return importMap;
  }

  injectImportMap(importMap) {
    // Remove old import map
    const oldMap = document.querySelector('script[type="importmap"]');
    if (oldMap) oldMap.remove();
    
    // Inject new import map
    const script = document.createElement('script');
    script.type = 'importmap';
    script.textContent = JSON.stringify(importMap, null, 2);
    document.head.prepend(script);
  }

  // Poll for environment changes
  startEnvironmentPolling() {
    setInterval(async () => {
      const activeEnv = await this.fetchActiveEnvironment();
      
      if (activeEnv.name !== this.currentEnv.name) {
        console.log('Environment switch detected');
        await this.handleEnvironmentSwitch(activeEnv);
      }
    }, 10000); // Check every 10 seconds
  }

  async handleEnvironmentSwitch(newEnv) {
    // Option 1: Hard reload (simplest)
    // window.location.reload();
    
    // Option 2: Graceful reload (better UX)
    await this.gracefulSwitch(newEnv);
  }

  async gracefulSwitch(newEnv) {
    // Unmount all current modules
    for (const [moduleId, moduleInfo] of this.moduleRegistry) {
      if (moduleInfo.instance.unmount) {
        await moduleInfo.instance.unmount();
      }
    }
    
    // Clear registry
    this.moduleRegistry.clear();
    
    // Load new environment
    await this.loadEnvironment(newEnv);
    
    // Remount all modules
    for (const [moduleId, moduleInfo] of this.moduleRegistry) {
      if (moduleInfo.instance.mount) {
        await moduleInfo.instance.mount(moduleInfo.config);
      }
    }
  }
}
4. Canary & Progressive Rollout
javascriptclass TrafficManager {
  constructor() {
    this.rules = [];
  }

  async loadTrafficRules() {
    const response = await fetch('/api/traffic-rules');
    this.rules = await response.json();
  }

  determineEnvironment(userId, userGroups) {
    for (const rule of this.rules) {
      if (!rule.enabled) continue;
      
      switch (rule.rule_type) {
        case 'percentage':
          // Hash-based consistent routing
          const hash = this.hashUserId(userId);
          if (hash % 100 < rule.percentage) {
            return rule.environment_name;
          }
          break;
          
        case 'user_group':
          if (userGroups.some(g => rule.user_groups.includes(g))) {
            return rule.environment_name;
          }
          break;
          
        case 'feature_flag':
          if (this.isFeatureFlagEnabled(rule.flag_name, userId)) {
            return rule.environment_name;
          }
          break;
      }
    }
    
    // Default to current active environment
    return 'blue';
  }

  hashUserId(userId) {
    // Simple hash function for consistent distribution
    let hash = 0;
    for (let i = 0; i < userId.length; i++) {
      hash = ((hash << 5) - hash) + userId.charCodeAt(i);
      hash = hash & hash;
    }
    return Math.abs(hash);
  }
}
5. Health Checks & Rollback
javascriptclass HealthMonitor {
  constructor() {
    this.healthChecks = new Map();
    this.errorThreshold = 0.05; // 5% error rate triggers rollback
  }

  async monitorEnvironment(envName) {
    const modules = await this.getEnvironmentModules(envName);
    
    for (const module of modules) {
      this.healthChecks.set(module.id, {
        errors: 0,
        requests: 0,
        lastCheck: Date.now()
      });
    }
    
    // Start monitoring
    this.startHealthChecking(envName);
  }

  recordModuleError(moduleId) {
    const health = this.healthChecks.get(moduleId);
    if (health) {
      health.errors++;
      health.requests++;
      
      const errorRate = health.errors / health.requests;
      
      if (errorRate > this.errorThreshold) {
        this.triggerRollback(moduleId, errorRate);
      }
    }
  }

  async triggerRollback(moduleId, errorRate) {
    console.error(`Module ${moduleId} error rate ${errorRate} exceeds threshold`);
    
    // Notify operations
    await fetch('/api/alerts/rollback', {
      method: 'POST',
      body: JSON.stringify({
        moduleId,
        errorRate,
        timestamp: new Date().toISOString()
      })
    });
    
    // Automatic rollback to previous environment
    await fetch('/api/environments/rollback', {
      method: 'POST'
    });
  }
}
6. Complete Deployment Workflow
javascript// Orchestrator for blue-green deployments
class DeploymentOrchestrator {
  async deploy(modules, targetVersion) {
    // 1. Determine standby environment
    const standbyEnv = await this.getStandbyEnvironment();
    
    // 2. Deploy new modules to standby
    await this.deployToEnvironment(standbyEnv, modules, targetVersion);
    
    // 3. Run health checks
    const healthy = await this.runHealthChecks(standbyEnv);
    if (!healthy) {
      throw new Error('Health checks failed');
    }
    
    // 4. Start canary rollout (10% traffic)
    await this.startCanaryRollout(standbyEnv, 10);
    
    // 5. Monitor for 10 minutes
    await this.monitorCanary(standbyEnv, 600000);
    
    // 6. Gradually increase traffic
    await this.increaseTraffic(standbyEnv, [25, 50, 75, 100]);
    
    // 7. Full switch
    await this.switchEnvironment(standbyEnv);
    
    // 8. Keep old environment as standby for quick rollback
    console.log('Deployment complete');
  }

  async rollback() {
    const previousEnv = await this.getPreviousEnvironment();
    await this.switchEnvironment(previousEnv);
  }
}
This setup gives you:

Zero-downtime deployments
Instant rollback capability
Gradual traffic shifting
Per-user or per-group routing
Efficient dependency management
Version isolation

Would you like me to create a working implementation of any specific component?SHow to push these dependencies and how can index.html read the importmapsPushing Dependencies & Import Map Integration
1. Publishing Modules to NPM/Registry
Module Structure for Publishing
my-module/
├── package.json
├── src/
│   ├── index.tsx
│   └── components/
├── dist/
│   ├── index.js (ESM)
│   └── index.d.ts (TypeScript definitions)
└── README.md
package.json Configuration:
json{
  "name": "@my-org/dashboard-widget",
  "version": "2.1.0",
  "type": "module",
  "main": "./dist/index.js",
  "module": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "import": "./dist/index.js",
      "types": "./dist/index.d.ts"
    }
  },
  "files": [
    "dist"
  ],
  "peerDependencies": {
    "react": "^18.0.0",
    "react-dom": "^18.0.0"
  },
  "dependencies": {
    "chart.js": "^4.2.0"
  },
  "publishConfig": {
    "access": "public",
    "registry": "https://registry.npmjs.org/"
  },
  "microfrontend": {
    "mountPoint": "mount",
    "unmountPoint": "unmount",
    "configSchema": {
      "type": "object",
      "properties": {
        "apiEndpoint": { "type": "string" },
        "theme": { "type": "string" }
      }
    }
  }
}
Build & Publish Script
javascript// build.js
import { build } from 'esbuild';
import { dtsPlugin } from 'esbuild-plugin-d.ts';

await build({
  entryPoints: ['src/index.tsx'],
  bundle: true,
  format: 'esm',
  outdir: 'dist',
  external: ['react', 'react-dom'], // Don't bundle peer dependencies
  splitting: true,
  sourcemap: true,
  target: 'es2020',
  plugins: [dtsPlugin()]
});

console.log('Build complete!');
Publishing:
bash# Build the module
npm run build

# Publish to NPM
npm publish

# Or to a private registry
npm publish --registry=https://your-private-registry.com
Module Entry Point (src/index.tsx)
typescriptimport React from 'react';
import { createRoot, Root } from 'react-dom/client';
import { DashboardWidget } from './components/DashboardWidget';

interface ModuleConfig {
  apiEndpoint: string;
  theme?: string;
}

let root: Root | null = null;

// Standard microfrontend interface
export function mount(container: HTMLElement, config: ModuleConfig) {
  root = createRoot(container);
  root.render(<DashboardWidget {...config} />);
}

export function unmount() {
  if (root) {
    root.unmount();
    root = null;
  }
}

// Also export components for direct use
export { DashboardWidget };
export type { ModuleConfig };

2. Import Map Generation & Injection
Backend Service - Import Map Generator
javascript// services/importMapGenerator.js
import semver from 'semver';

class ImportMapGenerator {
  constructor(moduleRegistry) {
    this.moduleRegistry = moduleRegistry;
    this.esmShCdn = 'https://esm.sh';
  }

  /**
   * Generate import map for a given application composition
   */
  async generateForComposition(compositionId) {
    // 1. Fetch composition configuration
    const composition = await this.fetchComposition(compositionId);
    
    // 2. Fetch metadata for all modules
    const modulesMetadata = await Promise.all(
      composition.modules.map(m => 
        this.moduleRegistry.getMetadata(m.moduleId, m.version)
      )
    );
    
    // 3. Resolve dependencies
    const resolved = this.resolveDependencies(modulesMetadata);
    
    // 4. Generate import map
    return this.buildImportMap(resolved, composition.modules);
  }

  resolveDependencies(modulesMetadata) {
    const depMap = new Map();
    const conflicts = [];

    // Collect all dependencies
    for (const metadata of modulesMetadata) {
      const allDeps = {
        ...metadata.dependencies,
        ...metadata.peerDependencies
      };

      for (const [dep, versionRange] of Object.entries(allDeps)) {
        if (!depMap.has(dep)) {
          depMap.set(dep, []);
        }
        depMap.get(dep).push({
          moduleId: metadata.name,
          versionRange,
          requestedBy: metadata.name
        });
      }
    }

    // Resolve to single version or detect conflicts
    const resolved = new Map();

    for (const [dep, requests] of depMap.entries()) {
      const versions = requests.map(r => r.versionRange);
      const compatible = this.findCompatibleVersion(versions);

      if (compatible) {
        resolved.set(dep, compatible);
      } else {
        conflicts.push({ dep, requests });
      }
    }

    return { resolved, conflicts };
  }

  findCompatibleVersion(versionRanges) {
    // Get latest versions that satisfy all ranges
    const latestVersions = versionRanges
      .map(range => semver.maxSatisfying(['18.2.0', '18.3.1', '17.0.2'], range))
      .filter(Boolean);

    if (latestVersions.length === 0) return null;

    // Find the highest version that satisfies all ranges
    const highest = latestVersions.reduce((max, v) => 
      semver.gt(v, max) ? v : max
    );

    // Verify this version satisfies all ranges
    const satisfiesAll = versionRanges.every(range => 
      semver.satisfies(highest, range)
    );

    return satisfiesAll ? highest : null;
  }

  buildImportMap(resolved, modules) {
    const importMap = {
      imports: {},
      scopes: {}
    };

    // Add shared dependencies
    for (const [dep, version] of resolved.resolved.entries()) {
      importMap.imports[dep] = this.buildEsmUrl(dep, version);
    }

    // Add module entries
    for (const module of modules) {
      importMap.imports[module.moduleId] = 
        this.buildEsmUrl(module.moduleId, module.version, {
          deps: Array.from(resolved.resolved.entries())
            .map(([dep, ver]) => `${dep}@${ver}`)
        });
    }

    // Handle conflicts with scopes
    for (const conflict of resolved.conflicts) {
      for (const request of conflict.requests) {
        const scopeKey = this.buildEsmUrl(request.moduleId, '*');
        if (!importMap.scopes[scopeKey]) {
          importMap.scopes[scopeKey] = {};
        }
        
        // Use specific version for this module's scope
        const specificVersion = semver.minSatisfying(
          ['18.2.0', '17.0.2'], 
          request.versionRange
        );
        importMap.scopes[scopeKey][conflict.dep] = 
          this.buildEsmUrl(conflict.dep, specificVersion);
      }
    }

    return importMap;
  }

  buildEsmUrl(pkg, version, options = {}) {
    let url = `${this.esmShCdn}/${pkg}@${version}`;
    
    // Add dependency hints to esm.sh
    if (options.deps && options.deps.length > 0) {
      const depsQuery = options.deps.map(d => `deps=${d}`).join('&');
      url += `?${depsQuery}`;
    }

    // Add other esm.sh options
    if (options.target) {
      url += `${url.includes('?') ? '&' : '?'}target=${options.target}`;
    }

    return url;
  }

  async fetchComposition(compositionId) {
    // Fetch from database
    return {
      id: compositionId,
      modules: [
        { moduleId: '@my-org/dashboard', version: '1.0.0' },
        { moduleId: '@my-org/chart-widget', version: '2.1.0' }
      ]
    };
  }
}

export default ImportMapGenerator;
API Endpoint for Import Maps
javascript// routes/importMap.js
import express from 'express';
import ImportMapGenerator from '../services/importMapGenerator.js';
import ModuleRegistry from '../services/moduleRegistry.js';

const router = express.Router();
const moduleRegistry = new ModuleRegistry();
const generator = new ImportMapGenerator(moduleRegistry);

/**
 * GET /api/import-maps/:compositionId
 * Returns the import map for a specific composition
 */
router.get('/:compositionId', async (req, res) => {
  try {
    const { compositionId } = req.params;
    const { environment = 'blue' } = req.query;

    // Generate import map
    const importMap = await generator.generateForComposition(
      compositionId, 
      environment
    );

    // Cache headers for CDN
    res.set({
      'Cache-Control': 'public, max-age=3600',
      'Content-Type': 'application/json'
    });

    res.json(importMap);
  } catch (error) {
    console.error('Error generating import map:', error);
    res.status(500).json({ error: 'Failed to generate import map' });
  }
});

/**
 * POST /api/import-maps/validate
 * Validates a composition's dependencies
 */
router.post('/validate', async (req, res) => {
  try {
    const { modules } = req.body;

    const modulesMetadata = await Promise.all(
      modules.map(m => moduleRegistry.getMetadata(m.moduleId, m.version))
    );

    const resolved = generator.resolveDependencies(modulesMetadata);

    res.json({
      valid: resolved.conflicts.length === 0,
      resolved: Object.fromEntries(resolved.resolved),
      conflicts: resolved.conflicts
    });
  } catch (error) {
    res.status(500).json({ error: 'Validation failed' });
  }
});

export default router;

3. Index.html Integration
Static index.html Template
html<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Microfrontend Platform</title>
  
  <!-- Import Map Placeholder - Will be injected -->
  <script type="importmap" id="import-map">
    {
      "imports": {}
    }
  </script>

  <!-- ES Module Shims for browsers without import map support -->
  <script async src="https://ga.jspm.io/npm:es-module-shims@1.8.0/dist/es-module-shims.js"></script>

  <style>
    body {
      margin: 0;
      font-family: system-ui, -apple-system, sans-serif;
    }
    
    #app-container {
      min-height: 100vh;
    }
    
    .module-container {
      border: 1px solid #e0e0e0;
      margin: 10px;
      padding: 20px;
    }
    
    .loading {
      display: flex;
      justify-content: center;
      align-items: center;
      height: 100vh;
    }
  </style>
</head>
<body>
  <div id="app-container">
    <div class="loading">Loading application...</div>
  </div>

  <!-- Bootstrap Script -->
  <script type="module">
    import { ApplicationLoader } from './loader.js';
    
    // Initialize application
    const loader = new ApplicationLoader();
    await loader.initialize();
  </script>
</body>
</html>
Dynamic Import Map Injection
Method 1: Server-Side Rendering (SSR)
javascript// server.js - Express server
import express from 'express';
import { readFileSync } from 'fs';
import ImportMapGenerator from './services/importMapGenerator.js';

const app = express();
const indexTemplate = readFileSync('./public/index.html', 'utf-8');

app.get('/', async (req, res) => {
  try {
    // Get composition ID from query, cookie, or user session
    const compositionId = req.query.composition || req.session?.compositionId;
    
    if (!compositionId) {
      return res.status(400).send('No composition specified');
    }

    // Generate import map
    const generator = new ImportMapGenerator();
    const importMap = await generator.generateForComposition(compositionId);

    // Inject import map into HTML
    const html = indexTemplate.replace(
      '<script type="importmap" id="import-map">\n    {\n      "imports": {}\n    }\n  </script>',
      `<script type="importmap" id="import-map">\n${JSON.stringify(importMap, null, 2)}\n  </script>`
    );

    res.send(html);
  } catch (error) {
    console.error('Error rendering page:', error);
    res.status(500).send('Failed to load application');
  }
});

app.listen(3000, () => {
  console.log('Server running on port 3000');
});
Method 2: Client-Side Injection (Dynamic)
javascript// loader.js - Application bootstrap
export class ApplicationLoader {
  constructor() {
    this.compositionId = this.getCompositionId();
    this.importMap = null;
    this.modules = new Map();
  }

  getCompositionId() {
    // Get from URL, localStorage, or default
    const params = new URLSearchParams(window.location.search);
    return params.get('composition') || 
           localStorage.getItem('compositionId') ||
           'default';
  }

  async initialize() {
    try {
      // 1. Fetch import map from backend
      await this.loadImportMap();
      
      // 2. Inject import map into DOM
      this.injectImportMap();
      
      // 3. Fetch composition configuration
      const composition = await this.fetchComposition();
      
      // 4. Load and mount all modules
      await this.loadModules(composition.modules);
      
      console.log('Application loaded successfully');
    } catch (error) {
      console.error('Failed to initialize application:', error);
      this.showError(error);
    }
  }

  async loadImportMap() {
    const response = await fetch(
      `/api/import-maps/${this.compositionId}?environment=${this.getEnvironment()}`
    );
    
    if (!response.ok) {
      throw new Error('Failed to load import map');
    }
    
    this.importMap = await response.json();
  }

  injectImportMap() {
    // Remove existing import map
    const existing = document.getElementById('import-map');
    if (existing) {
      existing.remove();
    }

    // Create and inject new import map
    const script = document.createElement('script');
    script.type = 'importmap';
    script.id = 'import-map';
    script.textContent = JSON.stringify(this.importMap, null, 2);
    
    // Import maps must be inserted before any module scripts
    document.head.insertBefore(script, document.head.firstChild);
    
    console.log('Import map injected:', this.importMap);
  }

  getEnvironment() {
    // Determine environment (blue/green)
    return localStorage.getItem('environment') || 'blue';
  }

  async fetchComposition() {
    const response = await fetch(`/api/compositions/${this.compositionId}`);
    return response.json();
  }

  async loadModules(moduleConfigs) {
    const container = document.getElementById('app-container');
    container.innerHTML = ''; // Clear loading message

    for (const config of moduleConfigs) {
      await this.loadModule(config, container);
    }
  }

  async loadModule(config, container) {
    try {
      // Create module container
      const moduleContainer = document.createElement('div');
      moduleContainer.className = 'module-container';
      moduleContainer.id = `module-${config.moduleId}`;
      container.appendChild(moduleContainer);

      // Dynamic import using the import map
      const module = await import(config.moduleId);
      
      // Mount the module
      if (typeof module.mount === 'function') {
        await module.mount(moduleContainer, config.config);
        this.modules.set(config.moduleId, { module, container: moduleContainer });
      } else {
        console.error(`Module ${config.moduleId} does not export a mount function`);
      }
    } catch (error) {
      console.error(`Failed to load module ${config.moduleId}:`, error);
      this.showModuleError(config.moduleId, error);
    }
  }

  showError(error) {
    document.getElementById('app-container').innerHTML = `
      <div style="padding: 20px; color: red;">
        <h2>Failed to load application</h2>
        <pre>${error.message}</pre>
      </div>
    `;
  }

  showModuleError(moduleId, error) {
    const container = document.getElementById(`module-${moduleId}`);
    if (container) {
      container.innerHTML = `
        <div style="color: red;">
          <strong>Module Error: ${moduleId}</strong>
          <pre>${error.message}</pre>
        </div>
      `;
    }
  }

  async reload() {
    // Unmount all modules
    for (const [moduleId, { module, container }] of this.modules) {
      if (typeof module.unmount === 'function') {
        await module.unmount();
      }
    }
    
    this.modules.clear();
    
    // Reinitialize
    await this.initialize();
  }
}

// Auto-initialize on load
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', async () => {
    const loader = new ApplicationLoader();
    await loader.initialize();
    
    // Expose for debugging
    window.appLoader = loader;
  });
} else {
  const loader = new ApplicationLoader();
  await loader.initialize();
  window.appLoader = loader;
}
Method 3: Inline Import Map (Build Time)
javascript// build-time-generator.js
import fs from 'fs';
import ImportMapGenerator from './services/importMapGenerator.js';

async function buildStatic(compositionId, outputPath) {
  const generator = new ImportMapGenerator();
  const importMap = await generator.generateForComposition(compositionId);
  
  const template = fs.readFileSync('./public/index.template.html', 'utf-8');
  
  const html = template.replace(
    '<!-- IMPORT_MAP_PLACEHOLDER -->',
    `<script type="importmap">\n${JSON.stringify(importMap, null, 2)}\n</script>`
  );
  
  fs.writeFileSync(outputPath, html);
  console.log(`Built static page with import map: ${outputPath}`);
}

// Usage
buildStatic('production-app', './dist/index.html');

4. Complete Workflow Example
Publishing a Module
bash# 1. Develop your module
cd my-dashboard-widget

# 2. Build
npm run build

# 3. Test locally
npm link

# 4. Publish to NPM
npm publish

# 5. Register in your module registry (API call)
curl -X POST https://your-platform.com/api/modules \
  -H "Content-Type: application/json" \
  -d '{
    "moduleId": "@my-org/dashboard-widget",
    "version": "1.0.0",
    "npmPackage": "@my-org/dashboard-widget",
    "entryPoint": "https://esm.sh/@my-org/dashboard-widget@1.0.0"
  }'
Using the Module in a Composition
javascript// User creates composition via UI or API
const composition = {
  id: 'my-app-v1',
  modules: [
    {
      moduleId: '@my-org/dashboard-widget',
      version: '1.0.0',
      config: {
        apiEndpoint: 'https://api.example.com',
        theme: 'dark'
      }
    },
    {
      moduleId: '@my-org/chart-widget',
      version: '2.1.0',
      config: {
        chartType: 'line'
      }
    }
  ]
};

// Save composition
await fetch('/api/compositions', {
  method: 'POST',
  body: JSON.stringify(composition)
});

// Access the app
window.location.href = '/?composition=my-app-v1';
```

### **The Flow**
```
1. User navigates to /?composition=my-app-v1
2. Server/Client fetches composition config
3. Backend generates import map for that composition
4. Import map injected into index.html
5. Browser parses import map
6. Application loader imports modules using short names
7. Modules loaded from esm.sh with correct dependencies
8. Modules mounted into DOM
This architecture provides:

✅ Zero bundler needed at runtime
✅ Automatic dependency resolution
✅ Version pinning and compatibility
✅ Dynamic composition
✅ CDN caching benefits
✅ Fast iteration cycles

Would you like me to create a complete working demo with all these pieces integrated?SHow to do blue greenComplete Blue-Green Deployment Implementation
I'll provide a comprehensive, production-ready blue-green deployment system for your microfrontend platform.
1. Database Schema
sql-- Environments (Blue/Green)
CREATE TABLE environments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(50) UNIQUE NOT NULL CHECK (name IN ('blue', 'green')),
  status VARCHAR(20) NOT NULL CHECK (status IN ('active', 'standby', 'deploying', 'unhealthy')),
  version VARCHAR(50),
  deployed_at TIMESTAMP,
  deployed_by VARCHAR(255),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Module versions per environment
CREATE TABLE environment_modules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id UUID REFERENCES environments(id) ON DELETE CASCADE,
  module_id VARCHAR(255) NOT NULL,
  version VARCHAR(50) NOT NULL,
  npm_package VARCHAR(255) NOT NULL,
  esm_url TEXT NOT NULL,
  config JSONB DEFAULT '{}',
  dependencies JSONB DEFAULT '{}',
  health_status VARCHAR(20) DEFAULT 'unknown' CHECK (health_status IN ('healthy', 'unhealthy', 'unknown')),
  error_count INTEGER DEFAULT 0,
  load_count INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(environment_id, module_id)
);

-- Traffic routing configuration
CREATE TABLE traffic_config (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  active_environment VARCHAR(50) REFERENCES environments(name),
  routing_strategy VARCHAR(50) NOT NULL DEFAULT 'simple' 
    CHECK (routing_strategy IN ('simple', 'canary', 'ab_test', 'blue_green')),
  canary_percentage INTEGER DEFAULT 0 CHECK (canary_percentage >= 0 AND canary_percentage <= 100),
  user_groups JSONB DEFAULT '[]',
  feature_flags JSONB DEFAULT '{}',
  enabled BOOLEAN DEFAULT true,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Deployment history
CREATE TABLE deployments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_name VARCHAR(50) REFERENCES environments(name),
  version VARCHAR(50) NOT NULL,
  status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'rolled_back')),
  deployed_by VARCHAR(255),
  started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  completed_at TIMESTAMP,
  rollback_from UUID REFERENCES deployments(id),
  deployment_notes TEXT,
  modules_deployed JSONB DEFAULT '[]'
);

-- Health check logs
CREATE TABLE health_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_name VARCHAR(50) REFERENCES environments(name),
  module_id VARCHAR(255),
  check_type VARCHAR(50) NOT NULL,
  status VARCHAR(20) NOT NULL,
  error_message TEXT,
  response_time_ms INTEGER,
  checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Initialize environments
INSERT INTO environments (name, status, version) VALUES
  ('blue', 'active', '1.0.0'),
  ('green', 'standby', '1.0.0');

INSERT INTO traffic_config (active_environment, routing_strategy) VALUES
  ('blue', 'simple');
2. Backend Services
Environment Manager Service
javascript// services/environmentManager.js
import { db } from '../database.js';
import { EventEmitter } from 'events';

class EnvironmentManager extends EventEmitter {
  constructor() {
    super();
    this.cache = new Map();
    this.cacheTimeout = 60000; // 1 minute
  }

  /**
   * Get current active environment
   */
  async getActiveEnvironment() {
    const cached = this.cache.get('active');
    if (cached && Date.now() - cached.timestamp < this.cacheTimeout) {
      return cached.data;
    }

    const result = await db.query(`
      SELECT e.*, 
             json_agg(
               json_build_object(
                 'module_id', em.module_id,
                 'version', em.version,
                 'esm_url', em.esm_url,
                 'config', em.config,
                 'dependencies', em.dependencies,
                 'health_status', em.health_status
               )
             ) as modules
      FROM environments e
      LEFT JOIN environment_modules em ON e.id = em.environment_id
      WHERE e.status = 'active'
      GROUP BY e.id
    `);

    const data = result.rows[0];
    this.cache.set('active', { data, timestamp: Date.now() });
    
    return data;
  }

  /**
   * Get standby environment
   */
  async getStandbyEnvironment() {
    const result = await db.query(`
      SELECT * FROM environments 
      WHERE status = 'standby'
      LIMIT 1
    `);
    return result.rows[0];
  }

  /**
   * Get environment by name
   */
  async getEnvironment(name) {
    const result = await db.query(`
      SELECT e.*,
             json_agg(
               json_build_object(
                 'module_id', em.module_id,
                 'version', em.version,
                 'esm_url', em.esm_url,
                 'config', em.config,
                 'dependencies', em.dependencies
               )
             ) as modules
      FROM environments e
      LEFT JOIN environment_modules em ON e.id = em.environment_id
      WHERE e.name = $1
      GROUP BY e.id
    `, [name]);

    return result.rows[0];
  }

  /**
   * Deploy modules to an environment
   */
  async deployToEnvironment(environmentName, modules, version, deployedBy) {
    const client = await db.getClient();
    
    try {
      await client.query('BEGIN');

      // Create deployment record
      const deploymentResult = await client.query(`
        INSERT INTO deployments (environment_name, version, status, deployed_by, modules_deployed)
        VALUES ($1, $2, 'in_progress', $3, $4)
        RETURNING id
      `, [environmentName, version, deployedBy, JSON.stringify(modules)]);

      const deploymentId = deploymentResult.rows[0].id;

      // Update environment status
      await client.query(`
        UPDATE environments 
        SET status = 'deploying', version = $1, updated_at = CURRENT_TIMESTAMP
        WHERE name = $2
      `, [version, environmentName]);

      // Get environment ID
      const envResult = await client.query(
        'SELECT id FROM environments WHERE name = $1',
        [environmentName]
      );
      const environmentId = envResult.rows[0].id;

      // Clear existing modules
      await client.query(
        'DELETE FROM environment_modules WHERE environment_id = $1',
        [environmentId]
      );

      // Insert new modules
      for (const module of modules) {
        await client.query(`
          INSERT INTO environment_modules 
          (environment_id, module_id, version, npm_package, esm_url, config, dependencies)
          VALUES ($1, $2, $3, $4, $5, $6, $7)
        `, [
          environmentId,
          module.moduleId,
          module.version,
          module.npmPackage,
          module.esmUrl,
          JSON.stringify(module.config || {}),
          JSON.stringify(module.dependencies || {})
        ]);
      }

      // Update deployment status
      await client.query(`
        UPDATE deployments 
        SET status = 'completed', completed_at = CURRENT_TIMESTAMP
        WHERE id = $1
      `, [deploymentId]);

      // Update environment status
      await client.query(`
        UPDATE environments 
        SET status = 'standby', deployed_at = CURRENT_TIMESTAMP, deployed_by = $1
        WHERE name = $2
      `, [deployedBy, environmentName]);

      await client.query('COMMIT');

      // Clear cache
      this.cache.clear();

      this.emit('deployment-completed', {
        environment: environmentName,
        version,
        deploymentId
      });

      return deploymentId;
    } catch (error) {
      await client.query('ROLLBACK');
      
      // Mark deployment as failed
      await db.query(`
        UPDATE deployments 
        SET status = 'failed', completed_at = CURRENT_TIMESTAMP
        WHERE environment_name = $1 AND status = 'in_progress'
      `, [environmentName]);

      throw error;
    } finally {
      client.release();
    }
  }

  /**
   * Switch active environment (Blue-Green switch)
   */
  async switchEnvironment(targetEnvironment) {
    const client = await db.getClient();

    try {
      await client.query('BEGIN');

      // Verify target environment is healthy
      const healthCheck = await this.checkEnvironmentHealth(targetEnvironment);
      if (!healthCheck.healthy) {
        throw new Error(`Environment ${targetEnvironment} is not healthy: ${healthCheck.reason}`);
      }

      // Get current active environment
      const currentActive = await client.query(
        "SELECT name FROM environments WHERE status = 'active'"
      );
      const previousEnvironment = currentActive.rows[0]?.name;

      // Switch environments
      await client.query(`
        UPDATE environments 
        SET status = CASE 
          WHEN name = $1 THEN 'active'
          ELSE 'standby'
        END,
        updated_at = CURRENT_TIMESTAMP
      `, [targetEnvironment]);

      // Update traffic config
      await client.query(`
        UPDATE traffic_config 
        SET active_environment = $1,
            routing_strategy = 'simple',
            canary_percentage = 0,
            updated_at = CURRENT_TIMESTAMP
      `, [targetEnvironment]);

      await client.query('COMMIT');

      // Clear cache
      this.cache.clear();

      // Emit event for real-time notifications
      this.emit('environment-switched', {
        from: previousEnvironment,
        to: targetEnvironment,
        timestamp: new Date().toISOString()
      });

      return {
        success: true,
        previousEnvironment,
        currentEnvironment: targetEnvironment
      };
    } catch (error) {
      await client.query('ROLLBACK');
      throw error;
    } finally {
      client.release();
    }
  }

  /**
   * Start canary deployment
   */
  async startCanary(targetEnvironment, percentage) {
    await db.query(`
      UPDATE traffic_config
      SET routing_strategy = 'canary',
          canary_percentage = $1,
          updated_at = CURRENT_TIMESTAMP
    `, [percentage]);

    this.cache.clear();

    this.emit('canary-started', {
      environment: targetEnvironment,
      percentage
    });
  }

  /**
   * Gradually increase canary traffic
   */
  async increaseCanary(percentage) {
    await db.query(`
      UPDATE traffic_config
      SET canary_percentage = $1,
          updated_at = CURRENT_TIMESTAMP
      WHERE routing_strategy = 'canary'
    `, [percentage]);

    this.cache.clear();

    this.emit('canary-increased', { percentage });
  }

  /**
   * Check environment health
   */
  async checkEnvironmentHealth(environmentName) {
    const result = await db.query(`
      SELECT 
        COUNT(*) as total_modules,
        SUM(CASE WHEN health_status = 'healthy' THEN 1 ELSE 0 END) as healthy_modules,
        SUM(CASE WHEN health_status = 'unhealthy' THEN 1 ELSE 0 END) as unhealthy_modules
      FROM environment_modules em
      JOIN environments e ON em.environment_id = e.id
      WHERE e.name = $1
    `, [environmentName]);

    const stats = result.rows[0];
    const healthyPercentage = (stats.healthy_modules / stats.total_modules) * 100;

    return {
      healthy: healthyPercentage >= 80, // 80% threshold
      healthyPercentage,
      stats,
      reason: healthyPercentage < 80 ? 'Less than 80% of modules are healthy' : null
    };
  }

  /**
   * Rollback to previous environment
   */
  async rollback(reason) {
    const client = await db.getClient();

    try {
      await client.query('BEGIN');

      // Get current active
      const current = await client.query(
        "SELECT name FROM environments WHERE status = 'active'"
      );
      const currentEnv = current.rows[0].name;

      // Get standby (previous)
      const standby = await client.query(
        "SELECT name FROM environments WHERE status = 'standby'"
      );
      const previousEnv = standby.rows[0].name;

      // Switch back
      await this.switchEnvironment(previousEnv);

      // Record rollback
      await client.query(`
        INSERT INTO deployments (environment_name, status, deployment_notes)
        VALUES ($1, 'rolled_back', $2)
      `, [currentEnv, `Rolled back from ${currentEnv} to ${previousEnv}. Reason: ${reason}`]);

      await client.query('COMMIT');

      this.emit('rollback-completed', {
        from: currentEnv,
        to: previousEnv,
        reason
      });

      return { success: true, rolledBackTo: previousEnv };
    } catch (error) {
      await client.query('ROLLBACK');
      throw error;
    } finally {
      client.release();
    }
  }

  /**
   * Get traffic configuration
   */
  async getTrafficConfig() {
    const result = await db.query('SELECT * FROM traffic_config WHERE enabled = true LIMIT 1');
    return result.rows[0];
  }
}

export default new EnvironmentManager();
Health Monitor Service
javascript// services/healthMonitor.js
import { db } from '../database.js';
import environmentManager from './environmentManager.js';

class HealthMonitor {
  constructor() {
    this.checkInterval = 30000; // 30 seconds
    this.errorThreshold = 0.1; // 10% error rate
    this.intervalId = null;
  }

  start() {
    console.log('Starting health monitor...');
    this.intervalId = setInterval(() => this.runHealthChecks(), this.checkInterval);
    this.runHealthChecks(); // Run immediately
  }

  stop() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  async runHealthChecks() {
    try {
      const environments = await db.query('SELECT name FROM environments');
      
      for (const env of environments.rows) {
        await this.checkEnvironment(env.name);
      }
    } catch (error) {
      console.error('Health check failed:', error);
    }
  }

  async checkEnvironment(environmentName) {
    const modules = await db.query(`
      SELECT em.* 
      FROM environment_modules em
      JOIN environments e ON em.environment_id = e.id
      WHERE e.name = $1
    `, [environmentName]);

    for (const module of modules.rows) {
      await this.checkModule(environmentName, module);
    }

    // Check overall environment health
    const health = await environmentManager.checkEnvironmentHealth(environmentName);
    
    if (!health.healthy && environmentName === (await environmentManager.getActiveEnvironment()).name) {
      console.warn(`Active environment ${environmentName} is unhealthy!`);
      // Optionally trigger automatic rollback
      await this.handleUnhealthyEnvironment(environmentName);
    }
  }

  async checkModule(environmentName, module) {
    const startTime = Date.now();
    let status = 'healthy';
    let errorMessage = null;

    try {
      // Attempt to fetch the module
      const response = await fetch(module.esm_url, {
        method: 'HEAD',
        timeout: 5000
      });

      if (!response.ok) {
        status = 'unhealthy';
        errorMessage = `HTTP ${response.status}`;
      }
    } catch (error) {
      status = 'unhealthy';
      errorMessage = error.message;
    }

    const responseTime = Date.now() - startTime;

    // Log health check
    await db.query(`
      INSERT INTO health_checks (environment_name, module_id, check_type, status, error_message, response_time_ms)
      VALUES ($1, $2, 'availability', $3, $4, $5)
    `, [environmentName, module.module_id, status, errorMessage, responseTime]);

    // Update module health status
    await db.query(`
      UPDATE environment_modules
      SET health_status = $1,
          error_count = CASE WHEN $1 = 'unhealthy' THEN error_count + 1 ELSE 0 END,
          updated_at = CURRENT_TIMESTAMP
      WHERE id = $2
    `, [status, module.id]);

    // Check error rate
    const errorRate = await this.getModuleErrorRate(module.id);
    if (errorRate > this.errorThreshold) {
      console.error(`Module ${module.module_id} error rate ${errorRate} exceeds threshold`);
    }
  }

  async getModuleErrorRate(moduleId) {
    const result = await db.query(`
      SELECT 
        COUNT(*) FILTER (WHERE status = 'unhealthy') as errors,
        COUNT(*) as total
      FROM health_checks
      WHERE module_id = (SELECT module_id FROM environment_modules WHERE id = $1)
        AND checked_at > NOW() - INTERVAL '10 minutes'
    `, [moduleId]);

    const { errors, total } = result.rows[0];
    return total > 0 ? errors / total : 0;
  }

  async handleUnhealthyEnvironment(environmentName) {
    // Auto-rollback if configured
    const config = await db.query(`
      SELECT * FROM traffic_config WHERE enabled = true
    `);

    // Implement your rollback policy here
    console.log(`Environment ${environmentName} needs attention`);
    
    // Optional: automatic rollback
    // await environmentManager.rollback('Automatic rollback due to health check failure');
  }
}

export default new HealthMonitor();
3. API Routes
javascript// routes/deployment.js
import express from 'express';
import environmentManager from '../services/environmentManager.js';
import healthMonitor from '../services/healthMonitor.js';
import { ImportMapGenerator } from '../services/importMapGenerator.js';

const router = express.Router();

/**
 * GET /api/environments
 * List all environments
 */
router.get('/environments', async (req, res) => {
  try {
    const result = await db.query(`
      SELECT e.*,
             (SELECT COUNT(*) FROM environment_modules WHERE environment_id = e.id) as module_count
      FROM environments e
      ORDER BY name
    `);
    res.json(result.rows);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * GET /api/environments/:name
 * Get specific environment details
 */
router.get('/environments/:name', async (req, res) => {
  try {
    const environment = await environmentManager.getEnvironment(req.params.name);
    res.json(environment);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * GET /api/environments/active
 * Get currently active environment
 */
router.get('/environments/active', async (req, res) => {
  try {
    const environment = await environmentManager.getActiveEnvironment();
    res.json(environment);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * POST /api/deploy
 * Deploy modules to standby environment
 */
router.post('/deploy', async (req, res) => {
  try {
    const { modules, version, deployedBy } = req.body;

    if (!modules || !Array.isArray(modules) || modules.length === 0) {
      return res.status(400).json({ error: 'Modules array is required' });
    }

    // Get standby environment
    const standby = await environmentManager.getStandbyEnvironment();
    
    if (!standby) {
      return res.status(400).json({ error: 'No standby environment available' });
    }

    // Deploy to standby
    const deploymentId = await environmentManager.deployToEnvironment(
      standby.name,
      modules,
      version,
      deployedBy || 'system'
    );

    res.json({
      success: true,
      deploymentId,
      environment: standby.name,
      message: `Deployed to ${standby.name} environment`
    });
  } catch (error) {
    console.error('Deployment failed:', error);
    res.status(500).json({ error: error.message });
  }
});

/**
 * POST /api/switch
 * Switch active environment (blue-green swap)
 */
router.post('/switch', async (req, res) => {
  try {
    const { targetEnvironment } = req.body;

    if (!targetEnvironment) {
      return res.status(400).json({ error: 'targetEnvironment is required' });
    }

    const result = await environmentManager.switchEnvironment(targetEnvironment);
    
    res.json({
      success: true,
      ...result,
      message: `Switched to ${targetEnvironment} environment`
    });
  } catch (error) {
    console.error('Environment switch failed:', error);
    res.status(500).json({ error: error.message });
  }
});

/**
 * POST /api/canary/start
 * Start canary deployment
 */
router.post('/canary/start', async (req, res) => {
  try {
    const { targetEnvironment, percentage = 10 } = req.body;

    await environmentManager.startCanary(targetEnvironment, percentage);

    res.json({
      success: true,
      message: `Started canary deployment to ${targetEnvironment} with ${percentage}% traffic`
    });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * POST /api/canary/increase
 * Increase canary traffic percentage
 */
router.post('/canary/increase', async (req, res) => {
  try {
    const { percentage } = req.body;

    if (percentage < 0 || percentage > 100) {
      return res.status(400).json({ error: 'Percentage must be between 0 and 100' });
    }

    await environmentManager.increaseCanary(percentage);

    res.json({
      success: true,
      percentage,
      message: `Increased canary traffic to ${percentage}%`
    });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * POST /api/rollback
 * Rollback to previous environment
 */
router.post('/rollback', async (req, res) => {
  try {
    const { reason = 'Manual rollback' } = req.body;

    const result = await environmentManager.rollback(reason);

    res.json({
      success: true,
      ...result,
      message: 'Rollback completed successfully'
    });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * GET /api/health/:environmentName
 * Check environment health
 */
router.get('/health/:environmentName', async (req, res) => {
  try {
    const health = await environmentManager.checkEnvironmentHealth(req.params.environmentName);
    res.json(health);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * GET /api/deployments
 * Get deployment history
 */
router.get('/deployments', async (req, res) => {
  try {
    const { limit = 20, environment } = req.query;
    
    let query = `
      SELECT * FROM deployments
      ${environment ? 'WHERE environment_name = $1' : ''}
      ORDER BY started_at DESC
      LIMIT $${environment ? 2 : 1}
    `;
    
    const params = environment ? [environment, limit] : [limit];
    const result = await db.query(query, params);
    
    res.json(result.rows);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

/**
 * GET /api/traffic-config
 * Get current traffic configuration
 */
router.get('/traffic-config', async (req, res) => {
  try {
    const config = await environmentManager.getTrafficConfig();
    res.json(config);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

export default router;
4. Client-Side Environment Router
javascript// public/environmentRouter.js
export class EnvironmentRouter {
  constructor() {
    this.currentEnvironment = null;
    this.trafficConfig = null;
    this.userId = this.getUserId();
    this.pollInterval = 10000; // 10 seconds
  }

  async initialize() {
    await this.determineEnvironment();
    this.startPolling();
    this.setupEventListeners();
  }

  getUserId() {
    // Get from cookie, localStorage, or session
    let userId = localStorage.getItem('userId');
    if (!userId) {
      userId = `user_${Math.random().toString(36).substr(2, 9)}`;
      localStorage.setItem('userId', userId);
    }
    return userId;
  }

  async determineEnvironment() {
    try {
      // Fetch traffic configuration
      const response = await fetch('/api/traffic-config');
      this.trafficConfig = await response.json();

      let targetEnvironment = this.trafficConfig.active_environment;

      // Apply routing strategy
      switch (this.trafficConfig.routing_strategy) {
        case 'simple':
          targetEnvironment = this.trafficConfig.active_environment;
          break;

        case 'canary':
          targetEnvironment = this.routeCanary();
          break;

        case 'ab_test':
          targetEnvironment = this.routeABTest();
          break;

        default:
          targetEnvironment = this.trafficConfig.active_environment;
      }

      this.currentEnvironment = targetEnvironment;
      localStorage.setItem('environment', targetEnvironment);

      return targetEnvironment;
    } catch (error) {
      console.error('Failed to determine environment:', error);
      return 'blue'; // Fallback
    }
  }

  routeCanary() {
    const { canary_percentage, active_environment } = this.trafficConfig;
    
    // Use sticky routing based on user ID
    const hash = this.hashString(this.userId);
    const bucket = hash % 100;

    // If user falls into canary bucket, route to standby
    if (bucket < canary_percentage) {
      return active_environment === 'blue' ? 'green' : 'blue';
    }

    return active_environment;
  }

  routeABTest() {
    // Similar to canary but with 50/50 split
    const hash = this.hashString(this.userId);
    return (hash % 2 === 0) ? 'blue' : 'green';
  }

  hashString(str) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = ((hash << 5) - hash) + char;
      hash = hash & hash; // Convert to 32-bit integer
    }
    return Math.abs(hash);
  }

  async loadEnvironment() {
    const environment = await this.determineEnvironment();
    
    // Fetch environment configuration
    const response = await fetch(`/api/environments/${environment}`);
    const config = await response.json();

    return config;
  }

  startPolling() {
    this.pollIntervalId = setInterval(async () => {
      const newEnvironment = await this.determineEnvironment();
      
      if (newEnvironment !== this.currentEnvironment) {
        console.log(`Environment changed: ${this.currentEnvironment} -> ${newEnvironment}`);
        this.handleEnvironmentChange(newEnvironment);
      }
    }, this.pollInterval);
  }

  stopPolling() {
    if (this.pollIntervalId) {
      clearInterval(this.pollIntervalId);
    }
  }

  setupEventListeners() {
    // Listen for server-sent events (optional)
    const eventSource = new EventSource('/api/events');
    
    eventSource.addEventListener('environment-switch', (event) => {
      const data = JSON.parse(event.data);
      console.log('Environment switch event received:', data);
      this.handleEnvironmentChange(data.to);
    });
  }

  handleEnvironmentChange(newEnvironment) {
    // Option 1: Hard reload (simplest)
    window.location.reload();
    
    // Option 2: Graceful transition (better UX)
    // this.gracefulTransition(newEnvironment);
  }

  async gracefulTransition(newEnvironment) {
    // Show transition UI
    this.showTransitionMessage();

    // Unmount current modules
    if (window.appLoader) {
      await window.appLoader.unmountAll();
    }

    // Update environment
    this.currentEnvironment = newEnvironment;
    localStorage.setItem('environment', newEnvironment);

    // Reload application
    if (window.appLoader) {
      await window.appLoader.reload();
    }

    // Hide transition UI
    this.hideTransitionMessage();
  }

  showTransitionMessage() {
    const overlay = document.createElement('div');
    overlay.id = 'environment-transition';
    overlay.innerHTML = `
      <div style="
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0,0,0,0.8);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 10000;
        color: white;
        font-size: 24px;
      ">
        <div>Updating application...</div>
      </div>
    `;
    document.body.appendChild(overlay);
  }

  hideTransitionMessage() {
    const overlay = document.getElementById('environment-transition');
    if (overlay) {
      overlay.remove();
    }
  }
}

// Auto-initialize
const router = new EnvironmentRouter();
await router.initialize();

// Expose globally
window.environmentRouter = router;
5. Updated Application Loader
javascript// public/loader.js
import { EnvironmentRouter } from './environmentRouter.js';

export class ApplicationLoader {
  constructor() {
    this.environmentRouter = new EnvironmentRouter();
    this.modules = new Map();
    this.importMap = null;
  }

  async initialize() {
    try {
      // 1. Determine which environment to use
      await this.environmentRouter.initialize();
      const environment = this.environmentRouter.currentEnvironment;

      console.log(`Loading environment: ${environment}`);

      // 2. Fetch environment configuration
      const envConfig = await this.environmentRouter.loadEnvironment();

      //SContinue
Generate and inject import map
const importMapGen = new ImportMapGenerator();
this.importMap = importMapGen.buildImportMap(
{ resolved: new Map(Object.entries(envConfig.dependencies || {})), conflicts: [] },
envConfig.modules || []
);
this.injectImportMap();
// 4. Load all modules
await this.loadModules(envConfig.modules || []);
console.log('Application loaded successfully');
} catch (error) {
console.error('Failed to initialize application:', error);
this.showError(error);
}
}

injectImportMap() {
const existing = document.getElementById('import-map');
if (existing) existing.remove();
const script = document.createElement('script');
script.type = 'importmap';
script.id = 'import-map';
script.textContent = JSON.stringify(this.importMap, null, 2);
document.head.insertBefore(script, document.head.firstChild);
}
async loadModules(moduleConfigs) {
const container = document.getElementById('app-container');
container.innerHTML = '';
for (const config of moduleConfigs) {
  await this.loadModule(config, container);
}
}
async loadModule(config, container) {
try {
const moduleContainer = document.createElement('div');
moduleContainer.className = 'module-container';
moduleContainer.id = module-${config.module_id};
container.appendChild(moduleContainer);
  // Dynamic import
  const module = await import(config.module_id);

  // Mount
  if (typeof module.mount === 'function') {
    await module.mount(moduleContainer, config.config);
    this.modules.set(config.module_id, { module, container: moduleContainer });

    // Report successful load
    this.reportModuleHealth(config.module_id, 'healthy');
  }
} catch (error) {
  console.error(`Failed to load module ${config.module_id}:`, error);
  this.reportModuleHealth(config.module_id, 'unhealthy', error.message);
}
}
async unmountAll() {
for (const [moduleId, { module }] of this.modules) {
if (typeof module.unmount === 'function') {
await module.unmount();
}
}
this.modules.clear();
}
async reload() {
await this.unmountAll();
await this.initialize();
}
async reportModuleHealth(moduleId, status, error = null) {
try {
await fetch('/api/module-health', {
method: 'POST',
headers: { 'Content-Type': 'application/json' },
body: JSON.stringify({
environment: this.environmentRouter.currentEnvironment,
moduleId,
status,
error,
timestamp: new Date().toISOString()
})
});
} catch (err) {
console.error('Failed to report module health:', err);
}
}
showError(error) {
document.getElementById('app-container').innerHTML =       <div style="padding: 20px; color: red;">         <h2>Failed to load application</h2>         <pre>${error.message}</pre>       </div>    ;
}
}
// Initialize
const loader = new ApplicationLoader();
await loader.initialize();
window.appLoader = loader;

## 6. Deployment CLI Tool
```javascript
// cli/deploy.js
#!/usr/bin/env node

import { Command } from 'commander';
import fetch from 'node-fetch';
import chalk from 'chalk';

const program = new Command();
const API_BASE = process.env.API_BASE || 'http://localhost:3000/api';

program
  .name('microfrontend-deploy')
  .description('CLI tool for blue-green deployments')
  .version('1.0.0');

program
  .command('deploy')
  .description('Deploy modules to standby environment')
  .option('-m, --modules <modules>', 'Modules JSON file')
  .option('-v, --version <version>', 'Version tag')
  .action(async (options) => {
    try {
      const modules = require(options.modules);
      
      console.log(chalk.blue('🚀 Starting deployment...'));
      
      const response = await fetch(`${API_BASE}/deploy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          modules,
          version: options.version,
          deployedBy: process.env.USER
        })
      });

      const result = await response.json();
      
      if (result.success) {
        console.log(chalk.green(`✓ Deployed to ${result.environment}`));
        console.log(chalk.gray(`  Deployment ID: ${result.deploymentId}`));
      } else {
        console.error(chalk.red('✗ Deployment failed:', result.error));
      }
    } catch (error) {
      console.error(chalk.red('✗ Error:', error.message));
    }
  });

program
  .command('switch')
  .description('Switch to target environment')
  .argument('<environment>', 'Target environment (blue/green)')
  .action(async (environment) => {
    try {
      console.log(chalk.blue(`🔄 Switching to ${environment}...`));
      
      const response = await fetch(`${API_BASE}/switch`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ targetEnvironment: environment })
      });

      const result = await response.json();
      
      if (result.success) {
        console.log(chalk.green(`✓ Switched from ${result.previousEnvironment} to ${result.currentEnvironment}`));
      } else {
        console.error(chalk.red('✗ Switch failed:', result.error));
      }
    } catch (error) {
      console.error(chalk.red('✗ Error:', error.message));
    }
  });

program
  .command('canary')
  .description('Start canary deployment')
  .argument('<environment>', 'Target environment')
  .option('-p, --percentage <percentage>', 'Traffic percentage', '10')
  .action(async (environment, options) => {
    try {
      console.log(chalk.blue(`🐤 Starting canary to ${environment} (${options.percentage}%)...`));
      
      const response = await fetch(`${API_BASE}/canary/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          targetEnvironment: environment,
          percentage: parseInt(options.percentage)
        })
      });

      const result = await response.json();
      console.log(chalk.green('✓ Canary started'));
    } catch (error) {
      console.error(chalk.red('✗ Error:', error.message));
    }
  });

program
  .command('rollback')
  .description('Rollback to previous environment')
  .option('-r, --reason <reason>', 'Rollback reason')
  .action(async (options) => {
    try {
      console.log(chalk.yellow('⚠️  Rolling back...'));
      
      const response = await fetch(`${API_BASE}/rollback`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason: options.reason })
      });

      const result = await response.json();
      
      if (result.success) {
        console.log(chalk.green(`✓ Rolled back to ${result.rolledBackTo}`));
      }
    } catch (error) {
      console.error(chalk.red('✗ Error:', error.message));
    }
  });

program
  .command('status')
  .description('Check deployment status')
  .action(async () => {
    try {
      const [envs, traffic] = await Promise.all([
        fetch(`${API_BASE}/environments`).then(r => r.json()),
        fetch(`${API_BASE}/traffic-config`).then(r => r.json())
      ]);

      console.log(chalk.bold('\n📊 Environment Status:\n'));
      
      envs.forEach(env => {
        const icon = env.status === 'active' ? '🟢' : '🔵';
        console.log(`${icon} ${env.name}: ${env.status} (v${env.version})`);
        console.log(chalk.gray(`   Modules: ${env.module_count}`));
      });

      console.log(chalk.bold('\n🚦 Traffic Configuration:\n'));
      console.log(`   Active: ${traffic.active_environment}`);
      console.log(`   Strategy: ${traffic.routing_strategy}`);
      if (traffic.routing_strategy === 'canary') {
        console.log(`   Canary: ${traffic.canary_percentage}%`);
      }
    } catch (error) {
      console.error(chalk.red('✗ Error:', error.message));
    }
  });

program.parse();
```

## 7. Complete Deployment Workflow
```bash
# 1. Deploy new version to standby
./deploy.js deploy --modules ./modules.json --version 2.0.0

# 2. Check health
./deploy.js status

# 3. Start canary with 10% traffic
./deploy.js canary green --percentage 10

# 4. Monitor for 10 minutes, then increase
./deploy.js canary green --percentage 25
./deploy.js canary green --percentage 50

# 5. Full switch
./deploy.js switch green

# 6. If issues occur, rollback
./deploy.js rollback --reason "High error rate detected"
```

This complete implementation provides:
- ✅ Zero-downtime deployments
- ✅ Instant rollback capability
- ✅ Canary deployments
- ✅ Health monitoring
- ✅ Traffic routing strategies
- ✅ Deployment history
- ✅ CLI tooling

Would you like me to create a working demo or expand on any specific part?SyesI'll create a complete working demo with all the components integrated. This will be a full-stack application you can run locally.Microfrontend Platform DemoInteractive artifact I've created a complete working demo of the Blue-Green deployment system! This interactive demo shows:
Features Demonstrated:

Environment Overview

Visual representation of Blue and Green environments
Real-time status indicators (Active/Standby)
Module listings with health status
Traffic configuration display


Deployment Operations

Deploy new versions to standby environment
Switch between Blue and Green
Canary deployment with gradual traffic increase (10% → 25% → 50%)
Instant rollback capability


Deployment History

Track all deployments
Version history
Status tracking



Try These Workflows:
Basic Blue-Green Switch:

Click "Deploy" tab
Click "Deploy to green"
Go to "Overview" tab
Click "Switch to green"

Canary Deployment:

Click "Start Canary (10%)"
Click "Increase to 25%"
Click "Increase to 50%"
Finally "Switch to green" for full cutover

Rollback:

Click "Rollback" button at any time to instantly revert

The demo simulates all backend operations with realistic delays and state management. Would you like me to create:

The actual backend implementation (Node.js/Express)?
Database migration scripts?
Docker setup for the full stack?
CI/CD pipeline configuration?

Let me know which part you'd like to dive deeper into!