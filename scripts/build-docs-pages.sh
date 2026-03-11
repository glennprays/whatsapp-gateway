#!/bin/bash
# Generates GitHub Pages documentation from source docs/ui/
# This script creates a docs-only version of the UI without the API/OpenAPI tab

set -e

SITE_DIR="${1:-_site}"

# Base URL for assets - empty string means root (/)
# Set BASE_PATH env var only if you need a custom prefix (e.g., /docs)
BASE_URL="${BASE_PATH:-}"

echo "Building documentation to $SITE_DIR..."
if [ -n "$BASE_URL" ]; then
  echo "Base URL: $BASE_URL"
else
  echo "Base URL: / (root)"
fi

# Create directory structure
mkdir -p "$SITE_DIR/assets/docs"

# Copy static assets
echo "Copying documentation files..."
cp -r docs/ui/assets/docs/* "$SITE_DIR/assets/docs/"

echo "Copying styles..."
cp docs/ui/assets/styles.css "$SITE_DIR/assets/"

echo "Copying marked.js..."
cp docs/ui/assets/marked.min.js "$SITE_DIR/assets/"

echo "Copying favicon files..."
cp docs/ui/assets/favicon-32x32.png "$SITE_DIR/assets/"
cp docs/ui/assets/favicon-16x16.png "$SITE_DIR/assets/"
cp docs/ui/assets/favicon.ico "$SITE_DIR/assets/"
cp docs/ui/assets/og-image.png "$SITE_DIR/assets/"

# Copy llms.txt and openapi.yaml for public access
echo "Copying llms.txt..."
cp llms.txt "$SITE_DIR/"

echo "Copying openapi.yaml..."
cp docs/openapi.yaml "$SITE_DIR/"

# Generate docs-only app.js
echo "Generating simplified app.js..."
cat >"$SITE_DIR/assets/app.js" <<'APPJS_EOF'
// ==========================================
// BASE PATH DETECTION
// ==========================================
function getBasePath() {
  const pathname = window.location.pathname;

  // Remove the HTML filename from the path to get the base directory
  let basePath = pathname.replace(/\/?[^\/]*\.html$/, '');

  // Normalize: "/" or "" should return "" (empty string)
  // This prevents protocol-relative URLs like "//assets/..."
  if (basePath === '/' || basePath === '') {
    return '';
  }

  return basePath;
}

// Helper function to build relative URLs
function buildUrl(path) {
  const basePath = getBasePath();
  const cleanPath = path.startsWith('/') ? path.substring(1) : path;

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
        { title: "Storage Configuration", file: "configuration/storage-configuration" },
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
  defaultDoc: "getting-started/introduction"
};

// ==========================================
// HASH ROUTING UTILITIES
// ==========================================
function parseHash() {
  const hash = window.location.hash.substring(1);

  if (!hash) {
    return { doc: null, section: null };
  }

  const parts = hash.split(':');
  return {
    doc: parts[0] || null,
    section: parts[1] || null
  };
}

function buildHash(doc, section) {
  if (section) {
    return `#${doc}:${section}`;
  }
  if (doc) {
    return `#${doc}`;
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
                <a href="${buildHash(link.file, null)}"
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

  document.querySelectorAll('.sidebar-link').forEach(link => {
    link.addEventListener('click', handleSidebarClick);
  });
}

// ==========================================
// EVENT HANDLERS
// ==========================================
function handleSidebarClick(e) {
  e.preventDefault();

  document.querySelectorAll('.sidebar-link').forEach(l => l.classList.remove('active'));
  this.classList.add('active');

  const docName = this.dataset.doc;
  window.history.pushState(null, '', buildHash(docName, null));
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

// ==========================================
// MARKDOWN LOADING
// ==========================================

function generateSlug(text) {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .trim();
}

function addHeadingAnchors(htmlContent, docName) {
  const tempDiv = document.createElement('div');
  tempDiv.innerHTML = htmlContent;

  const headings = tempDiv.querySelectorAll('h1, h2');
  headings.forEach(heading => {
    const text = heading.textContent;
    const slug = generateSlug(text);

    heading.id = slug;

    const anchor = document.createElement('a');
    anchor.href = buildHash(docName, slug);
    anchor.className = 'heading-anchor';
    anchor.textContent = '#';
    anchor.setAttribute('aria-label', `Link to ${text}`);

    anchor.addEventListener('click', (e) => {
      e.preventDefault();
      window.history.pushState(null, '', buildHash(docName, slug));
      heading.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });

    heading.appendChild(anchor);
  });

  return tempDiv.innerHTML;
}

async function loadMarkdownDocs(docName) {
  const contentDiv = document.getElementById('markdown-content');

  contentDiv.innerHTML = '<div class="loading-spinner"><div class="spinner"></div></div>';

  try {
    const markdownUrl = buildUrl(`assets/docs/${docName}.md`);

    const response = await fetch(markdownUrl);

    if (!response.ok) {
      throw new Error(`File ${docName}.md not found`);
    }

    const markdown = await response.text();
    let html = marked.parse(markdown);

    html = addHeadingAnchors(html, docName);

    contentDiv.innerHTML = html;

    const hashData = parseHash();
    if (hashData.section) {
      setTimeout(() => {
        const targetElement = document.getElementById(hashData.section);
        if (targetElement) {
          targetElement.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
      }, 100);
    } else {
      document.querySelector('.docs-content').scrollTop = 0;
    }

  } catch (error) {
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

  const logoLink = document.getElementById('logo-link');
  logoLink.href = window.location.pathname;

  generateSidebar();

  const hashData = parseHash();
  let initialDoc = DOCS_CONFIG.defaultDoc;

  if (hashData.doc) {
    let docFound = false;
    DOCS_CONFIG.sections.forEach(section => {
      section.links.forEach(link => {
        if (link.file === hashData.doc) {
          initialDoc = hashData.doc;
          docFound = true;

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

  loadMarkdownDocs(initialDoc);
});

// Handle hash changes
window.addEventListener('hashchange', () => {
  const hashData = parseHash();

  if (hashData.doc) {
    let isDocumentHash = false;
    DOCS_CONFIG.sections.forEach(section => {
      section.links.forEach(link => {
        if (link.file === hashData.doc) {
          isDocumentHash = true;

          document.querySelectorAll('.sidebar-link').forEach(l => {
            l.classList.remove('active');
            if (l.dataset.doc === hashData.doc) {
              l.classList.add('active');
            }
          });

          loadMarkdownDocs(hashData.doc);
        }
      });
    });

    if (!isDocumentHash && hashData.section) {
      const targetElement = document.getElementById(hashData.section);
      if (targetElement) {
        targetElement.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
  }
});
APPJS_EOF

# Generate docs-only index.html
echo "Generating simplified index.html..."
cat >"$SITE_DIR/index.html" <<HTML_EOF
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>WhatsApp Gateway - Open Source Go REST API</title>

  <!-- SEO Meta Tags -->
  <meta name="description" content="WhatsApp Gateway is an open-source Go project for WhatsApp API integration. Build messaging integrations with our REST API gateway.">
  <meta name="keywords" content="WhatsApp, WhatsApp API, Go, Golang, gateway, open source, messaging, integration">
  <meta name="author" content="Glenn Prays">
  <link rel="canonical" href="https://waga.glennprays.com/">

  <!-- Favicon -->
  <link rel="icon" type="image/png" sizes="32x32" href="${BASE_URL}/assets/favicon-32x32.png">
  <link rel="icon" type="image/png" sizes="16x16" href="${BASE_URL}/assets/favicon-16x16.png">
  <link rel="shortcut icon" href="${BASE_URL}/assets/favicon.ico">

  <!-- Open Graph / Facebook -->
  <meta property="og:type" content="website">
  <meta property="og:url" content="https://waga.glennprays.com/">
  <meta property="og:title" content="WhatsApp Gateway - Open Source Go REST API">
  <meta property="og:description" content="WhatsApp Gateway is an open-source Go project for WhatsApp API integration. Build messaging integrations with our REST API gateway.">
  <meta property="og:image" content="https://waga.glennprays.com/assets/og-image.png">

  <!-- Twitter -->
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:url" content="https://waga.glennprays.com/">
  <meta name="twitter:title" content="WhatsApp Gateway - Open Source Go REST API">
  <meta name="twitter:description" content="WhatsApp Gateway is an open-source Go project for WhatsApp API integration. Build messaging integrations with our REST API gateway.">
  <meta name="twitter:image" content="https://waga.glennprays.com/assets/og-image.png">

  <!-- Structured Data -->
  <script type="application/ld+json">
  {
    "@context": "https://schema.org",
    "@type": "SoftwareSourceCode",
    "name": "WhatsApp Gateway",
    "description": "Open-source WhatsApp Gateway written in Go. REST API gateway that abstracts WhatsApp Web complexity.",
    "author": {
      "@type": "Person",
      "name": "Glenn Prays"
    },
    "programmingLanguage": "Go",
    "license": "https://opensource.org/licenses/MIT",
    "codeRepository": "https://github.com/glennprays/whatsapp-gateway",
    "url": "https://waga.glennprays.com/"
  }
  </script>

  <!-- External CSS -->
  <link rel="stylesheet" href="${BASE_URL}/assets/styles.css">

  <!-- Marked.js for Markdown rendering -->
  <script src="${BASE_URL}/assets/marked.min.js"></script>

  <!-- Load fonts -->
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
</head>
<body>
  <!-- Header -->
  <div class="header">
    <div class="header-content">
      <div class="header-left">
        <button class="mobile-menu-btn" id="mobile-menu-btn" aria-label="Toggle menu">
          <span class="hamburger-line"></span>
          <span class="hamburger-line"></span>
          <span class="hamburger-line"></span>
        </button>
        <a href="#" class="logo" id="logo-link">
          <span class="logo-text">WhatsApp Gateway</span>
        </a>
        <div class="nav-tabs">
          <button class="nav-tab active">Docs</button>
        </div>
      </div>
      <div class="header-right">
        <a href="https://github.com/glennprays/whatsapp-gateway" class="github-link" target="_blank">GitHub →</a>
      </div>
    </div>
  </div>

  <div class="content-container">
    <!-- Documentation View -->
    <div id="docs-view" class="content-view active">
      <div class="docs-layout">
        <!-- Sidebar Navigation -->
        <nav class="docs-sidebar" id="sidebar">
          <!-- Sidebar will be populated by JavaScript -->
        </nav>
        <div id="sidebar-overlay"></div>

        <!-- Main Content -->
        <main class="docs-content">
          <div class="docs-inner">
            <div class="markdown-content" id="markdown-content">
              <div class="loading-spinner">
                <div class="spinner"></div>
              </div>
            </div>
          </div>
        </main>
      </div>
    </div>
  </div>

  <!-- External JavaScript -->
  <script src="${BASE_URL}/assets/app.js"></script>
</body>
</html>
HTML_EOF

# Create .nojekyll to prevent Jekyll processing
touch "$SITE_DIR/.nojekyll"

echo ""
echo "Done! Site built in $SITE_DIR/"
echo ""
echo "Structure:"
find "$SITE_DIR" -type f | head -30
echo ""
echo "Documentation count:"
find "$SITE_DIR/assets/docs" -name "*.md" | wc -l
