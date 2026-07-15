// ==========================================
// BASE PATH DETECTION
// ==========================================
function getBasePath() {
  // Get the current page's path
  const pathname = window.location.pathname;

  // Remove the HTML filename from the path to get the base directory
  // For example: /docs/index.html -> /docs
  // or /documentation/index.html -> /documentation
  // const lastSlashIndex = pathname.lastIndexOf('/');
  // const basePath = pathname.substring(0, lastSlashIndex);

  // remove / index.html or .html from the end if present 
  const basePath = pathname.replace(/\/?[^\/]*\.html$/, '');

  // Return the base path, ensuring it doesn't end with a slash
  // unless it's the root path
  return basePath === '' ? '' : basePath;
}

// Helper function to build relative URLs
function buildUrl(path) {
  const basePath = getBasePath();

  // Remove leading slash from path if present
  const cleanPath = path.startsWith('/') ? path.substring(1) : path;

  // Combine base path with the resource path
  // If basePath is empty (root), just return the path with leading slash
  if (basePath === '') {
    return '/' + cleanPath;
  }

  return basePath + "/" + cleanPath;
}

// ==========================================
// SIDEBAR CONFIG
// Loaded from assets/nav.json — the single source of truth shared by the Go
// console and the static GitHub Pages build, so the nav can no longer drift
// between them. To add/reorder docs, edit nav.json (not this file).
// ==========================================
let DOCS_CONFIG = { sections: [], defaultDoc: "" };

async function loadNavConfig() {
  const res = await fetch(buildUrl('assets/nav.json'));
  if (!res.ok) {
    throw new Error('assets/nav.json not found');
  }
  DOCS_CONFIG = await res.json();
}

// HAS_API is true only when the interactive API (RapiDoc) tab is present. The
// authenticated Go console renders it; the public static site does not, so all
// API-only behaviour below is gated on this instead of a separate build.
const HAS_API = !!document.querySelector('[data-view="api"]');

// ==========================================
// CURRENT VIEW STATE
// ==========================================
let currentView = 'docs'; // Track current active view
let currentDoc = null;    // Track the doc on screen (for theme re-render)

// ==========================================
// THEME TOGGLE (light / dark)
// ==========================================
const SUN_SVG = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>';
const MOON_SVG = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>';

function currentTheme() {
  return document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark';
}

function setupThemeToggle() {
  const btn = document.getElementById('theme-toggle');
  if (!btn) return;
  const paint = () => {
    const dark = currentTheme() === 'dark';
    btn.innerHTML = dark ? SUN_SVG : MOON_SVG;
    btn.setAttribute('aria-label', dark ? 'Switch to light theme' : 'Switch to dark theme');
  };
  paint();
  btn.addEventListener('click', () => {
    const next = currentTheme() === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    try { localStorage.setItem('waga-theme', next); } catch (e) {}
    paint();
    // Re-render current doc so mermaid diagrams pick up the new theme.
    if (currentDoc) loadMarkdownDocs(currentDoc);
  });
}

// ==========================================
// HASH ROUTING UTILITIES
// ==========================================
function parseHash() {
  const hash = window.location.hash.substring(1); // Remove #

  if (!hash) {
    return { view: 'docs', doc: null, section: null };
  }

  // Check if hash starts with view prefix (docs: or api:)
  if (hash.startsWith('docs:')) {
    const parts = hash.substring(5).split(':');
    return {
      view: 'docs',
      doc: parts[0] || null,
      section: parts[1] || null
    };
  }

  if (hash.startsWith('api:')) {
    return {
      view: 'api',
      target: hash.substring(4) || null
    };
  }

  // Backward compatibility: if no prefix, assume docs
  // Check if it's a doc name or section
  const parts = hash.split(':');
  return {
    view: 'docs',
    doc: parts[0] || null,
    section: parts[1] || null
  };
}

function buildHash(view, doc, section) {
  if (view === 'api') {
    return `#api:${doc || ''}`;
  }

  // Docs view
  if (section) {
    return `#docs:${doc}:${section}`;
  }
  if (doc) {
    return `#docs:${doc}`;
  }
  return '';
}

// ==========================================
// SIDEBAR GENERATION
// ==========================================
function generateSidebar() {
  const sidebar = document.getElementById('sidebar');
  let sidebarHTML = '';

  // Desktop-only notice for mobile users (only meaningful when the API tab exists)
  if (HAS_API) {
    sidebarHTML += `
      <div class="mobile-desktop-notice">
        <div class="note-box">
          <strong>Mobile View</strong>
          <p style="margin-top: 0.5rem; margin-bottom: 0;">
            API Reference is available on desktop. Open this page on a larger screen for full features.
          </p>
        </div>
      </div>
    `;
  }

  DOCS_CONFIG.sections.forEach(section => {
    sidebarHTML += `
      <div class="sidebar-section">
        <h3 class="sidebar-title">${section.title}</h3>
        <ul class="sidebar-links">
          ${section.links.map((link, index) => {
      const isFirst = section === DOCS_CONFIG.sections[0] && index === 0;
      return `
              <li>
                <a href="${buildHash('docs', link.file, null)}"
                   class="sidebar-link ${isFirst ? 'active' : ''}"
                   data-doc="${link.file}">
                  ${link.title}
                </a>
              </li>
            `;
    }).join('')}
        </ul>
      </div>
    `;
  });

  sidebar.innerHTML = sidebarHTML;

  // Add click handlers to all sidebar links
  document.querySelectorAll('.sidebar-link').forEach(link => {
    link.addEventListener('click', handleSidebarClick);
  });
}

// ==========================================
// EVENT HANDLERS
// ==========================================
function handleSidebarClick(e) {
  e.preventDefault();

  // Update active link
  document.querySelectorAll('.sidebar-link').forEach(l => l.classList.remove('active'));
  this.classList.add('active');

  // Load the corresponding doc
  const docName = this.dataset.doc;

  // Update URL hash with docs: prefix
  window.history.pushState(null, '', buildHash('docs', docName, null));

  // Switch to docs view if not already there
  if (currentView !== 'docs') {
    switchToView('docs');
  }

  loadMarkdownDocs(docName);

  // Close sidebar on mobile
  const sidebar = document.getElementById('sidebar');
  const sidebarOverlay = document.getElementById('sidebar-overlay');
  if (window.innerWidth <= 768 && sidebar.classList.contains('open')) {
    sidebar.classList.remove('open');
    sidebarOverlay.classList.remove('active');
    document.body.style.overflow = '';
  }
}

// Initialize marked.js
if (typeof marked !== 'undefined') {
  marked.setOptions({
    breaks: true,
    gfm: true
  });
}

// View switching
function switchToView(viewName) {
  currentView = viewName;

  const navTabs = document.querySelectorAll('.nav-tab');
  const contentViews = document.querySelectorAll('.content-view');

  // Update body class for view-specific styling
  document.body.classList.remove('docs-view', 'api-view');
  document.body.classList.add(`${viewName}-view`);

  // Update active tab
  navTabs.forEach(t => {
    t.classList.remove('active');
    if (t.dataset.view === viewName) {
      t.classList.add('active');
    }
  });

  // Update active view
  contentViews.forEach(view => {
    view.classList.remove('active');
    if (view.id === `${viewName}-view`) {
      view.classList.add('active');
    }
  });
}

// Setup view tab listeners
function setupViewTabs() {
  const navTabs = document.querySelectorAll('.nav-tab');

  navTabs.forEach(tab => {
    tab.addEventListener('click', () => {
      const viewName = tab.dataset.view;

      // Update hash based on view
      if (viewName === 'api') {
        window.history.pushState(null, '', buildHash('api', '', null));
      } else {
        // Stay on current doc or go to default
        const hashData = parseHash();
        const currentDoc = hashData.doc || DOCS_CONFIG.defaultDoc;
        window.history.pushState(null, '', buildHash('docs', currentDoc, null));
      }

      switchToView(viewName);
    });
  });
}

// ==========================================
// MARKDOWN LOADING
// ==========================================

// Helper function to generate slug from heading text
function generateSlug(text) {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '') // Remove special characters
    .replace(/\s+/g, '-')      // Replace spaces with hyphens
    .replace(/-+/g, '-')       // Replace multiple hyphens with single hyphen
    .trim();
}

// Helper function to add IDs and anchor links to headings
function addHeadingAnchors(htmlContent, docName) {
  const tempDiv = document.createElement('div');
  tempDiv.innerHTML = htmlContent;

  // Process h1 and h2 elements
  const headings = tempDiv.querySelectorAll('h1, h2');
  headings.forEach(heading => {
    const text = heading.textContent;
    const slug = generateSlug(text);

    // Set ID on the heading
    heading.id = slug;

    // Add anchor link
    const anchor = document.createElement('a');
    anchor.href = buildHash('docs', docName, slug);
    anchor.className = 'heading-anchor';
    anchor.textContent = '#';
    anchor.setAttribute('aria-label', `Link to ${text}`);

    // Add click handler to update URL without reload
    anchor.addEventListener('click', (e) => {
      e.preventDefault();
      window.history.pushState(null, '', buildHash('docs', docName, slug));
      heading.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });

    heading.appendChild(anchor);
  });

  return tempDiv.innerHTML;
}

// ==========================================
// MERMAID DIAGRAMS (lazy-loaded)
// ==========================================
// mermaid.min.js is ~3.4MB, so it is fetched only when a rendered doc actually
// contains a ```mermaid block, not on every page.
let mermaidPromise = null;
function ensureMermaid() {
  if (window.mermaid) return Promise.resolve(window.mermaid);
  if (mermaidPromise) return mermaidPromise;
  mermaidPromise = new Promise((resolve, reject) => {
    const s = document.createElement('script');
    s.src = buildUrl('assets/mermaid.min.js');
    s.onload = () => resolve(window.mermaid);
    s.onerror = () => { mermaidPromise = null; reject(new Error('mermaid failed to load')); };
    document.head.appendChild(s);
  });
  return mermaidPromise;
}

async function renderMermaidDiagrams(container) {
  // marked renders ```mermaid as <pre><code class="language-mermaid">source</code></pre>
  const blocks = container.querySelectorAll('code.language-mermaid');
  if (!blocks.length) return;

  let mermaid;
  try {
    mermaid = await ensureMermaid();
  } catch (e) {
    console.error(e);
    return; // leave the raw code block in place on failure
  }

  const isLight = document.documentElement.getAttribute('data-theme') === 'light';
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    theme: isLight ? 'default' : 'dark',
  });

  const nodes = [];
  blocks.forEach(code => {
    const pre = code.closest('pre') || code;
    const div = document.createElement('div');
    div.className = 'mermaid';
    div.textContent = code.textContent; // decoded source, not HTML-escaped
    pre.replaceWith(div);
    nodes.push(div);
  });

  try {
    await mermaid.run({ nodes });
  } catch (e) {
    console.error('mermaid render error', e);
  }
}

// ==========================================
// ON-PAGE TABLE OF CONTENTS + SCROLL SPY
// ==========================================
let tocObserver = null;

function buildTableOfContents(contentEl, docName) {
  const toc = document.getElementById('docs-toc');
  const inner = document.querySelector('.docs-inner');
  if (!toc) return;
  toc.innerHTML = '';
  if (tocObserver) { tocObserver.disconnect(); tocObserver = null; }

  const items = [];
  contentEl.querySelectorAll('h2, h3').forEach(h => {
    const text = (h.textContent || '').replace(/#\s*$/, '').trim();
    if (!text) return;
    if (!h.id) h.id = generateSlug(text); // h3 has no id from addHeadingAnchors
    items.push({ el: h, id: h.id, text: text, level: h.tagName === 'H3' ? 3 : 2 });
  });

  // Not worth a rail for a stub page.
  if (items.length < 2) {
    inner && inner.classList.remove('has-toc');
    return;
  }

  const title = document.createElement('div');
  title.className = 'docs-toc-title';
  title.textContent = 'On this page';
  toc.appendChild(title);

  const links = {};
  items.forEach(it => {
    const a = document.createElement('a');
    a.href = buildHash('docs', docName, it.id);
    a.textContent = it.text;
    a.dataset.id = it.id;
    if (it.level === 3) a.classList.add('lvl-3');
    a.addEventListener('click', e => {
      e.preventDefault();
      window.history.pushState(null, '', buildHash('docs', docName, it.id));
      it.el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });
    toc.appendChild(a);
    links[it.id] = a;
  });
  inner && inner.classList.add('has-toc');

  // Scroll-spy: highlight the topmost heading currently in view. The scroll
  // container is .docs-content, so the observer is rooted there.
  const root = document.querySelector('.docs-content');
  const visible = new Set();
  tocObserver = new IntersectionObserver(entries => {
    entries.forEach(en => {
      if (en.isIntersecting) visible.add(en.target.id);
      else visible.delete(en.target.id);
    });
    let activeId = null;
    for (const it of items) { if (visible.has(it.id)) { activeId = it.id; break; } }
    if (!activeId) return;
    Object.keys(links).forEach(id => links[id].classList.toggle('active', id === activeId));
  }, { root: root, rootMargin: '-10% 0px -70% 0px', threshold: 0 });
  items.forEach(it => tocObserver.observe(it.el));
}

async function loadMarkdownDocs(docName) {
  currentDoc = docName;
  const contentDiv = document.getElementById('markdown-content');

  // Show loading spinner
  contentDiv.innerHTML = '<div class="loading-spinner"><div class="spinner"></div></div>';

  try {
    // Build the relative URL for the markdown file
    const markdownUrl = buildUrl(`assets/docs/${docName}.md`);

    // Try to fetch the markdown file
    const response = await fetch(markdownUrl);

    if (!response.ok) {
      throw new Error(`File ${docName}.md not found`);
    }

    const markdown = await response.text();
    let html = marked.parse(markdown);

    // Add IDs and anchor links to headings
    html = addHeadingAnchors(html, docName);

    contentDiv.innerHTML = html;

    // Render any mermaid diagrams (lazy-loads the lib only if present)
    renderMermaidDiagrams(contentDiv);

    // Build the on-page table of contents + scroll spy
    buildTableOfContents(contentDiv, docName);

    // Check if there's a section hash and scroll to it
    const hashData = parseHash();
    if (hashData.section) {
      // Wait a bit for rendering to complete
      setTimeout(() => {
        const targetElement = document.getElementById(hashData.section);
        if (targetElement) {
          targetElement.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
      }, 100);
    } else {
      // Scroll to top if no section
      document.querySelector('.docs-content').scrollTop = 0;
    }

  } catch (error) {
    // Show error message with helpful information
    contentDiv.innerHTML = `
  <div style="text-align: center; padding: 4rem 2rem;">
    <h2 style="color: #ef4444; margin-bottom: 1rem; font-size: 1.5rem;">
      Documentation Not Available
    </h2>
    
    <p style="color: #a1a1a1; margin-bottom: 1.5rem; font-size: 1rem;">
      The page you are looking for could not be displayed at the moment.
    </p>

    <div class="info-box" style="max-width: 600px; margin: 2rem auto; text-align: left;">
      <p style="margin-bottom: 1rem;"><strong>What you can do:</strong></p>
      <ul style="margin-left: 1.5rem; color: #a1a1a1;">
        <li style="margin-bottom: 0.5rem;">Check the sidebar and select another section</li>
        <li style="margin-bottom: 0.5rem;">Refresh the page and try again</li>
        <li style="margin-bottom: 0.5rem;">Contact support if the issue continues</li>
      </ul>

      <p style="margin-top: 1rem; color: #a1a1a1;">
        If this content should be available, please reach out to the documentation owner or system administrator.
      </p>
    </div>

    <div class="info-box" style="max-width: 600px; margin: 0 auto; text-align: left;">
      <p style="margin-bottom: 1rem;">
        <strong>Need help?</strong> Use the navigation menu to explore available documentation sections.
      </p>
    </div>
  </div>
`;
    console.error('Error loading markdown:', error);
  }
}

// ==========================================
// INITIALIZATION
// ==========================================
window.addEventListener('DOMContentLoaded', async () => {
  // Mobile sidebar toggle
  const mobileMenuBtn = document.getElementById('mobile-menu-btn');
  const sidebarOverlay = document.getElementById('sidebar-overlay');
  const sidebar = document.getElementById('sidebar');

  function toggleSidebar() {
    sidebar.classList.toggle('open');
    sidebarOverlay.classList.toggle('active');
    document.body.style.overflow = sidebar.classList.contains('open') ? 'hidden' : '';
  }

  mobileMenuBtn?.addEventListener('click', toggleSidebar);
  sidebarOverlay?.addEventListener('click', toggleSidebar);

  setupThemeToggle();

  // Set up logo link to refresh/go to home
  const logoLink = document.getElementById('logo-link');
  logoLink.href = window.location.pathname;

  // Set up RapiDoc spec-url with relative path
  const rapiDocElement = document.getElementById('rapi-doc-element');
  if (rapiDocElement) {
    rapiDocElement.setAttribute('spec-url', buildUrl('yaml'));
  }

  // Load the shared nav config (assets/nav.json) before building the sidebar.
  try {
    await loadNavConfig();
  } catch (err) {
    console.error(err);
    document.getElementById('markdown-content').innerHTML =
      '<div style="padding:2rem;color:#ef4444;">Failed to load navigation (assets/nav.json).</div>';
    return;
  }

  // Generate sidebar from config
  generateSidebar();

  // Setup view tabs
  setupViewTabs();

  // Parse initial hash
  const hashData = parseHash();

  // Determine initial view and doc
  let initialView = hashData.view || 'docs';
  let initialDoc = DOCS_CONFIG.defaultDoc;

  // Redirect API to docs on mobile
  if (window.innerWidth <= 768 && initialView === 'api') {
    window.history.replaceState(null, '', buildHash('docs', DOCS_CONFIG.defaultDoc, null));
    initialView = 'docs';
  }

  if (initialView === 'docs') {
    if (hashData.doc) {
      // Check if the doc exists
      let docFound = false;
      DOCS_CONFIG.sections.forEach(section => {
        section.links.forEach(link => {
          if (link.file === hashData.doc) {
            initialDoc = hashData.doc;
            docFound = true;

            // Update active sidebar link
            document.querySelectorAll('.sidebar-link').forEach(l => {
              l.classList.remove('active');
              if (l.dataset.doc === hashData.doc) {
                l.classList.add('active');
              }
            });
          }
        });
      });
    }

    // Load initial documentation
    loadMarkdownDocs(initialDoc);
  }

  // Switch to initial view
  switchToView(initialView);

});

// Handle hash changes (when user clicks anchor links or uses back/forward)
window.addEventListener('hashchange', () => {
  const hashData = parseHash();

  // Redirect API to docs on mobile
  if (window.innerWidth <= 768 && hashData.view === 'api') {
    window.history.replaceState(null, '', buildHash('docs', DOCS_CONFIG.defaultDoc, null));
    return;
  }

  // Switch view if needed
  if (hashData.view !== currentView) {
    switchToView(hashData.view);
  }

  // Handle based on view
  if (hashData.view === 'docs') {
    if (hashData.doc) {
      // Check if it's a document change
      let isDocumentHash = false;
      DOCS_CONFIG.sections.forEach(section => {
        section.links.forEach(link => {
          if (link.file === hashData.doc) {
            isDocumentHash = true;

            // Update active sidebar link
            document.querySelectorAll('.sidebar-link').forEach(l => {
              l.classList.remove('active');
              if (l.dataset.doc === hashData.doc) {
                l.classList.add('active');
              }
            });

            // Load the document
            loadMarkdownDocs(hashData.doc);
          }
        });
      });

      // If not a document change, it might be just a section scroll
      if (!isDocumentHash && hashData.section) {
        const targetElement = document.getElementById(hashData.section);
        if (targetElement) {
          targetElement.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
      }
    }
  }
  // API view handles its own hash routing through RapiDoc
});
