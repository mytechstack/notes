https://www.linkedin.com/pulse/chapter-3-troubleshooting-performance-optimization-webpack-jerriwal-jqkbc/

Key Performance Issues & Solutions
1. Remote Loading Overhead

Remotes are loaded at runtime, adding network latency
Solution: Preload critical remotes using <link rel="preload"> or load them eagerly in your app bootstrap
Use prefetchRemote() to load modules before they're needed

2. Bundle Size & Code Splitting

Each remote often includes duplicate dependencies
Solution: Share common dependencies properly in your webpack config:

Mark libraries like React, React-DOM as singleton: true
Use requiredVersion to ensure compatibility
Share only what's necessary - don't over-share



3. Caching Strategy

Remote entries reload on every navigation
Solution: Leverage browser caching with proper cache headers
Use versioned URLs or content hashes for remotes
Consider service workers for aggressive caching

4. Runtime Chunk Loading

Module Federation adds runtime overhead
Solution: Optimize chunk splitting strategy
Use runtimeChunk: 'single' in webpack config
Minimize the number of remotes loaded per page

5. Development vs Production

Dev mode has no optimizations
Solution: Test performance in production builds
Enable compression (gzip/brotli)
Use production mode webpack optimizations


mmediate Fixes
1. Consolidate Shared Dependencies
Put this in your webpack config for each micro-frontend:
javascriptshared: {
  react: { singleton: true, eager: true },
  'react-dom': { singleton: true, eager: true },
  // Add other common libraries
}
eager: true bundles shared deps in the main chunk instead of separate network requests.
2. Reduce Remote Scope Loading
Each remote creates multiple requests for its container + chunks. Minimize remotes:

Combine related micro-frontends into one remote
Only expose what's actually consumed
Avoid loading remotes you don't use on a page

3. Use Eager Consumption
In your exposed modules config:
javascriptexposes: {
  './Component': './src/Component',
},
shared: {
  react: { 
    singleton: true, 
    eager: true,  // Prevents separate chunk loading
    requiredVersion: '^18.0.0'
  }
}
4. Bundle Analyzer
Run webpack-bundle-analyzer to see exactly what's being requested:
bashnpm install --save-dev webpack-bundle-analyzer
This shows which remotes/chunks are causing the cascade of requests.
5. HTTP/2 Server Push
If you control your server, use HTTP/2 which handles multiple requests efficiently over one connection.
Can you share:

How many remotes you're loading?
Are you seeing 20+ requests, 50+, or 100+?
Are these mostly JS chunks or also CSS/assets?


Modern Alternatives
1. Native ES Module Imports (Import Maps)

Browser-native module loading without bundler overhead
Use import maps to define module locations
Tools: SystemJS, es-module-shims
Best for: Simpler apps, avoiding build complexity
Tradeoff: Less tooling support, manual optimization

2. Single-SPA

Mature micro-frontend framework (predates Module Federation)
Better control over loading/mounting lifecycle
Can combine with webpack, Vite, or native modules
Best for: Complex multi-team apps with different frameworks
Tradeoff: More boilerplate, steeper learning curve

3. Vite Module Federation Plugin

Faster dev experience than webpack
vite-plugin-federation mimics Module Federation
Better HMR and build speeds
Best for: Modern stacks wanting MF benefits with speed
Tradeoff: Smaller ecosystem than webpack

4. Turbopack/Rspack with Module Federation

Rust-based bundlers (faster than webpack)
Rspack has native Module Federation support
Significantly faster builds
Best for: Large apps where build time matters
Tradeoff: Less mature, fewer plugins

5. Monorepo with Shared Packages

Nx, Turborepo, or pnpm workspaces
Build all apps together, share code via packages
Deploy separately but integrate at build time
Best for: Single org controlling all apps
Tradeoff: Not true runtime composition, tighter coupling

6. Micro-Frontends via iframes

Complete isolation between apps
Zero network overhead for shared code (each app is independent)
Best for: Strong isolation requirements
Tradeoff: Poor UX, communication overhead, styling challenges

My Recommendation
If network calls are your main issue, I'd suggest:

First: Try fixing Module Federation with eager loading and better sharing config (often solves 70% of issues)
If that fails: Consider Vite + vite-plugin-federation - it's the easiest migration with better performance
For complete rewrite: Go with a monorepo approach (Nx/Turborepo) - eliminates runtime loading entirely



Why Rspack Makes Sense
1. Native Module Federation Support

Built-in support (not a plugin) - it's a first-class feature
API-compatible with Webpack's Module Federation
Much faster builds (10x-20x in some cases)

2. Migration Path

Designed as a Webpack replacement
Most webpack configs work with minimal changes
Can migrate incrementally

3. Performance Benefits

Written in Rust (vs JavaScript for webpack)
Faster cold starts and rebuilds
Better handling of large dependency graphs
Should reduce your network call issues through faster optimization passes

Quick Migration Example
javascript// rspack.config.js
const rspack = require('@rspack/core');

module.exports = {
  plugins: [
    new rspack.container.ModuleFederationPlugin({
      name: 'app1',
      filename: 'remoteEntry.js',
      exposes: {
        './Component': './src/Component',
      },
      shared: {
        react: { singleton: true, eager: true },
        'react-dom': { singleton: true, eager: true },
      },
    }),
  ],
};
The API is nearly identical to webpack!
Considerations
Pros:

Keeps your Module Federation architecture
Massive build speed improvements
Active development (ByteDance backing)
Growing ecosystem

Cons:

Still relatively new (less mature than webpack 5)
Some webpack plugins may not work
Smaller community/fewer Stack Overflow answers
May have edge case bugs

My Recommendation
Given your network call issues, I'd try this approach:

First: Fix your Module Federation config (eager loading, proper sharing) - 1 day effort
If still slow: Migrate to Rspack - 2-5 days depending on complexity
Rspack's faster optimization should help reduce redundant chunks/calls



