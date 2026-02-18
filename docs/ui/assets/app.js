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
// CONFIGURATION: Edit this to customize your sidebar
// ==========================================
const DOCS_CONFIG = {
  sections: [
    {
      title: "Getting Started",
      links: [
        { title: "Introduction", file: "getting-started/introduction" },
        { title: "System Boundary", file: "getting-started/system-boundary" },
        { title: "Design Principles", file: "getting-started/design-principles" },
        { title: "Feature Matrix", file: "getting-started/feature-matrix" }
      ]
    },
    {
      title: "Architecture",
      links: [
        { title: "High Level Architecture", file: "architechture/high-level-architecture" },
        { title: "Component Overview", file: "architechture/component-overview" },
        { title: "Message Flow", file: "architechture/message-flow" },
        { title: "Webhook Flow", file: "architechture/webhook-flow" },
        { title: "Queue Processing", file: "architechture/queue-processing" }
      ]
    },
    {
      title: "Installation",
      links: [
        { title: "Prerequisites", file: "installation/prerequisites" },
        { title: "Docker Deployment", file: "installation/docker-deployment" },
        { title: "Binary Build", file: "installation/binary-build" },
        { title: "Production Deployment", file: "installation/production-deployment" },
        { title: "Reverse Proxy Setup", file: "installation/reverse-proxy-setup" }
      ]
    },
    {
      title: "Configuration",
      links: [
        { title: "Environment Variables", file: "configuration/environment-variables" },

      ]
    },
    {
      title: "Security",
      links: [
        { title: "Authentication and Security", file: "security/authentication-and-security" },
        { title: "[IMPORTANT] Security Considerations", file: "security/important-security-consideration" },
      ]
    }
  ],
  defaultDoc: "getting-started/introduction" // First doc to load
};

// ==========================================
// CURRENT VIEW STATE
// ==========================================
let currentView = 'docs'; // Track current active view

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

async function loadMarkdownDocs(docName) {
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
window.addEventListener('DOMContentLoaded', () => {
  // Set up logo link to refresh/go to home
  const logoLink = document.getElementById('logo-link');
  logoLink.href = window.location.pathname;

  // Set up RapiDoc spec-url with relative path
  const rapiDocElement = document.getElementById('rapi-doc-element');
  if (rapiDocElement) {
    rapiDocElement.setAttribute('spec-url', buildUrl('yaml'));
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
