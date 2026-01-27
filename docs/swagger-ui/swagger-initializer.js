window.onload = function () {
  const pathParts = window.location.pathname.split('/');

  // Remove 'index.html' and 'ui'
  pathParts.pop(); // index.html
  pathParts.pop(); // ui

  const basePath = pathParts.join('/');
  const swaggerYamlUrl = `${basePath}/yaml`;

  window.ui = SwaggerUIBundle({
    url: swaggerYamlUrl,
    dom_id: '#swagger-ui',
    deepLinking: true,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [
      SwaggerUIBundle.plugins.DownloadUrl
    ],
    layout: "StandaloneLayout"
  });
};
